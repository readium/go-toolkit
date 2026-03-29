package epub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadNcx(ctx context.Context, name string) (map[string]manifest.LinkList, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/ncx/"+name+".ncx"), map[string]string{
		NamespaceNCX: "ncx",
	})
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
