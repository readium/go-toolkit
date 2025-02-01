package url

import (
	gurl "net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFromInvalidURL(t *testing.T) {
	urlTests := []string{
		"f:///////f",
		":C",
	}
	for _, urlTest := range urlTests {
		_, err := URLFromString(urlTest)
		assert.Error(t, err, "Expected error parsing URL '%s'", urlTest)
	}
}

func TestCreateFromRelativePath(t *testing.T) {
	for _, urlTest := range []string{
		"/foo/bar",
		"foo/bar",
		"../bar",
	} {
		a, err := RelativeURLFromString(urlTest)
		if assert.NoError(t, err) {
			b, err := URLFromString(urlTest)
			if assert.NoError(t, err) {
				assert.Equal(t, a, b)
			}
		}
	}

	// Special characters valid in a path.
	u, err := RelativeURLFromString("$&+,/=@")
	if assert.NoError(t, err) {
		assert.Equal(t, "$&+,/=@", u.Path())
	}

	// Used in the EPUB parser
	uu, err := URLFromString("#")
	if assert.NoError(t, err) {
		assert.Empty(t, uu.Path())
		assert.Empty(t, uu.Fragment())
	}
}

func TestCreateFromFragmentOnly(t *testing.T) {
	u, err := URLFromString("#fragment")
	if assert.NoError(t, err) {
		guu, err := gurl.Parse("#fragment")
		if assert.NoError(t, err) {
			uu, err := RelativeURLFromGo(guu)
			if assert.NoError(t, err) {
				assert.Equal(t, uu, u)
			}
		}
	}
}

func TestCreateFromQueryOnly(t *testing.T) {
	u, err := URLFromString("?query=param")
	if assert.NoError(t, err) {
		guu, err := gurl.Parse("?query=param")
		if assert.NoError(t, err) {
			uu, err := RelativeURLFromGo(guu)
			if assert.NoError(t, err) {
				assert.Equal(t, uu, u)
			}
		}
	}
}

func TestCreateFromAbsoluteURL(t *testing.T) {
	u, err := URLFromString("http://example.com/foo")
	if assert.NoError(t, err) {
		guu, err := gurl.Parse("http://example.com/foo")
		if assert.NoError(t, err) {
			uu, err := AbsoluteURLFromGo(guu)
			if assert.NoError(t, err) {
				assert.Equal(t, uu, u)
			}
		}
	}

	u, err = URLFromString("file:///foo/bar")
	if assert.NoError(t, err) {
		guu, err := gurl.Parse("file:///foo/bar")
		if assert.NoError(t, err) {
			uu, err := AbsoluteURLFromGo(guu)
			if assert.NoError(t, err) {
				assert.Equal(t, uu, u)
			}
		}
	}
}

func TestString(t *testing.T) {
	for _, urlTest := range []string{
		"foo/bar?query#fragment",
		"http://example.com/foo/bar?query#fragment",
		"file:///foo/bar?query#fragment",
	} {
		u, err := URLFromString(urlTest)
		if assert.NoError(t, err) {
			assert.Equal(t, urlTest, u.String())
		}
	}
}

func TestPath(t *testing.T) {
	for k, v := range map[string]string{
		"foo/bar?query#fragment":                    "foo/bar",
		"http://example.com/foo/bar/":               "/foo/bar/",
		"http://example.com/foo/bar?query#fragment": "/foo/bar",
		"file:///foo/bar/":                          "/foo/bar/",
		"file:///foo/bar?query#fragment":            "/foo/bar",
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.Path())
		}
	}
}

func TestPathFromEmptyRelativeURL(t *testing.T) {
	u, err := RelativeURLFromString("#fragment")
	if assert.NoError(t, err) {
		assert.Empty(t, u.Path())
	}
}

func TestPathIsPercentDecoded(t *testing.T) {
	for k, v := range map[string]string{
		"foo/%25bar%20quz":                    "foo/%bar quz",
		"http://example.com/foo/%25bar%20quz": "/foo/%bar quz",
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.Path())
		}
	}
}

func TestFilename(t *testing.T) {
	for k, v := range map[string]string{
		"foo/bar?query#fragment":                    "bar",
		"foo/bar/?query#fragment":                   "",
		"http://example.com/foo/bar?query#fragment": "bar",
		"http://example.com/foo/bar/":               "",
		"file:///foo/bar?query#fragment":            "bar",
		"file:///foo/bar/":                          "",
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.Filename())
		}
	}
}

