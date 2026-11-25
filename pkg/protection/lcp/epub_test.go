package lcp

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	epubparser "github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type zipEntry struct {
	method  uint16
	crc     uint32
	raw     []byte // stored bytes, compressed or not
	content []byte
}

// readZip returns the entries of a ZIP file, and their names in order.
func readZip(t *testing.T, path string) (map[string]zipEntry, []string) {
	t.Helper()
	r, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer func() { assert.NoError(t, r.Close()) }()
	entries := make(map[string]zipEntry)
	var names []string
	for _, f := range r.File {
		raw, err := f.OpenRaw()
		require.NoError(t, err)
		rawBytes, err := io.ReadAll(raw)
		require.NoError(t, err)
		rc, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		entries[f.Name] = zipEntry{method: f.Method, crc: f.CRC32, raw: rawBytes, content: content}
		names = append(names, f.Name)
	}
	return entries, names
}

func fileAsset(t *testing.T, path string) asset.PublicationAsset {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	u, err := url.FromFilepath(abs)
	require.NoError(t, err)
	return asset.File(u)
}

// encryptionEntries parses the encryption.xml of an EPUB file.
func encryptionEntries(t *testing.T, path string) map[string]manifest.Encryption {
	t.Helper()
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), path)
	require.NoError(t, err)
	defer f.Close()
	doc, rerr := fetcher.ReadResourceAsXML(t.Context(), f.Get(t.Context(), manifest.Link{
		Href: manifest.MustNewHREFFromString(protection.PathEPUBEncryption, false),
	}))
	if rerr != nil {
		require.Equal(t, fetcher.CodeNotFound, rerr.Code)
		return nil
	}
	return epubparser.ParseEncryption(doc, protection.SchemeLCP)
}

func encryptEPUBToFile(t *testing.T, path string, key ContentKey, opts Options) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, EncryptEPUB(t.Context(), fileAsset(t, path), key, &buf, opts))
	return writeTemp(t, "encrypted.epub", buf.Bytes())
}

func decryptEPUBToFile(t *testing.T, path string, key ContentKey) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, DecryptEPUB(t.Context(), fileAsset(t, path), key, &buf, Options{}))
	return writeTemp(t, "decrypted.epub", buf.Bytes())
}

func TestEncryptEPUB(t *testing.T) {
	key := testKey("epub")
	for _, path := range []string{
		"testdata/compat.epub",
		"testdata/media-overlays.epub",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			encrypted := encryptEPUBToFile(t, path, key, Options{})
			assert.Equal(t, protection.LCP, identify(t, encrypted))

			source, sourceNames := readZip(t, path)
			output, outputNames := readZip(t, encrypted)
			require.Equal(t, "mimetype", outputNames[0])
			assert.EqualValues(t, zip.Store, output["mimetype"].method)

			// Every entry is carried over, and nothing else is added.
			expectedNames := map[string]bool{protection.PathEPUBEncryption: true}
			for _, name := range sourceNames {
				expectedNames[name] = true
			}
			actualNames := make(map[string]bool)
			for _, name := range outputNames {
				actualNames[name] = true
			}
			assert.Equal(t, expectedNames, actualNames)

			sourceEnc := encryptionEntries(t, path)
			outputEnc := encryptionEntries(t, encrypted)
			lcpEncrypted := make(map[string]bool)
			for href, enc := range outputEnc {
				if IsLCPEncrypted(&enc) {
					lcpEncrypted[containerPath(mustDecode(t, href))] = true
				} else {
					assert.Equal(t, sourceEnc[href], enc, "existing entry %s", href)
				}
			}
			require.NotEmpty(t, lcpEncrypted)

			// The cleartext resources keep their exact bytes and method.
			original := openPublication(t, path, streamer.Config{})
			for name, src := range source {
				if name == protection.PathEPUBEncryption || name == "mimetype" {
					continue
				}
				out := output[name]
				if lcpEncrypted[containerPath(name)] {
					assert.EqualValues(t, zip.Store, out.method, name)
					assert.NotEqual(t, src.content, out.content, name)
					continue
				}
				assert.Equal(t, src.method, out.method, name)
				assert.Equal(t, src.crc, out.crc, name)
				assert.True(t, bytes.Equal(src.raw, out.raw), "%s was not copied verbatim", name)
			}

			// The resources reading systems need while locked stay in the clear.
			for _, rel := range []string{"cover", "contents"} {
				for _, l := range original.Manifest.LinksWithRel(rel) {
					assert.False(t, lcpEncrypted[resourceKey(l.Href)], "%s (%s) is encrypted", l.Href, rel)
				}
			}

			// Reading through the decryptor yields the original resources.
			decrypted := openPublication(t, encrypted, streamer.Config{
				OnCreatePublication: func(b *pub.Builder) error { return DecryptBuilder(b, key) },
			})
			assertSameResources(t, original, decrypted)

			// Decrypting the EPUB restores every entry.
			restored, restoredNames := readZip(t, decryptEPUBToFile(t, encrypted, key))
			assert.Equal(t, "mimetype", restoredNames[0])
			for name, src := range source {
				if name == protection.PathEPUBEncryption {
					continue
				}
				assert.True(t, bytes.Equal(src.content, restored[name].content), "%s differs after decryption", name)
			}
			_, hasEncryption := restored[protection.PathEPUBEncryption]
			assert.Equal(t, sourceEnc != nil, hasEncryption)
		})
	}
}

