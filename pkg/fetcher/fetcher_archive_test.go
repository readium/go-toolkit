package fetcher

import (
	"bytes"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withArchiveFetcher(t *testing.T, callback func(a *ArchiveFetcher)) {
	a, err := NewArchiveFetcherFromPath(t.Context(), "./testdata/epub.epub")
	require.NoError(t, err)
	callback(a)
}

func TestArchiveFetcherLinks(t *testing.T) {
	makeTestLink := func(href string, typ *mediatype.MediaType, entryLength uint64, isCompressed bool) struct {
		manifest.Link
		manifest.Properties
	} {

		l := manifest.Link{
			Href:      manifest.MustNewHREFFromString(href, false),
			MediaType: typ,
		}
		p := manifest.Properties{
			"https://readium.org/webpub-manifest/properties#archive": map[string]interface{}{
				"entryLength":       entryLength,
				"isEntryCompressed": isCompressed,
			},
		}
		return struct {
			manifest.Link
			manifest.Properties
		}{l, p}
	}

	mustContain := []struct {
		manifest.Link
		manifest.Properties
	}{
		makeTestLink("mimetype", nil, 20, false),
		makeTestLink("EPUB/cover.xhtml", &mediatype.XHTML, 259, true),
		makeTestLink("EPUB/css/epub.css", &mediatype.CSS, 595, true),
		makeTestLink("EPUB/css/nav.css", &mediatype.CSS, 306, true),
		makeTestLink("EPUB/images/cover.png", &mediatype.PNG, 35809, true),
		makeTestLink("EPUB/nav.xhtml", &mediatype.XHTML, 2293, true),
		makeTestLink("EPUB/package.opf", &mediatype.OPF, 773, true),
		makeTestLink("EPUB/s04.xhtml", &mediatype.XHTML, 118269, true),
		makeTestLink("EPUB/toc.ncx", &mediatype.NCX, 1697, true),
		makeTestLink("META-INF/container.xml", &mediatype.XML, 176, true),
	}

	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		links, err := a.Links(t.Context())
		require.Nil(t, err)

		mustLinks := make([]manifest.Link, len(mustContain))
		for i, l := range mustContain {
			assert.Equal(t, l.Properties, a.Get(t.Context(), l.Link).Properties())
			mustLinks[i] = l.Link
		}
		assert.ElementsMatch(t, mustLinks, links)
	})
}

func TestArchiveFetcherLengthNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("unknown", false)})
		_, err := resource.Length(t.Context())
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherReadNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("unknown", false)})
		_, err := resource.Read(t.Context(), 0, 0)
		assert.Equal(t, NotFound(err.Cause), err)
		_, err = resource.Stream(t.Context(), &bytes.Buffer{}, 0, 0)
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherRead(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("mimetype", false)})
		bin, err := resource.Read(t.Context(), 0, 0)
		require.Nil(t, err)
		assert.Equal(t, "application/epub+zip", string(bin))
		var b bytes.Buffer
		n, err := resource.Stream(t.Context(), &b, 0, 0)
		require.Nil(t, err)
		assert.EqualValues(t, 20, n)
		assert.Equal(t, "application/epub+zip", b.String())
	})
}

func TestArchiveFetcherReadRange(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("mimetype", false)})
		bin, err := resource.Read(t.Context(), 0, 10)
		require.Nil(t, err)
		assert.Equal(t, "application", string(bin))
		var b bytes.Buffer
		n, err := resource.Stream(t.Context(), &b, 0, 10)
		require.Nil(t, err)
		assert.EqualValues(t, 11, n)
		assert.Equal(t, "application", b.String())
	})
}

func TestArchiveFetcherComputingLength(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("mimetype", false)})
		length, err := resource.Length(t.Context())
		require.Nil(t, err)
		assert.EqualValues(t, 20, length)
	})
}

func TestArchiveFetcherDirectoryLengthNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("EPUB", false)})
		_, err := resource.Length(t.Context())
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherFileNotFoundLength(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("unknown", false)})
		_, err := resource.Length(t.Context())
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherAddsProperties(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("EPUB/css/epub.css", false)})
		assert.Equal(t, manifest.Properties{
			"https://readium.org/webpub-manifest/properties#archive": map[string]interface{}{
				"entryLength":       uint64(595),
				"isEntryCompressed": true,
			},
		}, resource.Properties())
	})
}
