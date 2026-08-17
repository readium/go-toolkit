package epub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An in-memory fetcher serving string documents by href.
type stringFetcher map[string]string

func (f stringFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	return nil, nil
}

func (f stringFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	doc, ok := f[link.Href.String()]
	if !ok {
		return fetcher.NewFailureResource(link, fetcher.NotFound(nil))
	}
	return fetcher.NewBytesResource(link, func() []byte {
		return []byte(doc)
	})
}

func (f stringFetcher) Close() {}

func htmlLink(href string, alternates ...manifest.Link) manifest.Link {
	return manifest.Link{
		Href:       manifest.MustNewHREFFromString(href, false),
		MediaType:  &mediatype.XHTML,
		Alternates: alternates,
	}
}

func smilAlt(href string) manifest.Link {
	return manifest.Link{
		Href:      manifest.MustNewHREFFromString(href, false),
		MediaType: &mediatype.SMIL,
	}
}

const moTestSmil = `<?xml version="1.0" encoding="UTF-8"?>
<smil xmlns="http://www.w3.org/ns/SMIL" version="3.0"><body>
<par id="par1"><text src="a.xhtml#p1"/><audio src="audio/a.mp3" clipBegin="0s" clipEnd="1s"/></par>
</body></smil>`

// The media overlay service only covers resources with SMIL alternates: it swaps
// them for media overlay links, stays out of the manifest's links, and does not
// fall back to converting (X)HTML content.
func TestMediaOverlayServiceSMILOnly(t *testing.T) {
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{htmlLink("a.xhtml", smilAlt("a.smil")), htmlLink("b.xhtml")},
	}
	f := stringFetcher{"a.smil": moTestSmil}
	service, ok := MediaOverlayFactory()(pub.NewContext(m, f), false).(*MediaOverlayService)
	require.True(t, ok)

	assert.True(t, service.HasGuideForResource("a.xhtml"))
	// No fallback to HTML conversion
	assert.False(t, service.HasGuideForResource("b.xhtml"))
	// Never advertised in the manifest's links
	assert.Empty(t, service.Links())

	// The SMIL alternate was swapped for a media overlay link
	require.Len(t, m.ReadingOrder[0].Alternates, 1)
	assert.Equal(t, "~readium/media-overlay.json?ref=a.xhtml", m.ReadingOrder[0].Alternates[0].Href.String())

	doc, err := service.GuideForResource(context.Background(), "a.xhtml")
	require.NoError(t, err)
	require.NotEmpty(t, doc.Guided)
	assert.Equal(t, "a.xhtml#p1", doc.Guided[0].TextRef.String())
	// The only overlay in the reading order: no next/prev chain
	assert.Empty(t, doc.Links)

	// The swapped alternate links must be servable even when the service isn't public
	res, ok := service.Get(context.Background(), manifest.Link{
		Href: manifest.MustNewHREFFromString("~readium/media-overlay.json?ref=a.xhtml", false),
	})
	require.True(t, ok)
	data, rerr := res.Read(context.Background(), 0, 0)
	require.Nil(t, rerr)
	assert.NotEmpty(t, data)
}

// Overlays chain through next/prev media overlay links along the reading order.
func TestMediaOverlayServiceChain(t *testing.T) {
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{
			htmlLink("a.xhtml", smilAlt("a.smil")),
			htmlLink("b.xhtml", smilAlt("b.smil")),
		},
	}
	f := stringFetcher{"a.smil": moTestSmil, "b.smil": moTestSmil}
	service, ok := MediaOverlayFactory()(pub.NewContext(m, f), false).(*MediaOverlayService)
	require.True(t, ok)

	doc, err := service.GuideForResource(context.Background(), "a.xhtml")
	require.NoError(t, err)
	require.Len(t, doc.Links, 1)
	assert.Contains(t, doc.Links[0].Rels, "next")
	assert.Equal(t, "~readium/media-overlay.json?ref=b.xhtml", doc.Links[0].Href.String())
}

// Without SMIL alternates there is no media overlay service at all.
func TestMediaOverlayServiceAbsentWithoutSMIL(t *testing.T) {
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{htmlLink("a.xhtml"), htmlLink("b.xhtml")},
	}
	assert.Nil(t, MediaOverlayFactory()(pub.NewContext(m, stringFetcher{}), false))
}
