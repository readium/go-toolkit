package epub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadPackageDoc(ctx context.Context, name string) (*manifest.Manifest, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/package/"+name+".opf"), map[string]string{
		NamespaceOPF:                         "opf",
		NamespaceDC:                          "dc",
		VocabularyDCTerms:                    "dcterms",
		"http://www.idpf.org/2013/rendition": "rendition",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	d, err := ParsePackageDocument(n, url.MustURLFromString("OEBPS/content.opf"))
	if err != nil {
		return nil, err
	}

	manifest := PublicationFactory{
		FallbackTitle:   "fallback title",
		PackageDocument: *d,
	}.Create()

	return &manifest, nil
}

func TestPackageDocReadingProgressionNoneIsAuto(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "progression-none")
	require.NoError(t, err)
	assert.Equal(t, manifest.None, p.Metadata.ReadingProgression)
}

func TestPackageDocPageProgression(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "progression-default")
	require.NoError(t, err)
	assert.Equal(t, manifest.None, p.Metadata.ReadingProgression)
}

func TestPackageDocPageProgressionLTR(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "progression-ltr")
	require.NoError(t, err)
	assert.Equal(t, manifest.LTR, p.Metadata.ReadingProgression)
}

func TestPackageDocPageProgressionRTL(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "progression-rtl")
	require.NoError(t, err)
	assert.Equal(t, manifest.RTL, p.Metadata.ReadingProgression)
}

func TestPackageDocLinkPropertiesContains(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "links-properties")
	require.NoError(t, err)
	ro := p.ReadingOrder
	assert.Equal(t, []string{"mathml"}, ro[0].Properties.Contains())
	assert.Equal(t, []string{"remote-resources"}, ro[1].Properties.Contains())
	assert.Equal(t, []string{"js", "svg"}, ro[2].Properties.Contains())
	assert.Empty(t, ro[3].Properties.Contains())
	assert.Empty(t, ro[4].Properties.Contains())
}

func TestPackageDocLinkPropertiesRels(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "links-properties")
	require.NoError(t, err)
	ro := p.ReadingOrder
	assert.Equal(t, manifest.Strings{"cover"}, p.Resources[0].Rels)
	assert.Empty(t, ro[0].Rels)
	assert.Empty(t, ro[1].Rels)
	assert.Empty(t, ro[2].Rels)
	assert.Equal(t, manifest.Strings{"contents"}, ro[3].Rels)
	assert.Empty(t, ro[4].Rels)
}

func TestPackageDocLinkPropertiesPresentation(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "links-properties")
	require.NoError(t, err)
	ro := p.ReadingOrder
	assert.Equal(t, ro[0].Properties.Page(), manifest.PageRight)
	assert.Equal(t, ro[2].Properties.Page(), manifest.PageCenter)
	assert.Equal(t, ro[3].Properties.Page(), manifest.PageNone)
}

func TestPackageDocLinkReadingOrder(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "links")
	require.NoError(t, err)

	assert.Equal(t, manifest.LinkList{
		{
			Href:      manifest.MustNewHREFFromString("titlepage.xhtml", false),
			MediaType: &mediatype.XHTML,
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/chapter01.xhtml", false),
			MediaType: &mediatype.XHTML,
		},
	}, p.ReadingOrder)
}

func TestPackageDocLinkResources(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "links")
	require.NoError(t, err)

	ft := mediatype.OfString("application/vnd.ms-opentype")

	assert.Equal(t, manifest.LinkList{
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/fonts/MinionPro.otf", false),
			MediaType: ft,
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/nav.xhtml", false),
			MediaType: &mediatype.XHTML,
			Rels:      manifest.Strings{"contents"},
		},
		{
			Href:      manifest.MustNewHREFFromString("style.css", false),
			MediaType: &mediatype.CSS,
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/chapter02.xhtml", false),
			MediaType: &mediatype.XHTML,
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/chapter01.smil", false),
			MediaType: &mediatype.SMIL,
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/chapter02.smil", false),
			MediaType: &mediatype.SMIL,
			Duration:  1949.0,
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/images/alice01a.png", false),
			MediaType: &mediatype.PNG,
			Rels:      manifest.Strings{"cover"},
		},
		{
			Href:      manifest.MustNewHREFFromString("OEBPS/images/alice02a.gif", false),
			MediaType: &mediatype.GIF,
		},
		{
			Href: manifest.MustNewHREFFromString("OEBPS/nomediatype.txt", false),
		},
	}, p.Resources)
}

/*func TestPackageDocLinkFallbacksMappedToAlternates(t *testing.T) {
	p, err := loadPackageDoc(t.Context(), "fallbacks")
	assert.NoError(t, err)

	assert.Equal(t, manifest.LinkList{}, p.Resources)

}*/

func TestPackageDocLinkFallbacksCircularDependencies(t *testing.T) {
	_, err := loadPackageDoc(t.Context(), "fallbacks-termination")
	assert.NoError(t, err)
	// t.Logf("%+v\n", p)
}
