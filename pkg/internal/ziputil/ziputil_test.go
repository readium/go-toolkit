package ziputil

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWriterUsesBestCompression(t *testing.T) {
	var sb strings.Builder
	for i := 0; sb.Len() < 200_000; i++ {
		sb.WriteString("The quick brown fox jumps over the lazy dog, again and again. ")
		sb.WriteString(strings.Repeat("x", i%37))
	}
	data := []byte(sb.String())

	var best bytes.Buffer
	fw, err := flate.NewWriter(&best, flate.BestCompression)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, fw.Close())

	var buf bytes.Buffer
	zw := NewWriter(&buf)
	require.NoError(t, WriteBytes(zw, "text.txt", data, zip.Deflate))
	require.NoError(t, zw.Close())

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, r.File, 1)
	assert.EqualValues(t, best.Len(), r.File[0].CompressedSize64)
}

func TestWriteMimetype(t *testing.T) {
	var buf bytes.Buffer
	zw := NewWriter(&buf)
	require.NoError(t, WriteMimetype(zw, "application/epub+zip"))
	require.NoError(t, zw.Close())

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	f := r.File[0]
	assert.Equal(t, "mimetype", f.Name)
	assert.Equal(t, zip.Store, f.Method)
	assert.EqualValues(t, 20, f.ReaderVersion)
	assert.Empty(t, f.Extra, "OCF forbids an extra field in the mimetype header")
	assert.Zero(t, f.Flags&0x8, "no data descriptor")
}