func TestFilenameIsPercentDecoded(t *testing.T) {
	for k, v := range map[string]string{
		"foo/%25bar%20quz":                    "%bar quz",
		"http://example.com/foo/%25bar%20quz": "%bar quz",
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.Filename())
		}
	}
}

func TestExtension(t *testing.T) {
	for k, v := range map[string]string{
		"foo/bar.txt?query#fragment":                    "txt",
		"foo/bar?query#fragment":                        "",
		"foo/bar/?query#fragment":                       "",
		"http://example.com/foo/bar.txt?query#fragment": "txt",
		"http://example.com/foo/bar?query#fragment":     "",
		"http://example.com/foo/bar/":                   "",
		"file:///foo/bar.txt?query#fragment":            "txt",
		"file:///foo/bar?query#fragment":                "",
		"file:///foo/bar/":                              "",
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.Extension())
		}
	}
}

func TestExtensionIsPercentDecoded(t *testing.T) {
	for k, v := range map[string]string{
		"foo.%25bar":                    "%bar",
		"http://example.com/foo.%25bar": "%bar",
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.Extension())
		}
	}
}

func TestScheme(t *testing.T) {
	for k, v := range map[string]Scheme{
		"file:///foo/bar":         SchemeFromString("file"),
		"FILE:///foo/bar":         SchemeFromString("file"),
		"http://example.com/foo":  SchemeFromString("http"),
		"https://example.com/foo": SchemeFromString("https"),
	} {
		u, err := URLFromString(k)
		if assert.NoError(t, err) {
			assert.Equal(t, v, u.(AbsoluteURL).Scheme())
		}
	}

	u, _ := URLFromString("file:///foo/bar")
	assert.True(t, u.(AbsoluteURL).Scheme().IsFile())
	assert.False(t, u.(AbsoluteURL).Scheme().IsHTTP())

	u, _ = URLFromString("http://example.com/foo")
	assert.True(t, u.(AbsoluteURL).Scheme().IsHTTP())
	assert.False(t, u.(AbsoluteURL).Scheme().IsFile())

	u, _ = URLFromString("https://example.com/foo")
	assert.True(t, u.(AbsoluteURL).Scheme().IsHTTP())
}

func TestResolveHttpURL(t *testing.T) {
	base, _ := URLFromString("http://example.com/foo/bar")
	for k, v := range map[string]string{
		"quz/baz":         "http://example.com/foo/quz/baz",
		"../quz/baz":      "http://example.com/quz/baz",
		"/quz/baz":        "http://example.com/quz/baz",
		"#fragment":       "http://example.com/foo/bar#fragment",
		"file:///foo/bar": "file:///foo/bar",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}

	// With trailing slash
	base, _ = URLFromString("http://example.com/foo/bar/")
	for k, v := range map[string]string{
		"quz/baz":    "http://example.com/foo/bar/quz/baz",
		"../quz/baz": "http://example.com/foo/quz/baz",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}
}

func TestResolveFileURL(t *testing.T) {
	base, _ := URLFromString("file:///root/foo/bar")
	for k, v := range map[string]string{
		"quz":                        "file:///root/foo/quz",
		"quz/baz":                    "file:///root/foo/quz/baz",
		"../quz":                     "file:///root/quz",
		"/quz/baz":                   "file:///quz/baz",
		"http://example.com/foo/bar": "http://example.com/foo/bar",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}

	// With trailing slash
	base, _ = URLFromString("file:///root/foo/bar/")
	for k, v := range map[string]string{
		"quz/baz": "file:///root/foo/bar/quz/baz",
		"../quz":  "file:///root/foo/quz",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}
}

func TestResolveTwoRelativeURLs(t *testing.T) {
	base, _ := URLFromString("foo/bar")
	for k, v := range map[string]string{
		"quz/baz":                    "foo/quz/baz",
		"../quz/baz":                 "quz/baz",
		"/quz/baz":                   "/quz/baz",
		"#fragment":                  "foo/bar#fragment",
		"http://example.com/foo/bar": "http://example.com/foo/bar",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}

	// With trailing slash
	base, _ = URLFromString("foo/bar/")
	for k, v := range map[string]string{
		"quz/baz":    "foo/bar/quz/baz",
		"../quz/baz": "foo/quz/baz",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}

	// With starting slash
	base, _ = URLFromString("/foo/bar")
	for k, v := range map[string]string{
		"quz/baz":  "/foo/quz/baz",
		"/quz/baz": "/quz/baz",
	} {
		u, _ := URLFromString(v)
		ur, _ := URLFromString(k)
		assert.Equal(t, u, base.Resolve(ur))
	}
}