func mustDecode(t *testing.T, href string) string {
	t.Helper()
	u, err := url.URLFromString(href)
	require.NoError(t, err)
	return u.Path()
}

// explode extracts a ZIP file into a new directory.
func explode(t *testing.T, path string) string {
	t.Helper()
	dir := t.TempDir()
	entries, _ := readZip(t, path)
	for name, e := range entries {
		target := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, e.content, 0o644))
	}
	return dir
}

func TestEncryptEPUBExploded(t *testing.T) {
	key := testKey("exploded")
	u, err := url.FromFilepath(explode(t, "testdata/compat.epub"))
	require.NoError(t, err)
	// A directory can't be sniffed as an EPUB: its media type must be given.
	exploded := asset.FileWithMediaType(u, &mediatype.EPUB)

	var buf bytes.Buffer
	require.NoError(t, EncryptEPUB(t.Context(), exploded, key, &buf, Options{}))
	encrypted := writeTemp(t, "encrypted.epub", buf.Bytes())
	assert.Equal(t, protection.LCP, identify(t, encrypted))
	assert.Equal(t, encryptionEntries(t, "testdata/compat-old-lcp.epub"), encryptionEntries(t, encrypted))

	original := openPublication(t, "testdata/compat.epub", streamer.Config{})
	decrypted, err := DecryptPublication(openPublication(t, encrypted, streamer.Config{}), key)
	require.NoError(t, err)
	assertSameResources(t, original, decrypted)
}

func TestEncryptEPUBCustomSelection(t *testing.T) {
	encrypted := encryptEPUBToFile(t, "testdata/compat.epub", testKey("custom"), Options{
		// Tries to encrypt everything, including container files.
		Encrypt:  func(manifest.Link) bool { return true },
		Compress: func(manifest.Link) bool { return false },
	})
	enc := encryptionEntries(t, encrypted)
	for _, href := range []string{"OEBPS/chapter1.xhtml", "OEBPS/nav.xhtml", "OEBPS/images/cover.jpg"} {
		require.Contains(t, enc, href)
		assert.NotEqual(t, CompressionDeflate, enc[href].Compression, href)
	}
	// Container files and obfuscated fonts are never encrypted.
	for _, href := range []string{"OEBPS/content.opf", "META-INF/container.xml"} {
		assert.NotContains(t, enc, href)
	}
	assert.Equal(t, "http://www.idpf.org/2008/embedding", enc["OEBPS/fonts/font.otf"].Algorithm)
}

func TestEncryptEPUBDeterministic(t *testing.T) {
	encrypt := func() []byte {
		var buf bytes.Buffer
		require.NoError(t, EncryptEPUB(t.Context(), fileAsset(t, "testdata/compat.epub"), testKey("det"), &buf, Options{
			Random: seededReader(3),
		}))
		return buf.Bytes()
	}
	assert.Equal(t, encrypt(), encrypt())
}

func TestEncryptEPUBRefusesProtected(t *testing.T) {
	for _, path := range []string{
		"testdata/compat-old-lcp.epub",
		"../testdata/fake-lcp.epub",
		"../testdata/fake-adept.epub",
		"../testdata/fake-fairplay.epub",
		// Proprietary encryption declared in encryption.xml, which is not
		// recognized as DRM but can't be carried over either.
		"../testdata/yahoo.ypub",
	} {
		err := EncryptEPUB(t.Context(), fileAsset(t, path), testKey("x"), io.Discard, Options{})
		assert.ErrorIs(t, err, ErrAlreadyProtected, path)
	}
}

func TestEncryptEPUBKeepsObfuscatedFonts(t *testing.T) {
	key := testKey("fonts")
	const font = "OEBPS/fonts/font.otf"
	// Even when asked to encrypt everything.
	encrypted := encryptEPUBToFile(t, "testdata/compat.epub", key, Options{
		Encrypt: func(manifest.Link) bool { return true },
	})

	// The obfuscated font entry is copied verbatim, still obfuscated with
	// the identifier-derived key, and keeps its encryption.xml entry.
	source, _ := readZip(t, "testdata/compat.epub")
	output, _ := readZip(t, encrypted)
	assert.Equal(t, source[font].method, output[font].method)
	assert.True(t, bytes.Equal(source[font].raw, output[font].raw))
	assert.Equal(t, manifest.Encryption{Algorithm: protection.AlgorithmIDPFObfuscation}, encryptionEntries(t, encrypted)[font])

	// Reading systems deobfuscate it as before.
	link := manifest.Link{Href: manifest.MustNewHREFFromString(font, false)}
	original := openPublication(t, "testdata/compat.epub", streamer.Config{})
	decrypted := openPublication(t, encrypted, streamer.Config{
		OnCreatePublication: func(b *pub.Builder) error { return DecryptBuilder(b, key) },
	})
	assert.Equal(t, readResource(t, original, link), readResource(t, decrypted, link))
}

func TestDecryptEPUBRefusesUnprotected(t *testing.T) {
	err := DecryptEPUB(t.Context(), fileAsset(t, "testdata/compat.epub"), testKey("x"), io.Discard, Options{})
	assert.ErrorIs(t, err, ErrNotProtected)
}

func TestDecryptEPUBWrongKey(t *testing.T) {
	err := DecryptEPUB(t.Context(), fileAsset(t, "testdata/compat-old-lcp.epub"), testKey("wrong"), io.Discard, Options{})
	assert.Error(t, err)
}
