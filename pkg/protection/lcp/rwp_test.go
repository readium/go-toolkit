package lcp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRWPManifest = `{
  "@context": "https://readium.org/webpub-manifest/context.jsonld",
  "metadata": {
    "title": "Test",
    "conformsTo": "https://readium.org/webpub-manifest/profiles/divina",
    "x-custom": {"kept": true, "number": 12345678901234567890}
  },
  "links": [{"rel": "self", "href": "manifest.json", "type": "application/webpub+json"}],
  "readingOrder": [
    {"href": "images/cover.jpg", "type": "image/jpeg", "rel": "cover"},
    {"href": "images/page 1.jpg", "type": "image/jpeg", "properties": {"page": "right"},
     "alternate": [{"href": "images/page 1.webp", "type": "image/webp"}]},
    {"href": "chapter.html", "type": "text/html"}
  ],
  "resources": [
    {"href": "style.css", "type": "text/css"},
    {"href": "images/figure.png", "type": "image/png"}
  ]
}`

var testRWPFiles = map[string][]byte{
	"images/cover.jpg":    randomBytes(1000),
	"images/page 1.jpg":   randomBytes(5000),
	"images/page 1.webp":  randomBytes(4000),
	"chapter.html":        textBytes(6000),
	"style.css":           textBytes(300),
	"images/figure.png":   randomBytes(2000),
	"extra/unlisted.json": textBytes(100),
}

// buildRWP writes a packaged webpub and returns its path.
func buildRWP(t *testing.T) string {
	t.Helper()
	return buildRWPWith(t, testRWPManifest)
}

// buildRWPWith writes a packaged webpub with the given manifest and the
// testRWPFiles, and returns its path.
func buildRWPWith(t *testing.T, manifestJSON string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("manifest.json")
	require.NoError(t, err)
	_, err = w.Write([]byte(manifestJSON))
	require.NoError(t, err)
	for _, name := range []string{"images/cover.jpg", "images/page 1.jpg", "images/page 1.webp", "chapter.html", "style.css", "images/figure.png", "extra/unlisted.json"} {
		method := uint16(zip.Deflate)
		if filepath.Ext(name) != ".html" && filepath.Ext(name) != ".css" && filepath.Ext(name) != ".json" {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		require.NoError(t, err)
		_, err = w.Write(testRWPFiles[name])
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return writeTemp(t, "test.webpub", buf.Bytes())
}

func encryptedHrefs(t *testing.T, rawManifest []byte) map[string]manifest.Encryption {
	t.Helper()
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rawManifest, &raw))
	m, err := manifest.ManifestFromJSON(raw, true)
	require.NoError(t, err)
	encs := make(map[string]manifest.Encryption)
	collect := func(l manifest.Link) {
		if enc := l.Properties.Encryption(); enc != nil {
			encs[l.Href.String()] = *enc
		}
	}
	walkLinks(m.ReadingOrder, collect)
	walkLinks(m.Resources, collect)
	return encs
}

func TestEncryptRWP(t *testing.T) {
	key := testKey("rwp")
	source := buildRWP(t)

	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, source), key, &buf, Options{}))
	encrypted := writeTemp(t, "encrypted.webpub", buf.Bytes())

	sourceEntries, sourceNames := readZip(t, source)
	output, outputNames := readZip(t, encrypted)
	assert.Equal(t, "manifest.json", outputNames[0])
	assert.ElementsMatch(t, sourceNames, outputNames)

	// The manifest is patched, keeping what the model doesn't know about.
	var patched map[string]any
	dec := json.NewDecoder(bytes.NewReader(output["manifest.json"].content))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&patched))
	custom := patched["metadata"].(map[string]any)["x-custom"].(map[string]any)
	assert.Equal(t, true, custom["kept"])
	assert.Equal(t, json.Number("12345678901234567890"), custom["number"])

	// Everything listed in the reading order and resources is encrypted,
	// except the cover.
	assert.Equal(t, map[string]manifest.Encryption{
		"images/page%201.jpg":  {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, OriginalLength: 5000},
		"images/page%201.webp": {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, OriginalLength: 4000},
		"chapter.html":         {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, Compression: CompressionDeflate, OriginalLength: 6000},
		"style.css":            {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, Compression: CompressionDeflate, OriginalLength: 300},
		"images/figure.png":    {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, OriginalLength: 2000},
	}, encryptedHrefs(t, output["manifest.json"].content))

	// Other properties of encrypted links are kept.
	page := patched["readingOrder"].([]any)[1].(map[string]any)
	assert.Equal(t, "right", page["properties"].(map[string]any)["page"])

	// The cover, and files the manifest doesn't list, are copied verbatim.
	for _, name := range []string{"images/cover.jpg", "extra/unlisted.json"} {
		assert.Equal(t, sourceEntries[name].method, output[name].method, name)
		assert.True(t, bytes.Equal(sourceEntries[name].raw, output[name].raw), "%s not copied verbatim", name)
	}
	for _, name := range []string{"images/page 1.jpg", "images/page 1.webp", "chapter.html", "style.css", "images/figure.png"} {
		assert.EqualValues(t, zip.Store, output[name].method, name)
		assert.NotEqual(t, testRWPFiles[name], output[name].content, name)
	}

	// The webpub parser reads the encryption properties back, and the
	// decryptor restores the resources.
	original := openPublication(t, source, streamer.Config{})
	decrypted := openPublication(t, encrypted, streamer.Config{
		OnCreatePublication: func(b *pub.Builder) error { return DecryptBuilder(b, key) },
	})
	assertSameResources(t, original, decrypted)

	// Decrypting restores the entries and the manifest.
	var restoredBuf bytes.Buffer
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, encrypted), key, &restoredBuf, Options{}))
	restored, _ := readZip(t, writeTemp(t, "restored.webpub", restoredBuf.Bytes()))
	for name, content := range testRWPFiles {
		assert.True(t, bytes.Equal(content, restored[name].content), "%s differs", name)
	}
	assert.Empty(t, encryptedHrefs(t, restored["manifest.json"].content))
	var originalJSON, restoredJSON any
	require.NoError(t, json.Unmarshal([]byte(testRWPManifest), &originalJSON))
	require.NoError(t, json.Unmarshal(restored["manifest.json"].content, &restoredJSON))
	assert.Equal(t, originalJSON, restoredJSON)
}

