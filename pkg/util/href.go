package util

import (
	"net/url"
	"strings"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"golang.org/x/net/idna"
)

type QueryParameter struct {
	name  string
	value string
}

type HREF struct {
	href     string
	baseHref string
}

func NewHREF(href string, base string) HREF {
	if base == "" {
		base = "/"
	}
	return HREF{href: href, baseHref: base}
}

// Returns the normalized string representation for this HREF.
func (h HREF) String() (string, error) {
	baseHref := extensions.RemovePercentEncoding(h.baseHref)
	href := extensions.RemovePercentEncoding(h.href)

	// HREF is just an anchor inside the base.
	if strings.TrimSpace(href) == "" || strings.HasPrefix(href, "#") {
		return baseHref + href, nil
	}

	// HREF is already absolute.
	uri, err := url.Parse(extensions.AddPercentEncodingPath(href))
	if err != nil {
		return "", err
	}
	if uri.IsAbs() {
		return href, nil
	}

	baseuri, err := url.Parse(extensions.AddPercentEncodingPath(baseHref))
	if err != nil {
		return "", err
	}

	uri = baseuri.ResolveReference(uri)
	var url string
	if uri.Scheme == "https" || uri.Scheme == "http" {
		url = uri.String()
	} else {
		url = uri.String()
		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}
	}
	return extensions.RemovePercentEncoding(url), nil
}

// Returns the normalized string representation for this HREF, encoded for URL uses.
func (h HREF) PercentEncodedString() (string, error) {
	str, err := h.String()
	if err != nil {
		return "", err
	}
	str = extensions.AddPercentEncodingPath(str)
	if strings.HasPrefix(str, "/") {
		str = "file://" + str
	}

	ul, err := url.Parse(str)
	if err != nil {
		return "", err
	}

	idh, err := idna.ToASCII(ul.Hostname())
	if err != nil {
		idh = ul.Hostname()
	}
	if ul.Port() != "" {
		idh = idh + ":" + ul.Port()
	}

	ui := url.URL{
		Scheme:   ul.Scheme,
		Opaque:   ul.Opaque,
		User:     ul.User,
		Host:     idh,
		Path:     ul.Path,
		Fragment: ul.Fragment,
		RawQuery: ul.RawQuery,
	}
	return strings.TrimPrefix(ui.String(), "file://"), nil // TODO: why (or why not) does this need forced ASCII?
}

// Returns the query parameters present in this HREF, in the order they appear.
func (h HREF) QueryParameters() (url.Values, error) {
	ul, err := h.PercentEncodedString()
	if err != nil {
		return nil, err
	}
	ulx, err := url.Parse(ul)
	if err != nil {
		return nil, err
	}
	return ulx.Query(), nil
}
