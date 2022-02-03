package util

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func hrefString(t *testing.T, href string, base string) string {
	h, err := NewHREF(href, base).String()
	assert.NoError(t, err)
	return h
}

func TestHrefNormalizeToBase(t *testing.T) {
	assert.Equal(t, "/folder/", hrefString(t, "", "/folder/"))
	assert.Equal(t, "/", hrefString(t, "/", "/folder/"))

	assert.Equal(t, "/foo/bar.txt", hrefString(t, "foo/bar.txt", ""))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "foo/bar.txt", "/"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "foo/bar.txt", "/file.txt"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "foo/bar.txt", "/folder"))
	assert.Equal(t, "/folder/foo/bar.txt", hrefString(t, "foo/bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/folder/foo/bar.txt", hrefString(t, "foo/bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "foo/bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/folder/foo/bar.txt", hrefString(t, "foo/bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "/foo/bar.txt", hrefString(t, "/foo/bar.txt", ""))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "/foo/bar.txt", "/"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "/foo/bar.txt", "/file.txt"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "/foo/bar.txt", "/folder"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "/foo/bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "/foo/bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "/foo/bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "/foo/bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "/foo/bar.txt", hrefString(t, "../foo/bar.txt", ""))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "../foo/bar.txt", "/"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "../foo/bar.txt", "/file.txt"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "../foo/bar.txt", "/folder"))
	assert.Equal(t, "/foo/bar.txt", hrefString(t, "../foo/bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "../foo/bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "../foo/bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefString(t, "../foo/bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "/bar.txt", hrefString(t, "foo/../bar.txt", ""))
	assert.Equal(t, "/bar.txt", hrefString(t, "foo/../bar.txt", "/"))
	assert.Equal(t, "/bar.txt", hrefString(t, "foo/../bar.txt", "/file.txt"))
	assert.Equal(t, "/bar.txt", hrefString(t, "foo/../bar.txt", "/folder"))
	assert.Equal(t, "/folder/bar.txt", hrefString(t, "foo/../bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/folder/bar.txt", hrefString(t, "foo/../bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/bar.txt", hrefString(t, "foo/../bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/folder/bar.txt", hrefString(t, "foo/../bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "http://absolute.com/foo/bar.txt", hrefString(t, "http://absolute.com/foo/bar.txt", "/"))
	assert.Equal(t, "http://absolute.com/foo/bar.txt", hrefString(t, "http://absolute.com/foo/bar.txt", "https://example.com/"))

	// Anchor and query parameters are preserved
	assert.Equal(t, "/foo/bar.txt#anchor", hrefString(t, "foo/bar.txt#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefString(t, "foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefString(t, "/foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "http://absolute.com/foo/bar.txt?query=param#anchor", hrefString(t, "http://absolute.com/foo/bar.txt?query=param#anchor", "/"))

	assert.Equal(t, "/foo/bar.txt#anchor", hrefString(t, "foo/bar.txt#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefString(t, "foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefString(t, "/foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "http://absolute.com/foo/bar.txt?query=param#anchor", hrefString(t, "http://absolute.com/foo/bar.txt?query=param#anchor", "/"))

	// HREF that is just an anchor
	assert.Equal(t, "/#anchor", hrefString(t, "#anchor", ""))
	assert.Equal(t, "/#anchor", hrefString(t, "#anchor", "/"))
	assert.Equal(t, "/file.txt#anchor", hrefString(t, "#anchor", "/file.txt"))
	assert.Equal(t, "/folder#anchor", hrefString(t, "#anchor", "/folder"))
	assert.Equal(t, "/folder/#anchor", hrefString(t, "#anchor", "/folder/"))
	assert.Equal(t, "http://example.com/folder/file.txt#anchor", hrefString(t, "#anchor", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/folder#anchor", hrefString(t, "#anchor", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/folder/#anchor", hrefString(t, "#anchor", "http://example.com/folder/"))

	// HREF containing spaces.
	assert.Equal(t, "/foo bar.txt", hrefString(t, "foo bar.txt", ""))
	assert.Equal(t, "/foo bar.txt", hrefString(t, "foo bar.txt", "/"))
	assert.Equal(t, "/foo bar.txt", hrefString(t, "foo bar.txt", "/file.txt"))
	assert.Equal(t, "/foo bar.txt", hrefString(t, "foo bar.txt", "/base folder"))
	assert.Equal(t, "/base folder/foo bar.txt", hrefString(t, "foo bar.txt", "/base folder/"))
	assert.Equal(t, "/base folder/foo bar.txt", hrefString(t, "foo bar.txt", "/base folder/file.txt"))
	assert.Equal(t, "/base folder/foo bar.txt", hrefString(t, "foo bar.txt", "base folder/file.txt"))

	// HREF containing special characters
	assert.Equal(t, "/base%folder/foo bar/baz%qux.txt", hrefString(t, "foo bar/baz%qux.txt", "/base%folder/"))
	assert.Equal(t, "/base folder/foo bar/baz%qux.txt", hrefString(t, "foo%20bar/baz%25qux.txt", "/base%20folder/"))
	assert.Equal(t, "http://example.com/foo bar/baz qux.txt", hrefString(t, "foo bar/baz qux.txt", "http://example.com/base%20folder"))
	assert.Equal(t, "http://example.com/base folder/foo bar/baz qux.txt", hrefString(t, "foo bar/baz qux.txt", "http://example.com/base%20folder/"))
	assert.Equal(t, "http://example.com/base folder/foo bar/baz%qux.txt", hrefString(t, "foo bar/baz%qux.txt", "http://example.com/base%20folder/"))
	assert.Equal(t, "/foo bar.txt?query=param#anchor", hrefString(t, "/foo bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "http://example.com/foo bar.txt?query=param#anchor", hrefString(t, "/foo bar.txt?query=param#anchor", "http://example.com/"))
	assert.Equal(t, "http://example.com/foo bar.txt?query=param#anchor", hrefString(t, "/foo%20bar.txt?query=param#anchor", "http://example.com/"))
	assert.Equal(t, "http://absolute.com/foo bar.txt?query=param#Hello world £500", hrefString(t, "http://absolute.com/foo%20bar.txt?query=param#Hello%20world%20%C2%A3500", "/"))
	assert.Equal(t, "http://absolute.com/foo bar.txt?query=param#Hello world £500", hrefString(t, "http://absolute.com/foo bar.txt?query=param#Hello world £500", "/"))
}

func hrefPEString(t *testing.T, href string, base string) string {
	h, err := NewHREF(href, base).PercentEncodedString()
	assert.NoError(t, err)
	return h
}

func TestHrefPercentEncodedString(t *testing.T) {
	assert.Equal(t, "/folder/", hrefPEString(t, "", "/folder/"))
	assert.Equal(t, "/", hrefPEString(t, "/", "/folder/"))

	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "foo/bar.txt", ""))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "/"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "/file.txt"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "/folder"))
	assert.Equal(t, "/folder/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/folder/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/folder/foo/bar.txt", hrefPEString(t, "foo/bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", ""))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "/"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "/file.txt"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "/folder"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "/foo/bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", ""))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "/"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "/file.txt"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "/folder"))
	assert.Equal(t, "/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/foo/bar.txt", hrefPEString(t, "../foo/bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "/bar.txt", hrefPEString(t, "foo/../bar.txt", ""))
	assert.Equal(t, "/bar.txt", hrefPEString(t, "foo/../bar.txt", "/"))
	assert.Equal(t, "/bar.txt", hrefPEString(t, "foo/../bar.txt", "/file.txt"))
	assert.Equal(t, "/bar.txt", hrefPEString(t, "foo/../bar.txt", "/folder"))
	assert.Equal(t, "/folder/bar.txt", hrefPEString(t, "foo/../bar.txt", "/folder/"))
	assert.Equal(t, "http://example.com/folder/bar.txt", hrefPEString(t, "foo/../bar.txt", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/bar.txt", hrefPEString(t, "foo/../bar.txt", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/folder/bar.txt", hrefPEString(t, "foo/../bar.txt", "http://example.com/folder/"))

	assert.Equal(t, "http://absolute.com/foo/bar.txt", hrefPEString(t, "http://absolute.com/foo/bar.txt", "/"))
	assert.Equal(t, "http://absolute.com/foo/bar.txt", hrefPEString(t, "http://absolute.com/foo/bar.txt", "https://example.com/"))

	// Anchor and query parameters are preserved
	assert.Equal(t, "/foo/bar.txt#anchor", hrefPEString(t, "foo/bar.txt#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefPEString(t, "foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefPEString(t, "/foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "http://absolute.com/foo/bar.txt?query=param#anchor", hrefPEString(t, "http://absolute.com/foo/bar.txt?query=param#anchor", "/"))

	assert.Equal(t, "/foo/bar.txt#anchor", hrefPEString(t, "foo/bar.txt#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefPEString(t, "foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "/foo/bar.txt?query=param#anchor", hrefPEString(t, "/foo/bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "http://absolute.com/foo/bar.txt?query=param#anchor", hrefPEString(t, "http://absolute.com/foo/bar.txt?query=param#anchor", "/"))

	// HREF that is just an anchor
	assert.Equal(t, "/#anchor", hrefPEString(t, "#anchor", ""))
	assert.Equal(t, "/#anchor", hrefPEString(t, "#anchor", "/"))
	assert.Equal(t, "/file.txt#anchor", hrefPEString(t, "#anchor", "/file.txt"))
	assert.Equal(t, "/folder#anchor", hrefPEString(t, "#anchor", "/folder"))
	assert.Equal(t, "/folder/#anchor", hrefPEString(t, "#anchor", "/folder/"))
	assert.Equal(t, "http://example.com/folder/file.txt#anchor", hrefPEString(t, "#anchor", "http://example.com/folder/file.txt"))
	assert.Equal(t, "http://example.com/folder#anchor", hrefPEString(t, "#anchor", "http://example.com/folder"))
	assert.Equal(t, "http://example.com/folder/#anchor", hrefPEString(t, "#anchor", "http://example.com/folder/"))

	// HREF containing spaces.
	assert.Equal(t, "/foo%20bar.txt", hrefPEString(t, "foo bar.txt", ""))
	assert.Equal(t, "/foo%20bar.txt", hrefPEString(t, "foo bar.txt", "/"))
	assert.Equal(t, "/foo%20bar.txt", hrefPEString(t, "foo bar.txt", "/file.txt"))
	assert.Equal(t, "/foo%20bar.txt", hrefPEString(t, "foo bar.txt", "/base folder"))
	assert.Equal(t, "/base%20folder/foo%20bar.txt", hrefPEString(t, "foo bar.txt", "/base folder/"))
	assert.Equal(t, "/base%20folder/foo%20bar.txt", hrefPEString(t, "foo bar.txt", "/base folder/file.txt"))
	assert.Equal(t, "/base%20folder/foo%20bar.txt", hrefPEString(t, "foo bar.txt", "base folder/file.txt"))

	// HREF containing special characters
	assert.Equal(t, "/base%25folder/foo%20bar/baz%25qux.txt", hrefPEString(t, "foo bar/baz%qux.txt", "/base%folder/"))
	assert.Equal(t, "/base%20folder/foo%20bar/baz%25qux.txt", hrefPEString(t, "foo%20bar/baz%25qux.txt", "/base%20folder/"))
	assert.Equal(t, "http://example.com/foo%20bar/baz%20qux.txt", hrefPEString(t, "foo bar/baz qux.txt", "http://example.com/base%20folder"))
	assert.Equal(t, "http://example.com/base%20folder/foo%20bar/baz%20qux.txt", hrefPEString(t, "foo bar/baz qux.txt", "http://example.com/base%20folder/"))
	assert.Equal(t, "http://example.com/base%20folder/foo%20bar/baz%25qux.txt", hrefPEString(t, "foo bar/baz%qux.txt", "http://example.com/base%20folder/"))
	assert.Equal(t, "/foo%20bar.txt?query=param#anchor", hrefPEString(t, "/foo bar.txt?query=param#anchor", "/"))
	assert.Equal(t, "http://example.com/foo%20bar.txt?query=param#anchor", hrefPEString(t, "/foo bar.txt?query=param#anchor", "http://example.com/"))
	assert.Equal(t, "http://example.com/foo%20bar.txt?query=param#anchor", hrefPEString(t, "/foo%20bar.txt?query=param#anchor", "http://example.com/"))
	assert.Equal(t, "http://absolute.com/foo%20bar.txt?query=param#Hello%20world%20%C2%A3500", hrefPEString(t, "http://absolute.com/foo%20bar.txt?query=param#Hello%20world%20%C2%A3500", "/"))

	assert.Equal(t, "http://absolute.com/foo%20bar.txt?query=param#Hello%20world%20%C2%A3500", hrefPEString(t, "http://absolute.com/foo bar.txt?query=param#Hello world £500", "/"))

}

func hrefQueryParams(t *testing.T, href string) url.Values {
	h, err := NewHREF(href, "").QueryParameters()
	assert.NoError(t, err)
	return h
}

func TestHrefQueryParameters(t *testing.T) {
	assert.Equal(t, make(url.Values), hrefQueryParams(t, "http://domain.com/path"))
	assert.Equal(t, url.Values{
		"query": []string{"param"},
	}, hrefQueryParams(t, "http://domain.com/path?query=param#anchor"))
	assert.Equal(t, url.Values{
		"query": []string{"param", "other"},
		"fruit": []string{"banana"},
		"empty": []string{""},
	}, hrefQueryParams(t, "http://domain.com/path?query=param&fruit=banana&query=other&empty"))
}
