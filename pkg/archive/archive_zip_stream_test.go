package archive

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Deterministic, poorly-compressible payload so stored/deflated sizes differ
// and range checks catch off-by-one errors.
func streamTestPayload(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*7 + i>>8 + i>>13)
	}
	return data
}

const (
	storedName  = "media/stored.bin"
	deflateName = "text/deflated.txt"
)

var (
	storedPayload  = streamTestPayload(100_000)
	deflatePayload = bytes.Repeat([]byte("readium go-toolkit stream test payload. "), 5000)
)

func makeStreamTestZip(t *testing.T) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, err := zw.CreateHeader(&zip.FileHeader{Name: storedName, Method: zip.Store})
	require.NoError(t, err)
	_, err = w.Write(storedPayload)
	require.NoError(t, err)

	w, err = zw.CreateHeader(&zip.FileHeader{Name: deflateName, Method: zip.Deflate})
	require.NoError(t, err)
	_, err = w.Write(deflatePayload)
	require.NoError(t, err)

	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// withStreamTestArchives runs the callback against the same zip opened from a
// local file (raw-file fast paths) and from memory (fallback paths).
func withStreamTestArchives(t *testing.T, callback func(t *testing.T, a Archive)) {
	data := makeStreamTestZip(t)

	fpath := filepath.Join(t.TempDir(), "stream.zip")
	require.NoError(t, os.WriteFile(fpath, data, 0o600))
	u, err := url.FromFilepath(fpath)
	require.NoError(t, err)
	fileArchive, err := DefaultArchiveFactory{}.Open(t.Context(), u, "")
	require.NoError(t, err)
	defer fileArchive.Close()

	bytesArchive, err := DefaultArchiveFactory{}.OpenBytes(t.Context(), data, "")
	require.NoError(t, err)
	defer bytesArchive.Close()

	t.Run("file", func(t *testing.T) { callback(t, fileArchive) })
	t.Run("bytes", func(t *testing.T) { callback(t, bytesArchive) })
}

func TestZipStreamMatchesContent(t *testing.T) {
	entries := map[string][]byte{
		storedName:  storedPayload,
		deflateName: deflatePayload,
	}
	ranges := []struct {
		name       string
		start, end int64
	}{
		{"full", 0, 0},
		{"prefix", 0, 9},
		{"middle", 1000, 4999},
		{"suffixExact", int64(len(storedPayload)) - 100, int64(len(storedPayload)) - 1},
		{"clampedEnd", 5, int64(len(storedPayload)) + 500},
		{"negativeStart", -5, 9},
	}

	withStreamTestArchives(t, func(t *testing.T, a Archive) {
		for name, content := range entries {
			entry, err := a.Entry(name)
			require.NoError(t, err)

			for _, r := range ranges {
				var b bytes.Buffer
				n, err := entry.Stream(&b, r.start, r.end)
				require.NoError(t, err, "%s %s", name, r.name)

				start, end := r.start, r.end
				if start < 0 {
					start = 0
				}
				expected := content
				if !(r.start == 0 && r.end == 0) {
					if end > int64(len(content))-1 {
						end = int64(len(content)) - 1
					}
					expected = content[start : end+1]
				}
				assert.EqualValues(t, len(expected), n, "%s %s", name, r.name)
				assert.True(t, bytes.Equal(expected, b.Bytes()), "%s %s content mismatch", name, r.name)

				// Stream and Read must agree
				rb, err := entry.Read(r.start, r.end)
				require.NoError(t, err, "%s %s", name, r.name)
				assert.True(t, bytes.Equal(rb, b.Bytes()), "%s %s Stream/Read mismatch", name, r.name)
			}
		}
	})
}

func TestZipStreamStartPastEnd(t *testing.T) {
	withStreamTestArchives(t, func(t *testing.T, a Archive) {
		entry, err := a.Entry(storedName)
		require.NoError(t, err)

		var b bytes.Buffer
		n, err := entry.Stream(&b, int64(len(storedPayload))+10, int64(len(storedPayload))+20)
		if err != nil {
			// Fallback paths surface EOF here; either way nothing is written.
			assert.ErrorIs(t, err, io.EOF)
		}
		assert.LessOrEqual(t, n, int64(0))
		assert.Empty(t, b.Bytes())
	})
}

func TestZipStreamCompressed(t *testing.T) {
	withStreamTestArchives(t, func(t *testing.T, a Archive) {
		entry, err := a.Entry(deflateName)
		require.NoError(t, err)

		var b bytes.Buffer
		n, err := entry.Stream(&b, 0, 0)
		require.NoError(t, err)
		assert.EqualValues(t, len(deflatePayload), n)

		var cb bytes.Buffer
		cn, err := entry.StreamCompressed(&cb)
		require.NoError(t, err)
		assert.EqualValues(t, entry.CompressedLength(), cn)
		assert.EqualValues(t, entry.CompressedLength(), cb.Len())

		// The streamed bytes must be the actual deflate stream of the content
		fr := flate.NewReader(bytes.NewReader(cb.Bytes()))
		inflated, err := io.ReadAll(fr)
		require.NoError(t, err)
		assert.True(t, bytes.Equal(deflatePayload, inflated))

		// Stored entries have no compressed representation
		stored, err := a.Entry(storedName)
		require.NoError(t, err)
		_, err = stored.StreamCompressed(&bytes.Buffer{})
		assert.Error(t, err)
	})
}

func TestZipStreamCompressedGzip(t *testing.T) {
	withStreamTestArchives(t, func(t *testing.T, a Archive) {
		entry, err := a.Entry(deflateName)
		require.NoError(t, err)

		var b bytes.Buffer
		n, err := entry.StreamCompressedGzip(&b)
		require.NoError(t, err)
		assert.EqualValues(t, b.Len(), n)

		gr, err := gzip.NewReader(bytes.NewReader(b.Bytes()))
		require.NoError(t, err)
		gunzipped, err := io.ReadAll(gr)
		require.NoError(t, err)
		require.NoError(t, gr.Close())
		assert.True(t, bytes.Equal(deflatePayload, gunzipped))
	})
}
