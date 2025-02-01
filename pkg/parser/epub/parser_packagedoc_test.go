package epub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

func loadPackageDoc(name string) (*manifest.Manifest, error) {
	n, rerr := fetcher.NewFileResource(manifest.Link{}, "./testdata/package/"+name+".opf").ReadAsXML(map[string]string{
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
	p, err := loadPackageDoc("progression-none")
	assert.NoError(t, err)
	assert.Equal(t, manifest.Auto, p.Metadata.ReadingProgression)
}

func TestPackageDocPageProgression(t *testing.T) {
	p, err := loadPackageDoc("progression-default")
	assert.NoError(t, err)
	assert.Equal(t, manifest.Auto, p.Metadata.ReadingProgression)
}

func TestPackageDocPageProgressionLTR(t *testing.T) {
	p, err := loadPackageDoc("progression-ltr")
	assert.NoError(t, err)
	assert.Equal(t, manifest.LTR, p.Metadata.ReadingProgression)
}

func TestPackageDocPageProgressionRTL(t *testing.T) {
	p, err := loadPackageDoc("progression-rtl")
	assert.NoError(t, err)
	assert.Equal(t, manifest.RTL, p.Metadata.ReadingProgression)
}

func TestPackageDocLinkPropertiesContains(t *testing.T) {
	p, err := loadPackageDoc("links-properties")
	assert.NoError(t, err)
	ro := p.ReadingOrder
	assert.Equal(t, []string{"mathml"}, ro[0].Properties.Contains())
	assert.Equal(t, []string{"remote-resources"}, ro[1].Properties.Contains())
	assert.Equal(t, []string{"js", "svg"}, ro[2].Properties.Contains())
	assert.Empty(t, ro[3].Properties.Contains())
	assert.Empty(t, ro[4].Properties.Contains())
}

func TestPackageDocLinkPropertiesRels(t *testing.T) {
	p, err := loadPackageDoc("links-properties")
	assert.NoError(t, err)
	ro := p.ReadingOrder
	assert.Equal(t, manifest.Strings{"cover"}, p.Resources[0].Rels)
	assert.Empty(t, ro[0].Rels)
	assert.Empty(t, ro[1].Rels)
	assert.Empty(t, ro[2].Rels)
	assert.Equal(t, manifest.Strings{"contents"}, ro[3].Rels)
	assert.Empty(t, ro[4].Rels)
}

func TestPackageDocLinkPropertiesPresentation(t *testing.T) {
	p, err := loadPackageDoc("links-properties")
	assert.NoError(t, err)
	ro := p.ReadingOrder
	assert.Equal(t, ro[0].Properties.Layout(), manifest.EPUBLayoutFixed)
	assert.Equal(t, ro[0].Properties.Overflow(), manifest.OverflowAuto)
	assert.Equal(t, ro[0].Properties.Orientation(), manifest.OrientationAuto)
	assert.Equal(t, ro[0].Properties.Page(), manifest.PageRight)
	assert.Equal(t, ro[0].Properties.Spread(), manifest.Spread(""))

	assert.Equal(t, ro[1].Properties.Layout(), manifest.EPUBLayoutReflowable)
	assert.Equal(t, ro[1].Properties.Overflow(), manifest.OverflowPaginated)
	assert.Equal(t, ro[1].Properties.Orientation(), manifest.OrientationLandscape)
	assert.Equal(t, ro[1].Properties.Page(), manifest.PageLeft)
	assert.Equal(t, ro[1].Properties.Spread(), manifest.Spread(""))

	assert.Equal(t, ro[2].Properties.Layout(), manifest.EPUBLayout(""))
	assert.Equal(t, ro[2].Properties.Overflow(), manifest.OverflowScrolled)
	assert.Equal(t, ro[2].Properties.Orientation(), manifest.OrientationPortrait)
	assert.Equal(t, ro[2].Properties.Page(), manifest.PageCenter)
	assert.Equal(t, ro[2].Properties.Spread(), manifest.Spread(""))

	assert.Equal(t, ro[3].Properties.Layout(), manifest.EPUBLayout(""))
	assert.Equal(t, ro[3].Properties.Overflow(), manifest.OverflowScrolled)
	assert.Equal(t, ro[3].Properties.Orientation(), manifest.Orientation(""))
	assert.Equal(t, ro[3].Properties.Page(), manifest.Page(""))
	assert.Equal(t, ro[3].Properties.Spread(), manifest.SpreadAuto)
}

func TestPackageDocLinkReadingOrder(t *testing.T) {
	p, err := loadPackageDoc("links")
	assert.NoError(t, err)

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
	p, err := loadPackageDoc("links")
	assert.NoError(t, err)

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
	p, err := loadPackageDoc("fallbacks")
	assert.NoError(t, err)

	assert.Equal(t, manifest.LinkList{}, p.Resources)

}*/

func TestPackageDocLinkFallbacksCircularDependencies(t *testing.T) {
	_, err := loadPackageDoc("fallbacks-termination")
	assert.NoError(t, err)
	// t.Logf("%+v\n", p)
}
