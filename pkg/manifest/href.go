package manifest

import (
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/readium/go-toolkit/pkg/util/url/uritemplates"
)

// An hypertext reference points to a resource in a [Publication].
// It is potentially templated, use [Resolve] to get the actual URL.
type HREF struct {
	// Only one of these two is set in an instance.
	href     url.URL
	template string
}

// Creates an [HREF] from a valid URL.
func NewHREF(href url.URL) HREF {
	return HREF{href: href}
}

// Proxy for NewHREFFromString which panics if the URL is invalid.
func MustNewHREFFromString(href string, templated bool) HREF {
	h, err := NewHREFFromString(href, templated)
	if err != nil {
		panic(err)
	}
	return h
}

// Creates an [HREF] from a valid URL or URL template (RFC 6570).
// `templated` Indicates whether [href] is a URL template.
func NewHREFFromString(href string, templated bool) (HREF, error) {
	if templated {
		// Check that the produced URL is valid
		eurl, _, err := uritemplates.Expand(href, nil)
		if err != nil {
			return HREF{}, err
		}
		_, err = url.URLFromString(eurl)
		if err != nil {
			return HREF{}, err
		}
		return HREF{
			template: href,
		}, err
	} else {
		u, err := url.URLFromString(href)
		if err != nil {
			return HREF{}, err
		}
		return NewHREF(u), nil
	}
}

// Returns the URL represented by this HREF, resolved to the given [base] URL.
// If the HREF is a template, the [parameters] are used to expand it according to RFC 6570.
func (h HREF) Resolve(base url.URL, parameters map[string]string) url.URL {
	if h.IsTemplated() {
		exp, _, err := uritemplates.Expand(h.template, parameters)
		if err != nil {
			panic("Invalid URL template expansion: " + err.Error())
		}
		u, err := url.URLFromString(exp)
		if err != nil {
			panic("Invalid URL template expansion: " + err.Error())
		}
		if base == nil {
			return u
		}
		return base.Resolve(u)
	} else {
		if base == nil {
			return h.href
		}
		return base.Resolve(h.href)
	}
}

// Indicates whether this HREF is templated.
func (h HREF) IsTemplated() bool {
	return h.template != ""
}

// List of URI template parameter keys, if the HREF is templated.
func (h HREF) Parameters() []string {
	if h.IsTemplated() {
		v, _ := uritemplates.Values(h.template)
		return v
	}
	return []string{}
}

// Resolves the receiver HREF to the given [baseUrl].
func (h HREF) ResolveTo(baseURL url.URL) HREF {
	if h.IsTemplated() {
		// WARNING: Cannot safely resolve a URI template to a base URL before expanding it
	} else {
		h.href = baseURL.Resolve(h.href)
	}
	return h
}

func (h HREF) String() string {
	if h.IsTemplated() {
		return h.template
	}
	return h.href.String()
}
