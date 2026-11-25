package webpub

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLimitsManifestSize(t *testing.T) {
	// The manifest was read whole, however much it inflated.
	oversized := `{"metadata": {"title": "` + strings.Repeat("x", manifest.MaxManifestSize) + `"}, "readingOrder": []}`

	packagePath := filepath.Join(t.TempDir(), "publication.webpub")
	f, err := os.Create(packagePath)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	w, err := zw.Create("manifest.json")
	require.NoError(t, err)
	_, err = w.Write([]byte(oversized))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	_, err = parseFileAsset(t, nil, packagePath, &mediatype.ReadiumWebpub)
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge)

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(oversized), 0o644))
	_, err = parseFileAsset(t, nil, manifestPath, &mediatype.ReadiumWebpubManifest)
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge)
}

func TestParseNullManifest(t *testing.T) {
	// A manifest.json of "null" made the parser dereference a nil manifest.
	for _, mt := range []*mediatype.MediaType{&mediatype.LCPProtectedPDF, &mediatype.ReadiumWebpub} {
		packagePath := filepath.Join(t.TempDir(), "publication.zip")
		f, err := os.Create(packagePath)
		require.NoError(t, err)
		zw := zip.NewWriter(f)
		w, err := zw.Create("manifest.json")
		require.NoError(t, err)
		_, err = w.Write([]byte("null"))
		require.NoError(t, err)
		require.NoError(t, zw.Close())
		require.NoError(t, f.Close())
		_, err = parseFileAsset(t, nil, packagePath, mt)
		assert.Error(t, err, mt.String())
	}

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte("null"), 0o644))
	_, err := parseFileAsset(t, nil, manifestPath, &mediatype.ReadiumWebpubManifest)
	assert.Error(t, err)
}
