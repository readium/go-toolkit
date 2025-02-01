package manifest

import (
	"encoding/json"
	"testing"

	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

/*func TestLinkTemplateParameters(t *testing.T) {
	assert.Equal(
		t,
		[]string{"x", "hello", "y", "z", "w"},
		Link{Href: "url{?x,hello,y}name{z,y,w}", Templated: true}.TemplateParameters(),
	)
}

func TestLinkTemplateExpand(t *testing.T) {
	assert.Equal(
		t,
		Link{
			Href:      "url?x=aaa&hello=Hello,%20world&y=bname",
			Templated: false,
		},
		Link{
			Href:      "url{?x,hello,y}name",
			Templated: true,
		}.ExpandTemplate(map[string]string{
			"x":     "aaa",
			"hello": "Hello, world",
			"y":     "b",
		}),
	)
}*/

func TestLinkUnmarshalMinimalJSON(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "http://href"}`), &l))
	u, _ := url.URLFromString("http://href")
	assert.Equal(t, Link{Href: NewHREF(u)}, l, "parsed JSON object should be equal to Link object")
}

func TestLinkUnmarshalFullJSON(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{
		"href": "http://href",
		"type": "application/pdf",
		"templated": true,
		"title": "Link Title",
		"rel": ["publication", "cover"],
		"properties": {
			"orientation": "landscape"
		},
		"height": 1024,
		"width": 768,
		"bitrate": 74.2,
		"duration": 45.6,
		"language": "fr",
		"alternate": [
			{"href": "alternate1"},
			{"href": "alternate2"}
		],
		"children": [
			{"href": "http://child1"},
			{"href": "http://child2"}
		]
	}`), &l))
	h, _ := NewHREFFromString("http://href", true)
	assert.Equal(t, Link{
		Href:      h,
		MediaType: &mediatype.PDF,
		Title:     "Link Title",
		Rels:      []string{"publication", "cover"},
		Properties: Properties{
			"orientation": "landscape",
		},
		Height:    1024,
		Width:     768,
		Bitrate:   74.2,
		Duration:  45.6,
		Languages: []string{"fr"},
		Alternates: []Link{
			{Href: MustNewHREFFromString("alternate1", false)},
			{Href: MustNewHREFFromString("alternate2", false)},
		},
		Children: []Link{
			{Href: MustNewHREFFromString("http://child1", false)},
			{Href: MustNewHREFFromString("http://child2", false)},
		},
	}, l, "parsed JSON object should be equal to Link object")
}

func TestLinkUnmarshalNilJSON(t *testing.T) {
	s, err := LinkFromJSON(nil)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestLinkUnmarshalJSONRelString(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "rel": "publication"}`), &l))
	assert.Equal(t, Link{Href: MustNewHREFFromString("a", false), Rels: []string{"publication"}}, l)
}

func TestLinkUnmarshalJSONTemplatedDefaultFalse(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a"}`), &l))
	assert.False(t, l.Href.IsTemplated())
}

func TestLinkUnmarshalJSONTemplatedNilFalse(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "templated": null}`), &l))
	assert.False(t, l.Href.IsTemplated())
}

func TestLinkUnmarshalJSONMultipleLanguages(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "language": ["fr", "en"]}`), &l))
	assert.Equal(t, Link{Href: MustNewHREFFromString("a", false), Languages: []string{"fr", "en"}}, l)
}

func TestLinkUnmarshalJSONRequiresHref(t *testing.T) {
	var l Link
	assert.Error(t, json.Unmarshal([]byte(`{"type": "application/pdf"}`), &l))
}

func TestLinkUnmarshalJSONRequiresPositiveWidth(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "width": -20}`), &l))
	assert.Equal(t, l.Width, uint(0))
}

func TestLinkUnmarshalJSONRequiresPositiveHeight(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "height": -20}`), &l))
	assert.Equal(t, l.Height, uint(0))
}

