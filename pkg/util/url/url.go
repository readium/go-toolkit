package url

import (
	"errors"
	gurl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"golang.org/x/net/idna"
)

// A Uniform Resource Locator.
//
// https://url.spec.whatwg.org/
type URL interface {
	Path() string            // Decoded path segments identifying a location.
	Filename() string        // Decoded filename portion of the URL path.
	Extension() string       // Extension of the filename portion of the URL path.
	RemoveQuery() URL        // Returns a copy of this URL after dropping its query.
	Fragment() string        // Returns the decoded fragment present in this URL, if any.
	RemoveFragment() URL     // Returns a copy of this URL after dropping its fragment.
	Resolve(url URL) URL     // Resolves the given [url] to this URL.
	Relativize(url URL) URL  // Relativizes the given [url] against this URL.
	Normalize() URL          // Normalizes the URL using a subset of the RFC-3986 rules (https://datatracker.ietf.org/doc/html/rfc3986#section-6).
	String() string          // Encodes the URL to a string.
	Raw() *gurl.URL          // Returns the underlying Go URL.
	Equivalent(url URL) bool // Returns whether the receiver is equivalent to the given `url` after normalization.
}

// Creates a [RelativeURL] from a percent-decoded path.
func URLFromDecodedPath(path string) (RelativeURL, error) {
	return RelativeURLFromString(extensions.AddPercentEncodingPath(path))
}

// A proxy for [URLFromString] that panics on error.
func MustURLFromString(url string) URL {
	u, err := URLFromString(url)
	if err != nil {
		panic(err)
	}
	return u
}

// Creates a [URL] from its encoded string representation.
func URLFromString(url string) (URL, error) {
	u, err := gurl.Parse(url)
	if err != nil {
		return nil, err
	}
	return URLFromGo(u)
}

// Create a [URL] from a Go net/url URL.
func URLFromGo(url *gurl.URL) (URL, error) {
	if url.IsAbs() {
		return AbsoluteURLFromGo(url)
	} else {
		return RelativeURLFromGo(url)
	}
}

// Represents a relative Uniform Resource Locator.
// RelativeURL implements URL
type RelativeURL struct {
	url        *gurl.URL
	normalized bool
}

func (u RelativeURL) Path() string {
	return u.url.Path
}

// Filename implements URL
func (u RelativeURL) Filename() string {
	if strings.HasSuffix(u.url.Path, "/") {
		return ""
	}
	return path.Base(u.url.Path)
}

// Extension implements URL
func (u RelativeURL) Extension() string {
	return strings.TrimPrefix(path.Ext(u.Filename()), ".")
}

// RemoveQuery implements URL
func (u RelativeURL) RemoveQuery() URL {
	u.url.RawQuery = ""
	return RelativeURL{url: u.url, normalized: u.normalized}
}

// Fragment implements URL
func (u RelativeURL) Fragment() string {
	return u.url.Fragment
}

// RemoveFragment implements URL
func (u RelativeURL) RemoveFragment() URL {
	u.url.Fragment = ""
	return RelativeURL{url: u.url, normalized: u.normalized}
}

// Resolve implements URL
func (u RelativeURL) Resolve(url URL) URL {
	if _, ok := url.(AbsoluteURL); ok {
		return url
	} else if rel, ok := url.(RelativeURL); ok {
		res := u.url.ResolveReference(rel.url)

		// ResolveReference always adds a fowards slash to the path, even if the given URL has no slash prefix.
		// To match the other toolkits, we remove the slash if the URL and the given URL have no slash.
		if strings.HasPrefix(res.Path, "/") {
			if len(rel.url.Path) == 0 {
				res.Path = res.Path[1:]
			} else if !strings.HasPrefix(rel.url.Path, "/") && !strings.HasPrefix(u.url.Path, "/") {
				res.Path = res.Path[1:]
			}
		}

		return RelativeURL{url: res}
	} else {
		panic("URL type not supported")
	}
}