func TestEncryptRWPExploded(t *testing.T) {
	dir := explode(t, buildRWP(t))
	u, err := url.FromFilepath(dir)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), asset.FileWithMediaType(u, &mediatype.ReadiumWebpubManifest), testKey("x"), &buf, Options{}))
	output, names := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	assert.Equal(t, "manifest.json", names[0])
	assert.Len(t, encryptedHrefs(t, output["manifest.json"].content), 5)
}

func TestEncryptRWPCoverDeclaredInLinks(t *testing.T) {
	// The cover is listed in the resources without the "cover" relation,
	// which a link of `links` declares instead.
	m := strings.Replace(testRWPManifest,
		`{"href": "images/cover.jpg", "type": "image/jpeg", "rel": "cover"},`, ``, 1)
	m = strings.Replace(m,
		`{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "images/cover.jpg", "type": "image/jpeg"},`, 1)
	m = strings.Replace(m,
		`"links": [`,
		`"links": [{"rel": "cover", "href": "images/cover.jpg", "type": "image/jpeg"}, `, 1)
	require.NotEqual(t, testRWPManifest, m)

	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), &buf, Options{}))
	output, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	encrypted := encryptedHrefs(t, output["manifest.json"].content)
	assert.NotContains(t, encrypted, "images/cover.jpg")
	assert.Contains(t, encrypted, "images/figure.png")
	assert.Equal(t, testRWPFiles["images/cover.jpg"], output["images/cover.jpg"].content)
}

func TestEncryptRWPDeterministic(t *testing.T) {
	source := buildRWP(t)
	encrypt := func() []byte {
		var buf bytes.Buffer
		require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, source), testKey("det"), &buf, Options{Random: seededReader(9)}))
		return buf.Bytes()
	}
	assert.Equal(t, encrypt(), encrypt())
}

func TestEncryptRWPRefusesProtected(t *testing.T) {
	key := testKey("x")
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), key, &buf, Options{}))
	encrypted := writeTemp(t, "encrypted.webpub", buf.Bytes())
	err := EncryptRWP(t.Context(), fileAsset(t, encrypted), key, io.Discard, Options{})
	assert.ErrorIs(t, err, ErrAlreadyProtected)

	// A license alone marks the package as protected.
	withLicense := filepath.Join(explode(t, buildRWP(t)), "license.lcpl")
	require.NoError(t, os.WriteFile(withLicense, []byte("{}"), 0o644))
	u, err := url.FromFilepath(filepath.Dir(withLicense))
	require.NoError(t, err)
	err = EncryptRWP(t.Context(), asset.FileWithMediaType(u, &mediatype.ReadiumWebpubManifest), key, io.Discard, Options{})
	assert.ErrorIs(t, err, ErrAlreadyProtected)

	err = DecryptRWP(t.Context(), fileAsset(t, buildRWP(t)), key, io.Discard, Options{})
	assert.ErrorIs(t, err, ErrNotProtected)
}

func TestEncryptRWPOnlyEncryptsDeclarableResources(t *testing.T) {
	// A package file only referenced from `links` can't be declared as
	// encrypted, so even a selector accepting everything leaves it alone.
	m := strings.Replace(testRWPManifest,
		`"links": [`,
		`"links": [{"rel": "alternate", "href": "extra/unlisted.json", "type": "application/json"}, `, 1)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), &buf, Options{
		Encrypt: func(manifest.Link) bool { return true },
	}))
	output, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	assert.Equal(t, testRWPFiles["extra/unlisted.json"], output["extra/unlisted.json"].content)
	// The custom selector does encrypt the cover.
	assert.Contains(t, encryptedHrefs(t, output["manifest.json"].content), "images/cover.jpg")
}
