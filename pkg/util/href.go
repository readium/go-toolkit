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
		base = "/" // TODO check if works
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
	uri, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	if uri.IsAbs() {
		return href, nil
	}

	// Isolates the path from the anchor/query portion, which would be lost otherwise.
	splitIndex := strings.Index(href, "?")
	if splitIndex == -1 {
		splitIndex = strings.Index(href, "#")
		if splitIndex == -1 {
			splitIndex = len(href)
		}
	}
	suffix := href[splitIndex:]

	// path := href[0:splitIndex]
	// TODO determine if the rest is necessary https://github.com/readium/kotlin-toolkit/blob/6f9f5914090625cfc4f46637970bb94992d9f692/readium/shared/src/main/java/org/readium/r2/shared/util/Href.kt#L56

	baseuri, err := url.Parse(baseHref)
	if err != nil {
		return "", err
	}

	uri = baseuri.ResolveReference(uri)
	var url string
	if uri.Scheme == "https" || uri.Scheme == "http" {
		url = uri.String()
	} else {
		url = uri.Path + suffix
		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}
	}
	return extensions.RemovePercentEncoding(url), nil
}

func (h HREF) PercentEncodedString() (string, error) {
	str, err := h.String()
	if err != nil {
		return "", err
	}
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
	return strings.TrimPrefix(ui.String(), "file://"), nil // TODO: does this need forced ASCII?
}

// TODO queryParameters