// Relativize implements URL
// Note that unlike other functions, this can return nil!
// Logic copied from Java: https://github.com/openjdk/jdk/blob/de90204b60c408ef258a2d2515ad252de4b23536/src/java.base/share/classes/java/net/URI.java#L2269
func (u RelativeURL) Relativize(url URL) URL {
	if url, ok := url.(RelativeURL); ok {
		if len(u.url.Opaque) > 0 || len(url.url.Opaque) > 0 {
			return url
		}
		if u.url.Scheme != url.url.Scheme && u.url.Host != url.url.Host {
			return url
		}

		bp := path.Clean(u.url.Path)
		cp := path.Clean(url.url.Path)
		if bp != cp {
			if !strings.HasSuffix(bp, "/") {
				bp = bp + "/"
			}
			if !strings.HasPrefix(cp, bp) {
				return url
			}
		}

		return RelativeURL{url: &gurl.URL{
			Path:       cp[len(bp):],
			Fragment:   url.url.Fragment,
			RawQuery:   url.url.RawQuery,
			ForceQuery: url.url.ForceQuery,
		}}
	}

	// Cannot relativize a relative URL against an non-relative URL.
	return url
}

// Normalize implements URL
func (u RelativeURL) Normalize() URL {
	if u.normalized {
		// Already normalized
		return u
	}

	var hadSlash bool
	if strings.HasSuffix(u.url.Path, "/") {
		hadSlash = true
	}
	u.url.Path = path.Clean(u.url.Path)
	if hadSlash {
		u.url.Path += "/"
	}

	return RelativeURL{url: u.url, normalized: true}
}

// String implements URL
func (u RelativeURL) String() string {
	return u.url.String()
}

// Raw implements URL
func (u RelativeURL) Raw() *gurl.URL {
	return u.url
}

// Equivalent implements URL
func (u RelativeURL) Equivalent(url URL) bool {
	return u.Normalize().String() == url.Normalize().String()
}

// Creates a [RelativeURL] from its encoded string representation.
func RelativeURLFromString(url string) (RelativeURL, error) {
	u, err := gurl.Parse(url)
	if err != nil {
		return RelativeURL{}, err
	}
	return RelativeURLFromGo(u)
}

// Create a [RelativeURL] from a Go net/url URL.
func RelativeURLFromGo(url *gurl.URL) (RelativeURL, error) {
	if url.IsAbs() {
		return RelativeURL{}, errors.New("URL is not relative")
	}
	return RelativeURL{url: url}, nil
}

type AbsoluteURL struct {
	url        *gurl.URL
	scheme     Scheme
	normalized bool
}

// Path implements URL
func (u AbsoluteURL) Path() string {
	return u.url.Path
}

// Filename implements URL
func (u AbsoluteURL) Filename() string {
	if strings.HasSuffix(u.url.Path, "/") {
		return ""
	}
	return path.Base(u.url.Path)
}

// Extension implements URL
func (u AbsoluteURL) Extension() string {
	return strings.TrimPrefix(path.Ext(u.Filename()), ".")
}

// RemoveQuery implements URL
func (u AbsoluteURL) RemoveQuery() URL {
	u.url.RawQuery = ""
	return AbsoluteURL{url: u.url, scheme: u.scheme, normalized: u.normalized}
}

// Fragment implements URL
func (u AbsoluteURL) Fragment() string {
	return u.url.Fragment
}

// RemoveFragment implements URL
func (u AbsoluteURL) RemoveFragment() URL {
	u.url.Fragment = ""
	return AbsoluteURL{url: u.url, scheme: u.scheme, normalized: u.normalized}
}

// Resolve implements URL
func (u AbsoluteURL) Resolve(url URL) URL {
	if _, ok := url.(AbsoluteURL); ok {
		return url
	} else if rel, ok := url.(RelativeURL); ok {
		res := u.url.ResolveReference(rel.url)
		return AbsoluteURL{url: res, scheme: u.scheme}
	} else {
		panic("URL type not supported")
	}
}

