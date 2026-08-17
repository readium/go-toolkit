package pub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/guidednavigation/converter"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
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

func gnHTMLLink(href string, alternates ...manifest.Link) manifest.Link {
	return manifest.Link{
		Href:       manifest.MustNewHREFFromString(href, false),
		MediaType:  &mediatype.XHTML,
		Alternates: alternates,
	}
}

const gnTestDoc = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Test</title></head>
<body><p>Hello <img src="img.png" alt="An image"/> world</p></body>
</html>`

// The guided navigation service converts (X)HTML documents of the reading order
// and resources, leaves SMIL alternates alone, and does not touch the manifest.
func TestHTMLGuidedNavigationService(t *testing.T) {
	smilAlt := manifest.Link{
		Href:      manifest.MustNewHREFFromString("a.smil", false),
		MediaType: &mediatype.SMIL,
	}
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{
			gnHTMLLink("a.xhtml", smilAlt),
			{Href: manifest.MustNewHREFFromString("audio/a.mp3", false), MediaType: &mediatype.MPEGAudio},
			gnHTMLLink("b.xhtml"),
		},
		Resources: manifest.LinkList{gnHTMLLink("cover.xhtml")},
	}
	f := stringFetcher{"a.xhtml": gnTestDoc, "b.xhtml": gnTestDoc, "cover.xhtml": gnTestDoc}
	service, ok := HTMLGuidedNavigationServiceFactory()(NewContext(m, f), false).(*HTMLGuidedNavigationService)
	require.True(t, ok)

	assert.True(t, service.HasGuideForResource("a.xhtml"))
	assert.True(t, service.HasGuideForResource("cover.xhtml"))
	assert.False(t, service.HasGuideForResource("audio/a.mp3"))

	// The manifest is untouched: the SMIL alternate is still there
	require.Len(t, m.ReadingOrder[0].Alternates, 1)
	assert.Equal(t, "a.smil", m.ReadingOrder[0].Alternates[0].Href.String())

	// A document with a SMIL overlay is still converted from its (X)HTML content
	doc, err := service.GuideForResource(context.Background(), "a.xhtml")
	require.NoError(t, err)
	require.NotEmpty(t, doc.Guided)
	assert.Contains(t, doc.Guided[0].Role, guidednavigation.RoleBody)

	// Reading order documents chain along the reading order's HTML items
	require.Len(t, doc.Links, 1)
	assert.Contains(t, doc.Links[0].Rels, "next")
	assert.Equal(t, "~readium/guided-navigation.json?ref=b.xhtml", doc.Links[0].Href.String())

	// Resources aren't part of the chain
	doc, err = service.GuideForResource(context.Background(), "cover.xhtml")
	require.NoError(t, err)
	assert.Empty(t, doc.Links)
}

// The service link is advertised when public, and Get is gated on it.
func TestHTMLGuidedNavigationServiceLinks(t *testing.T) {
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{gnHTMLLink("a.xhtml")}}
	f := stringFetcher{"a.xhtml": gnTestDoc}
	gnLink := manifest.Link{
		Href: manifest.MustNewHREFFromString("~readium/guided-navigation.json?ref=a.xhtml", false),
	}

	private := HTMLGuidedNavigationServiceFactory()(NewContext(m, f), false)
	assert.Empty(t, private.Links())
	_, ok := private.Get(context.Background(), gnLink)
	assert.False(t, ok)

	public := HTMLGuidedNavigationServiceFactory()(NewContext(m, f), true)
	assert.Equal(t, manifest.LinkList{GuidedNavigationLink}, public.Links())
	res, ok := public.Get(context.Background(), gnLink)
	require.True(t, ok)
	data, rerr := res.Read(context.Background(), 0, 0)
	require.Nil(t, rerr)
	assert.NotEmpty(t, data)
}

// Converter options are threaded through, e.g. textref locators.
func TestHTMLGuidedNavigationServiceConverterOptions(t *testing.T) {
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{gnHTMLLink("a.xhtml")}}
	f := stringFetcher{"a.xhtml": gnTestDoc}
	service, ok := HTMLGuidedNavigationServiceFactory(converter.WithTextRefLocators())(NewContext(m, f), false).(*HTMLGuidedNavigationService)
	require.True(t, ok)

	doc, err := service.GuideForResource(context.Background(), "a.xhtml")
	require.NoError(t, err)
	require.NotEmpty(t, doc.Guided)
	require.NotEmpty(t, doc.Guided[0].Children)
	assert.Equal(t, "body > p", doc.Guided[0].Children[0].TextCSSSelector())
}

// Without (X)HTML documents there is no guided navigation service at all.
func TestHTMLGuidedNavigationServiceAbsentWithoutHTML(t *testing.T) {
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{{
			Href:      manifest.MustNewHREFFromString("audio/a.mp3", false),
			MediaType: &mediatype.MPEGAudio,
		}},
	}
	assert.Nil(t, HTMLGuidedNavigationServiceFactory()(NewContext(m, stringFetcher{}), false))
}
