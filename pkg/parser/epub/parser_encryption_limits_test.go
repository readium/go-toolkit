package epub

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEncryptionDataLimitsSize(t *testing.T) {
	// The detection of the protection doesn't read encryption.xml when there
	// is an LCP license: the parser did, whatever its size.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(protection.PathEPUBEncryption)
	require.NoError(t, err)
	_, err = w.Write([]byte(`<encryption a="` + string(bytes.Repeat([]byte("a"), protection.MaxMetadataSize)) + `"/>`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	path := filepath.Join(t.TempDir(), "package.epub")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), path)
	require.NoError(t, err)
	defer f.Close()

	_, err = parseEncryptionData(t.Context(), f, protection.LCP.URI(), nil)
	assert.True(t, errors.Is(err, fetcher.ErrResourceTooLarge), err)

	// Missing or invalid documents declare no encryption, as before.
	data, err := parseEncryptionData(t.Context(), fetcher.EmptyFetcher{}, protection.LCP.URI(), nil)
	require.NoError(t, err)
	assert.Nil(t, data)
}