func TestLinkUnmarshalJSONRequiresPositiveBitrate(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "bitrate": -20}`), &l))
	assert.Equal(t, l.Bitrate, float64(0))
}

func TestLinkUnmarshalJSONRequiresPositiveDuration(t *testing.T) {
	var l Link
	assert.NoError(t, json.Unmarshal([]byte(`{"href": "a", "duration": -20}`), &l))
	assert.Equal(t, l.Duration, float64(0))
}

func TestLinkUnmarshalJSONArray(t *testing.T) {
	var ll []Link
	assert.NoError(t, json.Unmarshal([]byte(`[
		{"href": "http://child1"},
		{"href": "http://child2"}
	]`), &ll))
	assert.Equal(t, []Link{
		{Href: MustNewHREFFromString("http://child1", false)},
		{Href: MustNewHREFFromString("http://child2", false)},
	}, ll, "parsed JSON array should be equal to Link slice")
}

func TestLinkUnmarshalJSONNilArray(t *testing.T) {
	ll, err := LinksFromJSONArray(nil)
	assert.NoError(t, err)
	assert.Equal(t, []Link{}, ll)
}

func TestLinkUnmarshalJSONArrayRefusesInvalidLinks(t *testing.T) {
	var ll []Link
	assert.Error(t, json.Unmarshal([]byte(`[
		{"title": "Title"},
		{"href": "http://child2"}
	]`), &ll))
}

func TestLinkMinimalJSON(t *testing.T) {
	b, err := json.Marshal(Link{Href: MustNewHREFFromString("http://href", false)})
	assert.NoError(t, err)
	assert.JSONEq(t, `{"href": "http://href"}`, string(b))
}

func TestLinkFullJSON(t *testing.T) {
	b, err := json.Marshal(Link{
		Href:      MustNewHREFFromString("http://href", true),
		MediaType: &mediatype.PDF,
		Title:     "Link Title",
		Rels:      []string{"publication", "cover"},
		Properties: Properties{
			"orientation": "landscape",
		},
		Height:    1024,
		Width:     768,
		Bitrate:   74.2,
		Duration:  45.6,
		Languages: []string{"fr"},
		Alternates: []Link{
			{Href: MustNewHREFFromString("alternate1", false)},
			{Href: MustNewHREFFromString("alternate2", false)},
		},
		Children: []Link{
			{Href: MustNewHREFFromString("http://child1", false)},
			{Href: MustNewHREFFromString("http://child2", false)},
		},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"href": "http://href",
		"type": "application/pdf",
		"templated": true,
		"title": "Link Title",
		"rel": ["publication", "cover"],
		"properties": {
			"orientation": "landscape"
		},
		"height": 1024,
		"width": 768,
		"bitrate": 74.2,
		"duration": 45.6,
		"language": "fr",
		"alternate": [
			{"href": "alternate1"},
			{"href": "alternate2"}
		],
		"children": [
			{"href": "http://child1"},
			{"href": "http://child2"}
		]
	}`, string(b))
}

func TestLinkJSONArray(t *testing.T) {
	b, err := json.Marshal([]Link{
		{Href: MustNewHREFFromString("http://child1", false)},
		{Href: MustNewHREFFromString("http://child2", false)},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `[
		{"href": "http://child1"},
		{"href": "http://child2"}
	]`, string(b))
}

/*func TestLinkUnknownMediaType(t *testing.T) {
	assert.Equal(t, &mediatype.Binary, Link{Href: MustNewHREFFromString("file", false)}.MediaType)
}*/

/*func TestLinkMediaTypeFromType(t *testing.T) {
	assert.Equal(t, mediatype.EPUB, Link{Href: "file", Type: "application/epub+zip"}.MediaType())
	assert.Equal(t, mediatype.PDF, Link{Href: "file", Type: "application/pdf"}.MediaType())
}*/

func TestLinkToURLRelativeToBase(t *testing.T) {
	assert.Equal(t, "http://host/folder/file.html", Link{Href: MustNewHREFFromString("folder/file.html", false)}.URL(url.MustURLFromString("http://host/"), nil).String())
}

func TestLinkToURLRelativeToBaseWithRootPrefix(t *testing.T) {
	assert.Equal(t, "http://host/folder/file.html", Link{Href: MustNewHREFFromString("folder/file.html", false)}.URL(url.MustURLFromString("http://host/"), nil).String())
}

func TestLinkToURLRelativeToNothing(t *testing.T) {
	assert.Equal(t, "folder/file.html", Link{Href: MustNewHREFFromString("folder/file.html", false)}.URL(nil, nil).String())
}

/*func TestLinkToURLWithInvalidHref(t *testing.T) {
	assert.Empty(t, Link{Href: MustNewHREFFromString("", false)}.URL(url.MustURLFromString("http://test.com"), nil).String())
}*/

func TestLinkToURLWithAbsoluteHref(t *testing.T) {
	assert.Equal(t, "http://test.com/folder/file.html", Link{Href: MustNewHREFFromString("http://test.com/folder/file.html", false)}.URL(url.MustURLFromString("http://host/"), nil).String())
}

func TestLinkToURLWithHrefContainingInvalidChars(t *testing.T) {
	// Original expected: "http://host/folder/Cory%20Doctorow's/a-fc.jpg". TODO: is it not good that the ' got escaped?
	assert.Equal(t, "http://host/folder/Cory%20Doctorow%27s/a-fc.jpg", Link{Href: MustNewHREFFromString("Cory Doctorow's/a-fc.jpg", false)}.URL(url.MustURLFromString("http://host/folder/"), nil).String())
}

func TestLinkFirstIndexLinkWithHrefInList(t *testing.T) {
	assert.Equal(t, -1, LinkList{Link{Href: MustNewHREFFromString("href", false)}}.IndexOfFirstWithHref(url.MustURLFromString("foobar")))

	assert.Equal(
		t,
		1,
		LinkList{
			Link{Href: MustNewHREFFromString("href1", false)},
			Link{Href: MustNewHREFFromString("href2", false)},
			Link{Href: MustNewHREFFromString("href2", false)}, // duplicated on purpose
		}.IndexOfFirstWithHref(url.MustURLFromString("href2")),
	)
}
