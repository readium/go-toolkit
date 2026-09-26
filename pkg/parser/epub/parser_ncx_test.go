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

func loadNcx(ctx context.Context, name string) (map[string]manifest.LinkList, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/ncx/"+name+".ncx"))
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseNCX(n, url.MustURLFromString("OEBPS/ncx.ncx")), nil
}

func TestNCXParserNewlinesTrimmedFromTitle(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-titles")
	require.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with new lines splitting the text",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false),
	})
}

func TestNCXParserSpacesTrimmedFromTitle(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-titles")
	require.NoError(t, err)
	assert.Contains(t, n["toc"], manifest.Link{
		Title: "A link with ignorable spaces",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter2.xhtml", false),
	})
}

func TestNCXParserEntryWithNoTitleOrChildrenIgnored(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-titles")
	require.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "",
		Href:  manifest.MustNewHREFFromString("OEBPS/xhtml/chapter3.xhtml", false),
	})
}

func TestNCXParserUnlinkedEntriesWithoutChildrenIgnored(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-titles")
	require.NoError(t, err)
	assert.NotContains(t, n["toc"], manifest.Link{
		Title: "An unlinked element without children must be ignored",
		Href:  manifest.MustNewHREFFromString("#", false),
	})
}

func TestNCXParserHierarchicalItemsAllowed(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-children")
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

func TestNCXParserEmptyNCX(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-empty")
	require.NoError(t, err)
	assert.Nil(t, n["toc"])
}

func TestNCXParserTOC(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-complex")
	require.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "Chapter 1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml", false)},
		{Title: "Chapter 2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter2.xhtml", false)},
	}, n["toc"])
}

func TestNCXParserPageList(t *testing.T) {
	n, err := loadNcx(t.Context(), "ncx-complex")
	require.NoError(t, err)
	assert.Equal(t, manifest.LinkList{
		{Title: "1", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml#page1", false)},
		{Title: "2", Href: manifest.MustNewHREFFromString("OEBPS/xhtml/chapter1.xhtml#page2", false)},
	}, n["page-list"])
}

func TestNCXParserDirectChildrenAndNamespaces(t *testing.T) {
	const source = `<n:ncx xmlns:n="http://www.daisy.org/z3986/2005/ncx/" xmlns:f="urn:foreign">
  <n:wrapper><n:navMap>
    <f:navPoint><n:navLabel><n:text>Foreign point</n:text></n:navLabel><n:content src="foreign.xhtml"/></f:navPoint>
    <n:wrapper><n:navPoint><n:navLabel><n:text>Nested point</n:text></n:navLabel><n:content src="nested.xhtml"/></n:navPoint></n:wrapper>
    <n:navPoint>
      <f:navLabel><n:text>Foreign label</n:text></f:navLabel>
      <n:navLabel><f:text>Foreign text</f:text></n:navLabel>
      <n:navLabel><n:wrapper><n:text>Nested text</n:text></n:wrapper></n:navLabel>
      <n:navLabel><n:text>First matching title</n:text><n:text>Second text</n:text></n:navLabel>
      <n:navLabel><n:text>Later title</n:text></n:navLabel>
      <f:content src="foreign-content.xhtml"/>
      <n:wrapper><n:content src="nested-content.xhtml"/></n:wrapper>
      <n:content src="chapter.xhtml"/><n:content src="second-content.xhtml"/>
      <n:navPoint><n:navLabel><n:text>Child</n:text></n:navLabel><n:content src="child.xhtml"/></n:navPoint>
    </n:navPoint>
    <n:navPoint><n:navLabel><n:text/></n:navLabel><n:navLabel><n:text>Ignored after empty text</n:text></n:navLabel><n:content src="empty-title.xhtml"/></n:navPoint>
    <n:navPoint><n:navLabel><n:text>Ignored after empty src</n:text></n:navLabel><n:content/><n:content src="second-src.xhtml"/></n:navPoint>
  </n:navMap></n:wrapper>
  <n:pageList>
    <f:pageTarget><n:navLabel><n:text>Foreign page</n:text></n:navLabel><n:content src="foreign-page.xhtml"/></f:pageTarget>
    <n:wrapper><n:pageTarget><n:navLabel><n:text>Nested page</n:text></n:navLabel><n:content src="nested-page.xhtml"/></n:pageTarget></n:wrapper>
    <n:pageTarget><n:navLabel/><n:navLabel><n:text>1</n:text></n:navLabel><n:content src="chapter.xhtml#page1"/></n:pageTarget>
  </n:pageList>
</n:ncx>`
	for _, prefix := range []string{"prefixed", "default"} {
		t.Run(prefix, func(t *testing.T) {
			input := source
			if prefix == "default" {
				input = strings.NewReplacer("xmlns:n=", "xmlns=", "<n:", "<", "</n:", "</").Replace(input)
			}
			document, err := xmlquery.Parse(strings.NewReader(input))
			require.NoError(t, err)
			navigation := ParseNCX(document, url.MustURLFromString("OPS/toc.ncx"))
			assert.Equal(t, manifest.LinkList{{
				Title: "First matching title", Href: manifest.MustNewHREFFromString("OPS/chapter.xhtml", false),
				Children: manifest.LinkList{{Title: "Child", Href: manifest.MustNewHREFFromString("OPS/child.xhtml", false)}},
			}}, navigation["toc"])
			assert.Equal(t, manifest.LinkList{{Title: "1", Href: manifest.MustNewHREFFromString("OPS/chapter.xhtml#page1", false)}}, navigation["page-list"])
		})
	}
}
