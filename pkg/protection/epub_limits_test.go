package protection

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inflatingXML writes a package holding an XML document at path, deflating
// to a few kilobytes but inflating beyond size.
func inflatingXML(t *testing.T, path string, size int) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})
	w, err := zw.Create(path)
	require.NoError(t, err)
	_, err = w.Write([]byte(`<root a="`))
	require.NoError(t, err)
	_, err = w.Write(bytes.Repeat([]byte("a"), size))
	require.NoError(t, err)
	_, err = w.Write([]byte(`"/>`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	file := filepath.Join(t.TempDir(), "package.epub")
	require.NoError(t, os.WriteFile(file, buf.Bytes(), 0o644))
	return file
}

func TestIdentifyEPUBProtectionLimitsMetadataSize(t *testing.T) {
	// A package of a few hundred kilobytes made detection allocate gigabytes.
	for _, path := range []string{pathFairplaySinf, pathAdeptRights, pathKoboRights, PathEPUBEncryption} {
		f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), inflatingXML(t, path, MaxMetadataSize))
		require.NoError(t, err)
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		_, _, err = IdentifyEPUBProtection(t.Context(), f)
		runtime.ReadMemStats(&after)
		f.Close()
		assert.True(t, errors.Is(err, fetcher.ErrResourceTooLarge), "%s: %v", path, err)
		assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(4*MaxMetadataSize), path)
	}

	// Below the limit, documents are read.
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), inflatingXML(t, PathEPUBEncryption, 1<<20))
	require.NoError(t, err)
	defer f.Close()
	scheme, doc, err := IdentifyEPUBProtection(t.Context(), f)
	require.NoError(t, err)
	assert.Equal(t, Generic, scheme)
	assert.NotNil(t, doc)
}
