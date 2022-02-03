package epub

import (
	"os"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func loadNavDoc(name string) (map[string][]manifest.Link, error) {
	r, err := os.Open("./testdata/navdoc/" + name + ".xhtml")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	n, err := xmlquery.Parse(r)
	if err != nil {
		return nil, err
	}

	return ParseNavDoc(n, "/OEBPS/xhtml/nav.xhtml"), nil
}

func TestNavDocParserNondirectDescendantOfBody(t *testing.T) {
	n, err := loadNavDoc("nav-section")
	assert.NoError(t, err)
	assert.Equal(t, []manifest.Link{
		{
			Title: "Chapter 1",
			Href:  "/OEBPS/xhtml/chapter1.xhtml",
		},
	}, n["toc"])
}

func TestNavDocParserNewlinesTrimmedFromTitle(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with new lines splitting the text",
		Href:  "/OEBPS/xhtml/chapter1.xhtml",
	})
}

func TestNavDocParserSpacesTrimmedFromTitle(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with ignorable spaces",
		Href:  "/OEBPS/xhtml/chapter2.xhtml",
	})
}

func TestNavDocParserNestestHTMLElementsAllowedInTitle(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with nested HTML elements",
		Href:  "/OEBPS/xhtml/chapter3.xhtml",
	})
}

func TestNavDocParserEntryWithoutTitleOrChildrenIgnored(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "",
		Href:  "/OEBPS/xhtml/chapter4.xhtml",
	})
}

func TestNavDocParserEntryWithoutLinkOrChildrenIgnored(t *testing.T) {
	n, err := loadNavDoc("nav-titles")
	assert.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "An unlinked element without children must be ignored",
		Href:  "#",
	})
}

func TestNavDocParserHierarchicalItemsNotAllowed(t *testing.T) {
	n, err := loadNavDoc("nav-children")
	assert.NoError(t, err)
	assert.Equal(t, []manifest.Link{
		{Title: "Introduction", Href: "/OEBPS/xhtml/introduction.xhtml"},
		{
			Title: "Part I",
			Href:  "#",
			Children: manifest.LinkList{
				{Title: "Chapter 1", Href: "/OEBPS/xhtml/part1/chapter1.xhtml"},
				{Title: "Chapter 2", Href: "/OEBPS/xhtml/part1/chapter2.xhtml"},
			},
		},
		{
			Title: "Part II",
			Href:  "/OEBPS/xhtml/part2/chapter1.xhtml",
			Children: manifest.LinkList{
				{Title: "Chapter 1", Href: "/OEBPS/xhtml/part2/chapter1.xhtml"},
				{Title: "Chapter 2", Href: "/OEBPS/xhtml/part2/chapter2.xhtml"},
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
	assert.Equal(t, []manifest.Link{
		{Title: "Chapter 1", Href: "/OEBPS/xhtml/chapter1.xhtml"},
		{Title: "Chapter 2", Href: "/OEBPS/xhtml/chapter2.xhtml"},
	}, n["toc"])
}

func TestNavDocParserPageList(t *testing.T) {
	n, err := loadNavDoc("nav-complex")
	assert.NoError(t, err)
	assert.Equal(t, []manifest.Link{
		{Title: "1", Href: "/OEBPS/xhtml/chapter1.xhtml#page1"},
		{Title: "2", Href: "/OEBPS/xhtml/chapter1.xhtml#page2"},
	}, n["page-list"])
}
