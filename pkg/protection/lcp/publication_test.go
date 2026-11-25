package lcp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openPublication(t *testing.T, path string, config streamer.Config) *pub.Publication {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	u, err := url.FromFilepath(abs)
	require.NoError(t, err)
	p, err := streamer.New(config).Open(t.Context(), asset.File(u), "")
	require.NoError(t, err, "failed opening %s", path)
	t.Cleanup(p.Close)
	return p
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

// localLinks returns the links of the reading order and resources (with
// their alternates) pointing inside the package.
func localLinks(m manifest.Manifest) []manifest.Link {
	var links []manifest.Link
	collect := func(l manifest.Link) {
		if !outsidePackage(l) {
			links = append(links, l)
		}
	}
	walkLinks(m.ReadingOrder, collect)
	walkLinks(m.Resources, collect)
	return links
}

func readResource(t *testing.T, p *pub.Publication, link manifest.Link) []byte {
	t.Helper()
	res := p.Get(t.Context(), link)
	defer res.Close()
	data, rerr := res.Read(t.Context(), 0, 0)
	require.Nil(t, rerr, "failed reading %s", link.Href)
	return data
}

// assertSameResources checks that every local resource of the expected
// publication reads identically through the actual one.
func assertSameResources(t *testing.T, expected, actual *pub.Publication) {
	t.Helper()
	links := localLinks(expected.Manifest)
	require.NotEmpty(t, links)
	for _, link := range links {
		assert.True(t, bytes.Equal(readResource(t, expected, link), readResource(t, actual, link)), "resource %s differs", link.Href)
	}
}

func identify(t *testing.T, path string) protection.Scheme {
	t.Helper()
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), path)
	require.NoError(t, err)
	defer f.Close()
	scheme, _, err := protection.IdentifyEPUBProtection(t.Context(), f)
	require.NoError(t, err)
	return scheme
}

func TestEPUBPublicationRoundTrip(t *testing.T) {
	key := testKey("publication")
	for _, path := range []string{
		"testdata/compat.epub",
		"testdata/media-overlays.epub",
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			original := openPublication(t, path, streamer.Config{})

			encrypted, err := EncryptPublication(t.Context(), original, key, Options{})
			require.NoError(t, err)

			// The selection follows the EPUB defaults.
			var count int
			for _, link := range localLinks(encrypted.Manifest) {
				if IsLCPEncrypted(link.Properties.Encryption()) {
					count++
					assert.False(t, bytes.Equal(readResource(t, encrypted, link), readResource(t, original, link)), "%s was served in the clear", link.Href)
					assert.NotContains(t, link.Rels, "cover")
					assert.NotContains(t, link.Rels, "contents")
				}
			}
			assert.NotZero(t, count)
			// The original publication is left untouched.
			for _, link := range localLinks(original.Manifest) {
				assert.False(t, IsLCPEncrypted(link.Properties.Encryption()), "original %s was modified", link.Href)
			}

			decrypted, err := DecryptPublication(encrypted, key)
			require.NoError(t, err)
			for _, link := range localLinks(decrypted.Manifest) {
				assert.False(t, IsLCPEncrypted(link.Properties.Encryption()), "%s still declares LCP encryption", link.Href)
			}
			assertSameResources(t, original, decrypted)
		})
	}
}

func TestEncryptPublicationRefusesProtected(t *testing.T) {
	protected := openPublication(t, "testdata/compat-old-lcp.epub", streamer.Config{})
	_, err := EncryptPublication(t.Context(), protected, testKey("x"), Options{})
	assert.ErrorIs(t, err, ErrAlreadyProtected)
}

func TestCheckUnprotected(t *testing.T) {
	withEnc := func(enc manifest.Encryption) manifest.Manifest {
		return manifest.Manifest{ReadingOrder: manifest.LinkList{{
			Href:       manifest.MustNewHREFFromString("a", false),
			Properties: manifest.Properties{"encrypted": enc.ToMap()},
		}}}
	}
	assert.NoError(t, checkUnprotected(manifest.Manifest{}))
	assert.NoError(t, checkUnprotected(withEnc(manifest.Encryption{Algorithm: protection.AlgorithmIDPFObfuscation})))
	assert.NoError(t, checkUnprotected(withEnc(manifest.Encryption{Algorithm: protection.AlgorithmAdobeObfuscation})))
	// Unknown encryption without a recognized DRM scheme, e.g. Yahoo's.
	assert.ErrorIs(t, checkUnprotected(withEnc(manifest.Encryption{Algorithm: "http://bookstore.yahoo.co.jp/ypub/1.0/enc"})), ErrAlreadyProtected)
	assert.ErrorIs(t, checkUnprotected(withEnc(manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC})), ErrAlreadyProtected)
}

