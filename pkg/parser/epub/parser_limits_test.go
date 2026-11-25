package epub

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zipFetcher returns a fetcher of a ZIP package holding the given files.
func zipFetcher(t *testing.T, files map[string][]byte) *fetcher.ArchiveFetcher {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.epub")
	out, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(out)
	for name, data := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, out.Close())
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(f.Close)
	return f
}

// oversizedXML is a valid XML document larger than MaxDocumentSize, which
// deflates to a few kilobytes.
var oversizedXML = []byte(`<root a="` + string(bytes.Repeat([]byte("a"), MaxDocumentSize)) + `"/>`)

const containerXML = `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`

func TestParserLimitsDocumentSize(t *testing.T) {
	// These documents were read whole, however much they inflated.
	_, err := GetRootFilePath(t.Context(), zipFetcher(t, map[string][]byte{"META-INF/container.xml": oversizedXML}))
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge, "container")

	_, err = NewParser(nil).Parse(t.Context(), fakeEPUBAsset{name: "test"}, zipFetcher(t, map[string][]byte{
		"META-INF/container.xml": []byte(containerXML),
		"OEBPS/content.opf":      oversizedXML,
	}))
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge, "package document")

	href := func(s string) url.URL {
		u, err := url.URLFromString(s)
		require.NoError(t, err)
		return u
	}
	epub3 := PackageDocument{EPUBVersion: 3, Manifest: []Item{{ID: "nav", Href: href("nav.xhtml"), Properties: []string{VocabularyItem + "nav"}}}}
	epub2 := PackageDocument{EPUBVersion: 2, Manifest: []Item{{ID: "ncx", Href: href("toc.ncx")}}, Spine: Spine{TOC: "ncx"}}
	f := zipFetcher(t, map[string][]byte{"nav.xhtml": oversizedXML, "toc.ncx": oversizedXML})
	_, err = parseNavigationData(t.Context(), epub3, f)
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge, "navigation document")
	_, err = parseNavigationData(t.Context(), epub2, f)
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge, "NCX")

	_, err = parseDisplayOptions(t.Context(), zipFetcher(t, map[string][]byte{"META-INF/com.apple.ibooks.display-options.xml": oversizedXML}))
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge, "display options")

	smil := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.smil", false)}
	service := MediaOverlayService{fetcher: zipFetcher(t, map[string][]byte{"chapter.smil": oversizedXML}), smilAlternates: map[string]manifest.Link{"chapter.xhtml": smil}}
	_, err = service.GuideForResource(t.Context(), "chapter.xhtml")
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge, "media overlay")

	// Missing or invalid optional documents are still ignored.
	invalid := zipFetcher(t, map[string][]byte{"nav.xhtml": []byte("<nav"), "META-INF/com.apple.ibooks.display-options.xml": []byte("<display")})
	nav, err := parseNavigationData(t.Context(), epub3, invalid)
	require.NoError(t, err)
	assert.Empty(t, nav)
	options, err := parseDisplayOptions(t.Context(), invalid)
	require.NoError(t, err)
	assert.Empty(t, options)
}

func TestGetRootFilePathInvalid(t *testing.T) {
	// A full-path which isn't a relative URL was reported with the error of
	// reading container.xml, a nil *fetcher.ResourceError, rather than its
	// own: the message said "no error".
	for _, fullPath := range []string{"c:x", "a:%zz"} {
		container := `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="` + fullPath + `" media-type="application/oebps-package+xml"/></rootfiles></container>`
		u, err := GetRootFilePath(t.Context(), zipFetcher(t, map[string][]byte{"META-INF/container.xml": []byte(container)}))
		assert.ErrorContains(t, err, "failed parsing rootfile full-path", fullPath)
		assert.ErrorContains(t, err, "not relative", fullPath)
		assert.Nil(t, u, fullPath)
	}
}
