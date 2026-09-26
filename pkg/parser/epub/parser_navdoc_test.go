package epub

import (
	"context"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadNavDoc(ctx context.Context, name string) (map[string]manifest.LinkList, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/navdoc/"+name+".xhtml"))
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseNavDoc(n, url.MustURLFromString("OEBPS/xhtml/nav.xhtml")), nil
}

func TestNavDocParserNondirectDescendantOfBody(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-section")
	require.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{
			Title: "Chapter 1",
			Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false),
		},
	}, n["toc"])
}

func TestNavDocParserNewlinesTrimmedFromTitle(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-titles")
	require.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with new lines splitting the text",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false),
	})
}

func TestNavDocParserSpacesTrimmedFromTitle(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-titles")
	require.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with ignorable spaces",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter2.xhtml", false),
	})
}

func TestNavDocParserNestestHTMLElementsAllowedInTitle(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-titles")
	require.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with nested HTML elements",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter3.xhtml", false),
	})
}

func TestNavDocParserEntryWithoutTitleOrChildrenIgnored(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-titles")
	require.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter4.xhtml", false),
	})
}

func TestNavDocParserEntryWithoutLinkOrChildrenIgnored(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-titles")
	require.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "An unlinked element without children must be ignored",
		Href:  manifest.MustNewHREFFromString("#", false),
	})
}

func TestNavDocParserHierarchicalItemsNotAllowed(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-children")
	require.NoError(t, err)
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
	n, err := loadNavDoc(t.Context(), "nav-empty")
	require.NoError(t, err)
	assert.Empty(t, n["toc"])
}

func TestNavDocParserTOC(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-complex")
	require.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "Chapter 1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false)},
		{Title: "Chapter 2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter2.xhtml", false)},
	}, n["toc"])
}

func TestNavDocParserPageList(t *testing.T) {
	n, err := loadNavDoc(t.Context(), "nav-complex")
	require.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml#page1", false)},
		{Title: "2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml#page2", false)},
	}, n["page-list"])
}

func TestNavDocParserDirectChildrenAndNamespaces(t *testing.T) {
	const source = `<h:html xmlns:h="http://www.w3.org/1999/xhtml" xmlns:e="http://www.idpf.org/2007/ops" xmlns:f="urn:foreign">
  <h:body><h:section><h:nav e:type="toc">
    <f:ol><h:li><h:a href="foreign-list.xhtml">Ignored foreign list</h:a></h:li></f:ol>
    <h:div><h:ol><h:li><h:a href="nested-list.xhtml">Ignored nested list</h:a></h:li></h:ol></h:div>
    <h:ol>
      <f:li><h:a href="foreign-item.xhtml">Ignored foreign item</h:a></f:li>
      <h:div><h:li><h:a href="nested-item.xhtml">Ignored nested item</h:a></h:li></h:div>
      <h:li>Leading text<!-- comment --><f:a href="foreign-anchor.xhtml">Foreign <h:em>anchor</h:em></f:a></h:li>
      <h:li><h:span>Part</h:span>
        <f:ol><h:li><h:a href="foreign-child-list.xhtml">Ignored foreign child list</h:a></h:li></f:ol>
        <h:div><h:ol><h:li><h:a href="nested-child-list.xhtml">Ignored nested child list</h:a></h:li></h:ol></h:div>
        <h:ol><h:li><h:a href="child.xhtml">Child</h:a></h:li></h:ol>
      </h:li>
      <h:li><h:a href="empty-children.xhtml">Empty children</h:a><h:ol/></h:li>
      <h:li><h:span>First child has no link</h:span><h:a href="second-anchor.xhtml">Ignored second anchor</h:a></h:li>
      <h:li><?ignore instruction?><h:a href="after-pi.xhtml">Ignored after processing instruction</h:a></h:li>
      <h:li>Text before element<h:a href="last.xhtml">Last</h:a></h:li>
    </h:ol>
    <h:ol><h:li><h:a href="second-list.xhtml">Ignored second direct list</h:a></h:li></h:ol>
  </h:nav></h:section></h:body>
</h:html>`
	for _, prefix := range []string{"prefixed", "default"} {
		t.Run(prefix, func(t *testing.T) {
			input := source
			if prefix == "default" {
				input = strings.NewReplacer("xmlns:h=", "xmlns=", "<h:", "<", "</h:", "</").Replace(input)
			}
			document, err := xmlquery.Parse(strings.NewReader(input))
			require.NoError(t, err)
			navigation := ParseNavDoc(document, url.MustURLFromString("OPS/nav.xhtml"))
			assert.Equal(t, manifest.LinkList{
				{Title: "Foreign anchor", Href: manifest.MustNewHREFFromString("OPS/foreign-anchor.xhtml", false)},
				{Title: "Part", Href: manifest.MustNewHREFFromString("#", false), Children: manifest.LinkList{
					{Title: "Child", Href: manifest.MustNewHREFFromString("OPS/child.xhtml", false)},
				}},
				{Title: "Empty children", Href: manifest.MustNewHREFFromString("OPS/empty-children.xhtml", false), Children: manifest.LinkList{}},
				{Title: "Last", Href: manifest.MustNewHREFFromString("OPS/last.xhtml", false)},
			}, navigation["toc"])
		})
	}
}