func TestEncryptPublicationKeepsObfuscatedFonts(t *testing.T) {
	key := testKey("fonts")
	original := openPublication(t, "testdata/compat.epub", streamer.Config{})
	encrypted, err := EncryptPublication(t.Context(), original, key, Options{
		Encrypt: func(manifest.Link) bool { return true },
	})
	require.NoError(t, err)

	font := encrypted.Manifest.Resources.FirstWithHref(url.MustURLFromString("OEBPS/fonts/font.otf"))
	require.NotNil(t, font)
	require.NotNil(t, font.Properties.Encryption())
	assert.Equal(t, protection.AlgorithmIDPFObfuscation, font.Properties.Encryption().Algorithm)
	plain := readResource(t, original, *font)
	assert.Equal(t, plain, readResource(t, encrypted, *font))

	decrypted, err := DecryptPublication(encrypted, key)
	require.NoError(t, err)
	decryptedFont := decrypted.Manifest.Resources.FirstWithHref(font.URL(nil, nil))
	require.NotNil(t, decryptedFont)
	assert.Equal(t, font.Properties.Encryption(), decryptedFont.Properties.Encryption())
	assert.Equal(t, plain, readResource(t, decrypted, *decryptedFont))
}

func TestDecryptBuilderKeepsServices(t *testing.T) {
	key := testKey("lcp-compat")
	cleartext := openPublication(t, "testdata/compat.epub", streamer.Config{})
	decrypted := openPublication(t, "testdata/compat-old-lcp.epub", streamer.Config{
		OnCreatePublication: func(b *pub.Builder) error {
			return DecryptBuilder(b, key)
		},
	})
	assertSameResources(t, cleartext, decrypted)
	assert.NotEmpty(t, decrypted.Positions(t.Context()))
}

func TestEncryptPublicationWebPub(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"chapter.html": textBytes(5000),
		"page.jpg":     randomBytes(3000),
		"cover.jpg":    randomBytes(1000),
		"style.css":    textBytes(500),
		"figure.png":   randomBytes(2000),
	}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
	}
	link := func(href string, mt *mediatype.MediaType, rels ...string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false), MediaType: mt, Rels: rels}
	}
	m := manifest.Manifest{
		Metadata: manifest.Metadata{LocalizedTitle: manifest.NewLocalizedStringFromString("Test")},
		ReadingOrder: manifest.LinkList{
			link("cover.jpg", &mediatype.JPEG, "cover"),
			link("chapter.html", &mediatype.HTML),
			link("page.jpg", &mediatype.JPEG),
		},
		Resources: manifest.LinkList{link("style.css", &mediatype.CSS), link("figure.png", &mediatype.PNG)},
	}
	original := pub.New(m, fetcher.NewFileFetcher("", dir), nil)

	key := testKey("webpub")
	encrypted, err := EncryptPublication(t.Context(), original, key, Options{Random: seededReader(7)})
	require.NoError(t, err)

	expected := map[string]*manifest.Encryption{
		"cover.jpg":    nil,
		"chapter.html": {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, Compression: CompressionDeflate, OriginalLength: 5000},
		"page.jpg":     {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, OriginalLength: 3000},
		"style.css":    {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, Compression: CompressionDeflate, OriginalLength: 500},
		"figure.png":   {Scheme: Scheme, Algorithm: AlgorithmAES256CBC, OriginalLength: 2000},
	}
	d, err := NewDecryptor(key)
	require.NoError(t, err)
	for _, l := range localLinks(encrypted.Manifest) {
		name := l.Href.String()
		assert.Equal(t, expected[name], l.Properties.Encryption(), name)

		res := encrypted.Get(t.Context(), l)
		ciphertext, rerr := res.Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		if expected[name] == nil {
			assert.Equal(t, files[name], ciphertext, name)
			continue
		}
		assert.NotEqual(t, files[name], ciphertext, name)

		// The ciphertext is stable across reads, and matches its length.
		length, rerr := res.Length(t.Context())
		require.Nil(t, rerr)
		assert.EqualValues(t, len(ciphertext), length, name)
		again, rerr := res.Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		assert.Equal(t, ciphertext, again, name)
		var streamed bytes.Buffer
		_, rerr = res.Stream(t.Context(), &streamed, 0, 0)
		require.Nil(t, rerr)
		assert.Equal(t, ciphertext, streamed.Bytes(), name)
		part, rerr := res.Read(t.Context(), 10, 40)
		require.Nil(t, rerr)
		assert.Equal(t, ciphertext[10:41], part, name)

		plain, rerr := d.Transform(res).Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		assert.Equal(t, files[name], plain, name)
	}
}

