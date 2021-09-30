package fetcher

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/stretchr/testify/assert"
)

func withArchiveFetcher(t *testing.T, callback func(a *ArchiveFetcher)) {
	a, err := NewArchiveFetcherFromPath("./testdata/epub.epub")
	assert.NoError(t, err)
	callback(a)
}

func TestArchiveFetcherLinks(t *testing.T) {
	makeTestLink := func(href string, typ string, entryLength uint64, isCompressed bool) pub.Link {
		return pub.Link{
			Href: href,
			Type: typ,
			Properties: pub.Properties{
				"https://readium.org/webpub-manifest/properties#archive": pub.Properties{
					"entryLength":       entryLength,
					"isEntryCompressed": isCompressed,
				},
			},
		}
	}

	mustContain := []pub.Link{
		makeTestLink("/mimetype", "", 20, false),
		makeTestLink("/EPUB/cover.xhtml", "application/xhtml+xml", 259, true),
		makeTestLink("/EPUB/css/epub.css", "text/css", 595, true),
		makeTestLink("/EPUB/css/nav.css", "text/css", 306, true),
		makeTestLink("/EPUB/images/cover.png", "image/png", 35809, true),
		makeTestLink("/EPUB/nav.xhtml", "application/xhtml+xml", 2293, true),
		makeTestLink("/EPUB/package.opf", "", 773, true),
		makeTestLink("/EPUB/s04.xhtml", "application/xhtml+xml", 118269, true),
		makeTestLink("/EPUB/toc.ncx", "", 1697, true),
		makeTestLink("/META-INF/container.xml", "text/xml", 176, true),
	}

	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		links, err := a.Links()
		assert.Nil(t, err)

		assert.ElementsMatch(t, mustContain, links)
	})
}

func TestArchiveFetcherLengthNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/unknown"})
		_, err := resource.Length()
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherReadNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/unknown"})
		_, err := resource.Read(0, 0)
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherRead(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/mimetype"})
		bin, err := resource.Read(0, 0)
		assert.Nil(t, err)
		assert.Equal(t, "application/epub+zip", string(bin))
	})
}

func TestArchiveFetcherReadRange(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/mimetype"})
		bin, err := resource.Read(0, 10)
		assert.Nil(t, err)
		assert.Equal(t, "application", string(bin))
	})
}

func TestArchiveFetcherComputingLength(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/mimetype"})
		length, err := resource.Length()
		assert.Nil(t, err)
		assert.EqualValues(t, 20, length)
	})
}

func TestArchiveFetcherDirectoryLengthNotFound(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/EPUB"})
		_, err := resource.Length()
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherFileNotFoundLength(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/unknown"})
		_, err := resource.Length()
		assert.Equal(t, NotFound(err.Cause), err)
	})
}

func TestArchiveFetcherAddsProperties(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/EPUB/css/epub.css"})
		assert.Equal(t, pub.Properties{
			"https://readium.org/webpub-manifest/properties#archive": pub.Properties{
				"entryLength":       uint64(595),
				"isEntryCompressed": true,
			},
		}, resource.Link().Properties)
	})
}

func TestArchiveFetcherOriginalPropertiesKept(t *testing.T) {
	withArchiveFetcher(t, func(a *ArchiveFetcher) {
		resource := a.Get(pub.Link{Href: "/EPUB/css/epub.css", Properties: pub.Properties{
			"other": "property",
		}})
		assert.Equal(t, pub.Properties{
			"other": "property",
			"https://readium.org/webpub-manifest/properties#archive": pub.Properties{
				"entryLength":       uint64(595),
				"isEntryCompressed": true,
			},
		}, resource.Link().Properties)
	})
}
