package manifest

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

var base, _ = url.URLFromString("http://readium/publication/")

func TestConvertStaticHREFToURL(t *testing.T) {
	u, _ := url.URLFromString("folder/chapter.xhtml")
	assert.Equal(t, u, NewHREF(u).Resolve(nil, nil))
	u2, _ := url.URLFromString("http://readium/publication/folder/chapter.xhtml")
	assert.Equal(t, u2, NewHREF(u).Resolve(base, nil))

	// Parameters are ignored
	assert.Equal(t, u, NewHREF(u).Resolve(nil, map[string]string{"a": "b"}))
}

func TestConvertTemplatedHREFToURL(t *testing.T) {
	template, _ := NewHREFFromString("url{?x,hello,y}name", true)

	parameters := map[string]string{
		"x":     "aaa",
		"hello": "Hello, world",
		"y":     "b",
		"foo":   "bar",
	}

	u, _ := url.URLFromString("urlname")
	assert.Equal(t, u, template.Resolve(nil, nil))

	u, _ = url.URLFromString("http://readium/publication/urlname")
	assert.Equal(t, u, template.Resolve(base, nil))

	u, _ = url.URLFromString("http://readium/publication/url?x=aaa&hello=Hello%2C%20world&y=bname")
	assert.Equal(t, u, template.Resolve(base, parameters))
}

func TestHREFIsTemplated(t *testing.T) {
	h, _ := NewHREFFromString("folder/chapter.xhtml", false)
	assert.False(t, h.IsTemplated())

	h, _ = NewHREFFromString("url", true)
	assert.True(t, h.IsTemplated())

	h, _ = NewHREFFromString("url{?x,hello,y}name", true)
	assert.True(t, h.IsTemplated())
}

func TestHREFParameters(t *testing.T) {
	h, _ := NewHREFFromString("url", false)
	assert.Equal(t, []string{}, h.Parameters())

	h, _ = NewHREFFromString("url", true)
	assert.Equal(t, []string{}, h.Parameters())

	h, _ = NewHREFFromString("url{?x,hello,y}name", true)
	assert.Equal(t, []string{"x", "hello", "y"}, h.Parameters())
}

func TestHREFToString(t *testing.T) {
	h, _ := NewHREFFromString("folder/chapter.xhtml", false)
	assert.Equal(t, "folder/chapter.xhtml", h.String())

	h, _ = NewHREFFromString("url{?x,hello,y}name", true)
	assert.Equal(t, "url{?x,hello,y}name", h.String())
}
