package mediatype

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnifferLimitsDocumentSize(t *testing.T) {
	oversized := append(append([]byte(`{"metadata": {"title": "`), bytes.Repeat([]byte("x"), MaxDocumentSize)...), []byte(`"}}`)...)

	// Archive entries were read whole, however much they inflated.
	path := filepath.Join(t.TempDir(), "package.unknown")
	out, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(out)
	for name, data := range map[string][]byte{"manifest.json": oversized, "mimetype": []byte("application/epub+zip")} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, out.Close())
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { assert.NoError(t, f.Close()) }()
	sc := &SnifferContext{content: NewSnifferFileContent(f)}
	defer sc.Close()
	assert.Nil(t, sc.ReadArchiveEntryAt(t.Context(), "manifest.json"))
	assert.Equal(t, []byte("application/epub+zip"), sc.ReadArchiveEntryAt(t.Context(), "mimetype"))

	// So was JSON content.
	assert.Nil(t, SnifferContext{content: NewSnifferBytesContent(oversized)}.ContentAsJSON())
	assert.NotNil(t, SnifferContext{content: NewSnifferBytesContent([]byte(`{"metadata": {}}`))}.ContentAsJSON())
}
