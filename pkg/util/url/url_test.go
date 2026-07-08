package url

import (
	gurl "net/url"
	"path/filepath"
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

// A URL that differs from the base only by host (or only by scheme) must not be
// relativized, even when its path shares the base's path prefix: dropping the host
// would silently repoint the URL at the base's origin.
func TestRelativizeDifferentOriginSamePath(t *testing.T) {
	base, _ := URLFromString("http://example.com/foo/")
	for _, k := range []string{
		"http://other-host.invalid/foo/quz/baz", // Same scheme, different host
		"https://example.com/foo/quz/baz",       // Different scheme, same host
	} {
		u, _ := URLFromString(k)
		assert.Equal(t, u, base.Relativize(u), k)
	}
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

// The methods documented as returning copies must not mutate the receiver's
// underlying net/url.URL, which is shared between the copies.
func TestRemoveFragmentReturnsCopy(t *testing.T) {
	for _, urlTest := range []string{
		"chapter.xhtml?q=1#fragment",
		"http://example.com/chapter.xhtml?q=1#fragment",
	} {
		u := MustURLFromString(urlTest)
		removed := u.RemoveFragment()
		assert.Empty(t, removed.Fragment(), "for URL '%s'", urlTest)
		assert.Equal(t, "fragment", u.Fragment(), "original URL '%s' must keep its fragment", urlTest)
		assert.Equal(t, urlTest, u.String(), "original URL '%s' must be unchanged", urlTest)
	}
}

func TestRemoveQueryReturnsCopy(t *testing.T) {
	for _, urlTest := range []string{
		"chapter.xhtml?q=1#fragment",
		"http://example.com/chapter.xhtml?q=1#fragment",
	} {
		u := MustURLFromString(urlTest)
		removed := u.RemoveQuery()
		assert.Empty(t, removed.Raw().RawQuery, "for URL '%s'", urlTest)
		assert.Equal(t, "fragment", removed.Fragment(), "RemoveQuery must keep the fragment of '%s'", urlTest)
		assert.Equal(t, urlTest, u.String(), "original URL '%s' must be unchanged", urlTest)
	}
}

func TestNormalizeReturnsCopy(t *testing.T) {
	for _, urlTest := range []struct {
		url        string
		normalized string
	}{
		{"foo/../bar.xhtml", "bar.xhtml"},
		{"http://example.com/foo/../bar.xhtml", "http://example.com/bar.xhtml"},
	} {
		u := MustURLFromString(urlTest.url)
		assert.Equal(t, urlTest.normalized, u.Normalize().String())
		assert.Equal(t, urlTest.url, u.String(), "original URL '%s' must be unchanged", urlTest.url)

		// Equivalent uses Normalize internally and must not mutate either side
		other := MustURLFromString(urlTest.normalized)
		assert.True(t, u.Equivalent(other))
		assert.Equal(t, urlTest.url, u.String(), "Equivalent must not mutate the receiver")
	}
}

// Windows drive paths round-trip through file URLs in the file:///C:/dir form,
// so that resolving relative references does not corrupt the drive letter.
func TestFilepathWindowsDrive(t *testing.T) {
	base, err := AbsoluteURLFromString("file:///C:/pub/manifest.json")
	if !assert.NoError(t, err) {
		return
	}
	rel, err := RelativeURLFromString("../audio/track.mp3")
	if !assert.NoError(t, err) {
		return
	}
	resolved := base.Resolve(rel).(AbsoluteURL)
	assert.Equal(t, "file:///C:/audio/track.mp3", resolved.String())
	assert.Equal(t, filepath.FromSlash("C:/audio/track.mp3"), resolved.ToFilepath())
}

func TestFilepathUnix(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("unix-style absolute path expectations are not stable on Windows")
	}
	u, err := FromFilepath("/pub/manifest.json")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "file:///pub/manifest.json", u.String())
	assert.Equal(t, filepath.FromSlash("/pub/manifest.json"), u.(AbsoluteURL).ToFilepath())
}

// A relative filepath is made absolute, so that resolving relative references against
// the resulting URL cannot silently turn it into a file system-absolute path.
func TestFilepathRelative(t *testing.T) {
	u, err := FromFilepath(filepath.Join("some", "relative", "file.txt"))
	if !assert.NoError(t, err) {
		return
	}
	abs, err := filepath.Abs(filepath.Join("some", "relative", "file.txt"))
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, abs, u.(AbsoluteURL).ToFilepath())
}