// Relativize implements URL
// Note that unlike other functions, this can return nil!
// Logic copied from Java: https://github.com/openjdk/jdk/blob/de90204b60c408ef258a2d2515ad252de4b23536/src/java.base/share/classes/java/net/URI.java#L2269
func (u AbsoluteURL) Relativize(url URL) URL {
	if url, ok := url.(AbsoluteURL); ok {
		if len(u.url.Opaque) > 0 || len(url.url.Opaque) > 0 {
			return url
		}
		if u.url.Scheme != url.url.Scheme && u.url.Host != url.url.Host {
			return url
		}

		bp := path.Clean(u.url.Path)
		cp := path.Clean(url.url.Path)
		if bp != cp {
			if !strings.HasSuffix(bp, "/") {
				bp = bp + "/"
			}
			if !strings.HasPrefix(cp, bp) {
				return url
			}
		}

		return RelativeURL{url: &gurl.URL{
			Path:       cp[len(bp):],
			Fragment:   url.url.Fragment,
			RawQuery:   url.url.RawQuery,
			ForceQuery: url.url.ForceQuery,
		}}
	}

	// Cannot relativize an absolute URL against a relative URL.
	return url
}

// Normalize implements URL
func (u AbsoluteURL) Normalize() URL {
	if u.normalized {
		// Already normalized
		return u
	}

	var hadSlash bool
	if strings.HasSuffix(u.url.Path, "/") {
		hadSlash = true
	}
	u.url.Path = path.Clean(u.url.Path)
	if hadSlash {
		u.url.Path += "/"
	}

	u.url.Scheme = SchemeFromString(u.url.Scheme).String()
	asciiHost, err := idna.ToASCII(u.url.Host)
	if err == nil {
		u.url.Host = asciiHost
	}

	return AbsoluteURL{url: u.url, scheme: Scheme(u.url.Scheme), normalized: true}
}

// String implements URL
func (u AbsoluteURL) String() string {
	return u.url.String()
}

// Raw implements URL
func (u AbsoluteURL) Raw() *gurl.URL {
	return u.url
}

// Equivalent implements URL
func (u AbsoluteURL) Equivalent(url URL) bool {
	return u.Normalize().String() == url.Normalize().String()
}

// Identifies the type of URL.
func (u AbsoluteURL) Scheme() Scheme {
	return u.scheme
}

// Indicates whether this URL points to a HTTP resource.
func (u AbsoluteURL) IsHTTP() bool {
	return u.scheme.IsHTTP()
}

// Indicates whether this URL points to a file.
func (u AbsoluteURL) IsFile() bool {
	return u.scheme.IsFile()
}

// Converts the URL to a filepath, if it's a file URL.
func (u AbsoluteURL) ToFilepath() string {
	if !u.IsFile() {
		return ""
	}
	return filepath.FromSlash(u.url.Path)
}

// Creates a [AbsoluteURL] from its encoded string representation.
func AbsoluteURLFromString(url string) (AbsoluteURL, error) {
	u, err := gurl.Parse(url)
	if err != nil {
		return AbsoluteURL{}, err
	}
	return AbsoluteURLFromGo(u)
}

// Create a [AbsoluteURL] from a Go net/url URL.
func AbsoluteURLFromGo(url *gurl.URL) (AbsoluteURL, error) {
	if !url.IsAbs() {
		return AbsoluteURL{}, errors.New("URL is not absolute")
	}
	scheme := SchemeFromString(url.Scheme)
	if scheme == "" {
		if url.Scheme == "" {
			return AbsoluteURL{}, errors.New("URL has no scheme")
		} else {
			return AbsoluteURL{}, errors.New("URL has an unsupported scheme")
		}
	}

	return AbsoluteURL{url: url, scheme: scheme}, nil
}

/*
According to the EPUB specification, the HREFs in the EPUB package must be valid URLs (so
percent-encoded). Unfortunately, many EPUBs don't follow this rule, and use invalid HREFs such
as `my chapter.html` or `/dir/my chapter.html`.

As a workaround, we assume the HREFs are valid percent-encoded URLs, and fallback to decoded paths
if we can't parse the URL.
*/
func FromEPUBHref(href string) (URL, error) {
	u, err := URLFromString(href)
	if err != nil {
		return URLFromDecodedPath(href)
	}
	return u, nil
}

func FromFilepath(path string) (URL, error) {
	return AbsoluteURLFromGo(&gurl.URL{
		Path:   filepath.ToSlash(path),
		Scheme: SchemeFile.String(),
	})
}

func FromLocalFile(file *os.File) (URL, error) {
	apath, err := filepath.Abs(file.Name())
	if err != nil {
		return nil, err
	}
	return FromFilepath(apath)
}
