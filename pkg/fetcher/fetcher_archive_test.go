package fetcher

import (
	"bytes"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func withArchiveFetcher(t *testing.T, callback func(a *ArchiveFetcher)) {
	a, err := NewArchiveFetcherFromPath("./testdata/epub.epub")
	assert.NoError(t, err)
	callback(a)
}

func TestArchiveFetcherLinks(t *testing.T) {
	makeTestLink := func(href string, typ string, entryLength uint64, isCompressed bool) struct {
		manifest.Link
		manifest.Properties
	} {
		l := manifest.Link{
			Href: href,
			Type: typ,
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
		makeTestLink("/mimetype", "", 20, false),
		makeTestLink("/EPUB/cover.xhtml", "application/xhtml+xml", 259, true),
		makeTestLink("/EPUB/css/epub.css", "text/css", 595, true),
		makeTestLink("/EPUB/css/nav.css", "text/css", 306, true),
		makeTestLink("/EPUB/images/cover.png", "image/png", 35809, true),
		makeTestLink("/EPUB/nav.xhtml", "application/xhtml+xml", 2293, true),
		makeTestLink("/EPUB/package.opf", "application/oebps-package+xml", 773, true),
		makeTestLink("/EPUB/s04.xhtml", "application/xhtml+xml", 118269, true),
		makeTestLink("/EPUB/toc.ncx", "application/x-dtbncx+xml", 1697, true),
		makeTestLink("/META-INF/container.xml", "application/xml", 176, true),
	}

	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		links, err := a.Links()
		assert.Nil(t, err)

		mustLinks := make([]manifest.Link, len(mustContain))
		for i, l := range mustContain {
			assert.Equal(t, l.Properties, a.Get(l.Link).Properties())
			mustLinks[i] = l.Link
		}
		assert.ElementsMatch(t, mustLinks, links)
	})
}

func TestArchiveFetcherLengthNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/unknown"})
		_, err := resource.Length()
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherReadNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/unknown"})
		_, err := resource.Read(0, 0)
		assert.Equal(t, NotFound(err.Cause), err)
		_, err = resource.Stream(&bytes.Buffer{}, 0, 0)
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherRead(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/mimetype"})
		bin, err := resource.Read(0, 0)
		if assert.Nil(t, err) {
			assert.Equal(t, "application/epub+zip", string(bin))
		}
		var b bytes.Buffer
		n, err := resource.Stream(&b, 0, 0)
		if assert.Nil(t, err) {
			assert.EqualValues(t, 20, n)
			assert.Equal(t, "application/epub+zip", b.String())
		}
	})
}

func TestArchiveFetcherReadRange(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/mimetype"})
		bin, err := resource.Read(0, 10)
		if assert.Nil(t, err) {
			assert.Equal(t, "application", string(bin))
		}
		var b bytes.Buffer
		n, err := resource.Stream(&b, 0, 10)
		if assert.Nil(t, err) {
			assert.EqualValues(t, 11, n)
			assert.Equal(t, "application", b.String())
		}
	})
}

func TestArchiveFetcherComputingLength(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/mimetype"})
		length, err := resource.Length()
		assert.Nil(t, err)
		assert.EqualValues(t, 20, length)
	})
}

func TestArchiveFetcherDirectoryLengthNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/EPUB"})
		_, err := resource.Length()
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherFileNotFoundLength(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/unknown"})
		_, err := resource.Length()
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherAddsProperties(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(manifest.Link{Href: "/EPUB/css/epub.css"})
		assert.Equal(t, manifest.Properties{
			"https://readium.org/webpub-manifest/properties#archive": map[string]interface{}{
				"entryLength":       uint64(595),
				"isEntryCompressed": true,
			},
		}, resource.Properties())
	})
}