func TestRelativizeHttpURL(t *testing.T) {
	base, _ := URLFromString("http://example.com/foo")
	for k, v := range map[string]string{
		"http://example.com/foo/quz/baz":   "quz/baz",
		"http://example.com/foo#fragment":  "#fragment",
		"http://example.com/foo/#fragment": "#fragment",
		"file:///foo/bar":                  "file:///foo/bar",
	} {
		u, _ := URLFromString(k)
		ur, _ := URLFromString(v)
		assert.Equal(t, ur, base.Relativize(u))
	}

	// With trailing slash
	base, _ = URLFromString("http://example.com/foo/")
	u, _ := URLFromString("http://example.com/foo/quz/baz")
	ur, _ := URLFromString("quz/baz")
	assert.Equal(t, ur, base.Relativize(u))
}

func TestRelativizeFileURL(t *testing.T) {
	base, _ := URLFromString("file:///root/foo")
	for k, v := range map[string]string{
		"file:///root/foo/quz/baz":   "quz/baz",
		"http://example.com/foo/bar": "http://example.com/foo/bar",
	} {
		u, _ := URLFromString(k)
		ur, _ := URLFromString(v)
		assert.Equal(t, ur, base.Relativize(u))
	}

	// With trailing slash
	base, _ = URLFromString("file:///root/foo/")
	u, _ := URLFromString("file:///root/foo/quz/baz")
	ur, _ := URLFromString("quz/baz")
	assert.Equal(t, ur, base.Relativize(u))
}

func TestRelativizeTwoRelativeURLs(t *testing.T) {
	base, _ := URLFromString("foo")
	for k, v := range map[string]string{
		"foo/quz/baz":                "quz/baz",
		"quz/baz":                    "quz/baz",
		"/quz/baz":                   "/quz/baz",
		"foo#fragment":               "#fragment",
		"foo/#fragment":              "#fragment",
		"http://example.com/foo/bar": "http://example.com/foo/bar",
	} {
		u, _ := URLFromString(k)
		ur, _ := URLFromString(v)
		assert.Equal(t, ur, base.Relativize(u))
	}

	// With trailing slash
	base, _ = URLFromString("foo/")
	u, _ := URLFromString("foo/quz/baz")
	ur, _ := URLFromString("quz/baz")
	assert.Equal(t, ur, base.Relativize(u))

	// With starting slash
	base, _ = URLFromString("/foo")
	u, _ = URLFromString("/foo/quz/baz")
	ur, _ = URLFromString("quz/baz")
	assert.Equal(t, ur, base.Relativize(u))
}

func TestFromFile(t *testing.T) {
	u, _ := AbsoluteURLFromString("file:///tmp/test.txt")
	f, _ := FromFilepath("/tmp/test.txt")
	assert.Equal(t, u, f)
}

func TestToFile(t *testing.T) {
	u, _ := AbsoluteURLFromString("file:///tmp/test.txt")
	assert.Equal(t, "/tmp/test.txt", u.ToFilepath())
}

func TestNormalize(t *testing.T) {
	// Scheme is lower case.
	u, _ := URLFromString("HTTP://example.com/foo")
	assert.Equal(t, "http://example.com/foo", u.Normalize().String())

	// Host becomes punycode equivalent.
	u, _ = URLFromString("http://도메인.com/foo")
	assert.Equal(t, "http://xn--hq1bm8jm9l.com/foo", u.Normalize().String())

	// Percent encoding of path is normalized.
	u, _ = URLFromString("HTTP://example.com/c'est%20valide")
	assert.Equal(t, "http://example.com/c'est%20valide", u.Normalize().String())
	u, _ = URLFromString("c'est%20valide")
	assert.Equal(t, "c'est%20valide", u.Normalize().String())

	// Relative paths are resolved.
	u, _ = URLFromString("http://example.com/foo/./bar//../baz")
	assert.Equal(t, "http://example.com/foo/baz", u.Normalize().String())
	u, _ = URLFromString("foo/./bar//../baz")
	assert.Equal(t, "foo/baz", u.Normalize().String())
	u, _ = URLFromString("foo/./bar/../../../baz")
	assert.Equal(t, "../baz", u.Normalize().String())

	// Trailing slash is kept.
	u, _ = URLFromString("http://example.com/foo/")
	assert.Equal(t, "http://example.com/foo/", u.Normalize().String())

	// The other components are left as-is.
	u, _ = URLFromString("http://user:password@example.com:443/foo?b=b&a=a#fragment")
	assert.Equal(t, "http://user:password@example.com:443/foo?b=b&a=a#fragment", u.Normalize().String())
}
