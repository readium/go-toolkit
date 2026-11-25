package archive

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeStoredZip writes a ZIP holding a stored entry with the given content.
func writeStoredZip(t *testing.T, path string, content []byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "entry.txt", Method: zip.Store})
	require.NoError(t, err)
	_, err = w.Write(content)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
}

func TestZIPStreamsFromTheArchiveOpened(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "publication.zip")
	original := bytes.Repeat([]byte("A"), 1000)
	writeStoredZip(t, path, original)
	a, err := gozipArchiveFactory{}.Open(path, "")
	require.NoError(t, err)
	defer a.Close()
	entry, err := a.Entry("entry.txt")
	require.NoError(t, err)

	// Replacing the archive, e.g. atomically by renaming another file over
	// it, used to mix the bytes of the replacement into the entries streamed
	// from the original, at its offsets.
	replacement := filepath.Join(dir, "replacement.zip")
	writeStoredZip(t, replacement, bytes.Repeat([]byte("B"), 1000))
	require.NoError(t, os.Rename(replacement, path))
	var buf bytes.Buffer
	_, err = entry.Stream(&buf, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, original, buf.Bytes())

	// A short copy, e.g. from an archive truncated in place, used to succeed.
	b, err := gozipArchiveFactory{}.Open(path, "")
	require.NoError(t, err)
	defer b.Close()
	truncated, err := b.Entry("entry.txt")
	require.NoError(t, err)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, os.Truncate(path, info.Size()-200))
	buf.Reset()
	_, err = truncated.Stream(&buf, 0, 0)
	assert.Error(t, err)
}

func TestZIPShortEntries(t *testing.T) {
	// An entry declaring more bytes than its content, as when its archive is
	// truncated, was streamed as if it were complete when read without its
	// checksum, as remote archives are.
	content := []byte("hello world")
	var compressed bytes.Buffer
	fw, err := flate.NewWriter(&compressed, flate.BestCompression)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, fw.Close())
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateRaw(&zip.FileHeader{
		Name: "entry.txt", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(content),
		CompressedSize64: uint64(compressed.Len()), UncompressedSize64: 1000,
	})
	require.NoError(t, err)
	_, err = w.Write(compressed.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	a := NewGoZIPArchive(zr, func() error { return nil }, true)
	entry, err := a.Entry("entry.txt")
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = entry.Stream(&out, 0, 0)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "whole")
	out.Reset()
	_, err = entry.Stream(&out, 5, 500)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF, "range")
}

func TestZIPWholeEntriesAreVerified(t *testing.T) {
	// Whole entries were read up to their declared length, while their
	// checksum is only verified at their end, as is their length: an entry
	// with a bad checksum, or inflating longer than declared, passed.
	content := bytes.Repeat([]byte("encrypted"), 100)
	deflated := func(data []byte) []byte {
		var b bytes.Buffer
		fw, err := flate.NewWriter(&b, flate.BestCompression)
		require.NoError(t, err)
		_, err = fw.Write(data)
		require.NoError(t, err)
		require.NoError(t, fw.Close())
		return b.Bytes()
	}
	for name, c := range map[string]struct {
		data []byte
		crc  uint32
	}{
		"bad checksum":         {content, crc32.ChecksumIEEE(content) + 1},
		"longer than declared": {append(bytes.Clone(content), "more"...), crc32.ChecksumIEEE(content)},
	} {
		compressed := deflated(c.data)
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.CreateRaw(&zip.FileHeader{
			Name: "entry.txt", Method: zip.Deflate, CRC32: c.crc,
			CompressedSize64: uint64(len(compressed)), UncompressedSize64: uint64(len(content)),
		})
		require.NoError(t, err)
		_, err = w.Write(compressed)
		require.NoError(t, err)
		require.NoError(t, zw.Close())
		path := filepath.Join(t.TempDir(), "archive.zip")
		require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
		a, err := gozipArchiveFactory{}.Open(path, "")
		require.NoError(t, err)
		entry, err := a.Entry("entry.txt")
		require.NoError(t, err)
		_, err = entry.Stream(io.Discard, 0, 0)
		assert.Error(t, err, name)
		a.Close()
	}
}