func TestEncryptPublicationKeepsMediaOverlays(t *testing.T) {
	source := openPublication(t, "testdata/media-overlays.epub", streamer.Config{})
	encrypted, err := EncryptPublication(t.Context(), source, testKey("overlays"), Options{})
	require.NoError(t, err)

	// Each chapter must point to its own encrypted SMIL resource after
	// EncryptPublication replaces the streamer's media overlay service links.
	pairings := make(map[string]string)
	for _, chapter := range encrypted.Manifest.ReadingOrder {
		for _, alt := range chapter.Alternates {
			assert.False(t, isMediaOverlayServiceLink(alt), alt.Href)
			if !isSMIL(alt) {
				continue
			}
			target := chapter.URL(nil, nil).Path()
			require.NotContains(t, pairings, target)
			pairings[target] = alt.URL(nil, nil).Path()
			assert.True(t, IsLCPEncrypted(alt.Properties.Encryption()), alt.Href)
			resource := encrypted.Manifest.Resources.FirstWithHref(alt.URL(nil, nil))
			require.NotNil(t, resource)
			assert.Equal(t, alt.Properties.Encryption(), resource.Properties.Encryption())
		}
	}
	assert.Equal(t, map[string]string{
		"OPS/xhtml/一.xhtml": "OPS/xhtml/一.smil",
		"OPS/xhtml/二.xhtml": "OPS/xhtml/二.smil",
	}, pairings)

	// The source still advertises its two media overlay service links.
	var serviceLinks int
	for _, l := range source.Manifest.ReadingOrder {
		for _, alt := range l.Alternates {
			assert.True(t, isMediaOverlayServiceLink(alt), alt.Href)
			serviceLinks++
		}
	}
	assert.Equal(t, 2, serviceLinks)
}

func TestLicensedEPUBServesMediaOverlays(t *testing.T) {
	packed := encryptEPUBToFile(t, "testdata/media-overlays.epub", testKey("lcp-compat"), Options{})
	var licensed bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, packed), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}))
	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{InsecureSkipVerify: true}},
	}).Open(t.Context(), fileAsset(t, writeTemp(t, "licensed.epub", licensed.Bytes())), testPassphrase)
	require.NoError(t, err)
	defer p.Close()
	overlays := 0
	for _, l := range p.Manifest.ReadingOrder {
		for _, alt := range l.Alternates {
			if !isMediaOverlayServiceLink(alt) {
				continue
			}
			overlays++
			data, rerr := p.Get(t.Context(), alt).Read(t.Context(), 0, 0)
			require.Nil(t, rerr, alt.Href)
			assert.Contains(t, string(data), "audioref", alt.Href)
		}
	}
	assert.Equal(t, 2, overlays)
}

func TestEncryptPublicationSkipsServiceLinks(t *testing.T) {
	// Links to publication services, e.g. left by a service in the manifest
	// of a parsed publication, are not resources of the package.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
	chapter := manifest.Link{
		Href:      manifest.MustNewHREFFromString("chapter.html", false),
		MediaType: &mediatype.HTML,
		Alternates: manifest.LinkList{
			{Href: manifest.MustNewHREFFromString("~readium/guided-navigation.json?ref=chapter.html", false)},
			{Href: manifest.MustNewHREFFromString("~readium/media-overlay.json{?ref}", true)},
		},
	}
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}}
	encrypted, err := EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), testKey("services"), Options{})
	require.NoError(t, err)
	link := encrypted.Manifest.ReadingOrder[0]
	assert.NotNil(t, link.Properties.Encryption())
	for _, alt := range link.Alternates {
		assert.Nil(t, alt.Properties.Encryption(), alt.Href)
	}
}

type locatedChapter struct {
	fetcher.Resource
	source url.AbsoluteURL
}

func (r locatedChapter) Location() url.AbsoluteURL { return r.source }

func TestPublicationsRequirePinnableURLs(t *testing.T) {
	link := manifest.Link{Href: manifest.MustNewHREFFromString("chapter", false)}
	source, err := url.AbsoluteURLFromString("https://example.org/chapter")
	require.NoError(t, err)
	f := fetcher.NewTransformingFetcher(servingFetcher{data: []byte("chapter")}, func(r fetcher.Resource) fetcher.Resource {
		if r.Link().Href.String() == "chapter" {
			return locatedChapter{r, source}
		}
		return r
	})
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{link}}, f, nil)
	key := testKey("publication")
	_, err = EncryptPublication(t.Context(), p, key, Options{})
	require.ErrorContains(t, err, "cannot pin its source URL")
	decrypted, err := DecryptPublication(p, key)
	require.NoError(t, err)
	res := decrypted.Get(t.Context(), link)
	defer res.Close()
	data, rerr := res.Read(t.Context(), 0, 0)
	require.NotNil(t, rerr)
	assert.ErrorContains(t, rerr, "cannot pin its source URL")
	assert.Empty(t, data)
}
