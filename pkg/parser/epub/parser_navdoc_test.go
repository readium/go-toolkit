package epub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

func loadNavDoc(name string) (map[string]manifest.LinkList, error) {
	n, rerr := fetcher.NewFileResource(manifest.Link{}, "./testdata/navdoc/"+name+".xhtml").ReadAsXML(map[string]string{
		NamespaceXHTML: "html",
		NamespaceOPS:   "epub",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseNavDoc(n, url.MustURLFromString("OEBPS/xhtml/nav.xhtml")), nil
}

func TestNavDocParserNondirectDescendantOfBody(t *testing.T) {
	n, err := loadNavDoc("nav-section")
	assert.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{
			Title: "Chapter 1",
			Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false),
		},
	}, n["toc"])
}

func TestNavDocParserNewlinesTrimmedFromTitle(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with new lines splitting the text",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false),
	})
}

func TestNavDocParserSpacesTrimmedFromTitle(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with ignorable spaces",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter2.xhtml", false),
	})
}

func TestNavDocParserNestestHTMLElementsAllowedInTitle(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with nested HTML elements",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter3.xhtml", false),
	})
}

func TestNavDocParserEntryWithoutTitleOrChildrenIgnored(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter4.xhtml", false),
	})
}

func TestNavDocParserEntryWithoutLinkOrChildrenIgnored(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "An unlinked element without children must be ignored",
		Href:  manifest.MustNewHREFFromString("#", false),
	})
}

func TestNavDocParserHierarchicalItemsNotAllowed(t *testing.T) {
	n, err := loadNavDoc("nav-children")
	assert.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "Introduction", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/introduction.xhtml", false)},
		{
			Title: "Part I",
			Href:  manifest.MustNewHREFFromString("#", false),
			Children: manifest.LinkList{
				{Title: "Chapter 1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/part1/chapter1.xhtml", false)},
				{Title: "Chapter 2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/part1/chapter2.xhtml", false)},
			},
		},
		{
			Title: "Part II",
			Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/part2/chapter1.xhtml", false),
			Children: manifest.LinkList{
				{Title: "Chapter 1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/part2/chapter1.xhtml", false)},
				{Title: "Chapter 2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/part2/chapter2.xhtml", false)},
			},
		},
	}, n["toc"])
}

func TestNavDocParserEmptyDocAccepted(t *testing.T) {
	n, err := loadNavDoc("nav-empty")
	assert.NoError(t, err)
	assert.Empty(t, n["toc"])
}

func TestNavDocParserTOC(t *testing.T) {
	n, err := loadNavDoc("nav-complex")
	assert.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "Chapter 1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false)},
		{Title: "Chapter 2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter2.xhtml", false)},
	}, n["toc"])
}

func TestNavDocParserPageList(t *testing.T) {
	n, err := loadNavDoc("nav-complex")
	assert.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml#page1", false)},
		{Title: "2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml#page2", false)},
	}, n["page-list"])
}
