package archive

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand/v2"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bytesReaderAtCloser struct{ *bytes.Reader }

func (bytesReaderAtCloser) Close() error { return nil }

// zipWith returns a ZIP archive holding one entry with the given content.
func zipWith(t *testing.T, name string, method uint16, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// forge overwrites a 32-bit size field of the central directory entry of the
// archive's only file: 20 for the compressed size, 24 for the uncompressed
// size.
func forge(data []byte, field int, size uint32) []byte {
	data = bytes.Clone(data)
	i := bytes.LastIndex(data, []byte{0x50, 0x4b, 0x01, 0x02})
	binary.LittleEndian.PutUint32(data[i+field:], size)
	return data
}

func allocated(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func openZip(t *testing.T, data []byte, minimizeReads bool) Archive {
	t.Helper()
	a, err := gozipArchiveFactory{}.OpenReader(bytesReaderAtCloser{bytes.NewReader(data)}, int64(len(data)), "", minimizeReads)
	require.NoError(t, err)
	return a
}

func TestZipForgedSizes(t *testing.T) {
	deflated := zipWith(t, "META-INF/container.xml", zip.Deflate, []byte("<container/>"))
	stored := zipWith(t, "image.jpg", zip.Store, bytes.Repeat([]byte{1}, 100))
	for name, data := range map[string][]byte{
		"compressed size larger than the archive": forge(deflated, 20, 0x7ffffff0),
		"deflated beyond deflate's ratio":         forge(deflated, 24, 0x7ffffff0),
		"stored sizes differing":                  forge(stored, 24, 0x7ffffff0),
	} {
		for _, minimizeReads := range []bool{false, true} {
			a := openZip(t, data, minimizeReads)
			e := a.Entries()[0]
			// A few hundred bytes of archive used to require 2 GiB of memory.
			assert.Less(t, allocated(func() {
				_, err := e.Read(0, 0)
				assert.ErrorIs(t, err, zip.ErrFormat, name)
				_, err = e.Stream(io.Discard, 0, 0)
				assert.ErrorIs(t, err, zip.ErrFormat, name)
				if e.CompressedAs(CompressionMethodDeflate) {
					_, err = e.ReadCompressed()
					assert.ErrorIs(t, err, zip.ErrFormat, name)
					_, err = e.ReadCompressedGzip()
					assert.ErrorIs(t, err, zip.ErrFormat, name)
				}
			}), uint64(1<<20), "%s (minimizeReads=%v)", name, minimizeReads)
		}
	}
}

func TestZipLargeEntries(t *testing.T) {
	// Highly compressible content, close to deflate's maximum ratio.
	content := make([]byte, 8<<20)
	for _, method := range []uint16{zip.Store, zip.Deflate} {
		data := zipWith(t, "big.bin", method, content)
		for _, minimizeReads := range []bool{false, true} {
			e := openZip(t, data, minimizeReads).Entries()[0]
			read, err := e.Read(0, 0)
			require.NoError(t, err)
			assert.Len(t, read, len(content))
			n, err := e.Stream(io.Discard, 0, 0)
			require.NoError(t, err)
			assert.EqualValues(t, len(content), n)
		}
	}
}

// stoppingWriter fails once it has received limit bytes.
type stoppingWriter struct {
	limit, n int
}

var errStop = errors.New("stop")

func (w *stoppingWriter) Write(p []byte) (int, error) {
	w.n += len(p)
	if w.n > w.limit {
		return 0, errStop
	}
	return len(p), nil
}

func TestZipRemoteStreamStopsEarly(t *testing.T) {
	// Random data doesn't compress: the deflated entry is as large as its
	// content, too large to be read in one go.
	content := make([]byte, 32<<20)
	_, _ = rand.NewChaCha8([32]byte{}).Read(content)
	data := zipWith(t, "big.bin", zip.Deflate, content)
	e := openZip(t, data, true).Entries()[0]
	require.Greater(t, e.CompressedLength(), uint64(streamCopyBufferSize))

	// The writer stops the stream at 1 MiB, before the entry is read whole.
	assert.Less(t, allocated(func() {
		_, err := e.Stream(&stoppingWriter{limit: 1 << 20}, 0, 0)
		assert.ErrorIs(t, err, errStop)
	}), uint64(16<<20))
}
