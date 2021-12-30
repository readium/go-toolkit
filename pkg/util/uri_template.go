package util

import (
	"strings"

	"github.com/agext/regexp"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

/**
 * A lightweight implementation of URI Template (RFC 6570).
 *
 * Only handles simple cases, fitting Readium's use cases.
 * See https://tools.ietf.org/html/rfc6570
 */

type URITemplate struct {
	uri string
}

func NewURITemplate(uri string) URITemplate {
	return URITemplate{
		uri: uri,
	}
}

var paramRegex = regexp.MustCompile(`\{\??([^}]+)\}`)
var expandRegex = regexp.MustCompile(`\{(\??)([^}]+)\}`)

// List of URI template parameter keys.
func (u URITemplate) Parameters() []string {
	params := paramRegex.FindAllStringSubmatch(u.uri, -1)
	ret := make([]string, 0, len(params))
	for _, p := range params {
		if len(p) != 2 {
			continue
		}
		for _, v := range strings.Split(p[1], ",") {
			ret = extensions.AddToSet(ret, v)
		}
	}

	return ret
}

func expandSimpleString(s string, parameters map[string]string) string {
	strs := strings.Split(s, ",")
	for i, str := range strs {
		v, _ := parameters[str]
		strs[i] = v
	}
	return strings.Join(strs, ",")
}

func expandFormStyle(s string, parameters map[string]string) string {
	strs := strings.Split(s, ",")
	var sb strings.Builder
	sb.WriteRune('?')
	for i, str := range strs {
		v, _ := parameters[str]
		if i != 0 {
			sb.WriteRune('&')
		}
		sb.WriteString(str)
		sb.WriteRune('=')
		if v == "" {
			continue
		}
		sb.WriteString(v)
	}
	return sb.String()
}

// Expands the HREF by replacing URI template variables by the given parameters.
func (u URITemplate) Expand(parameters map[string]string) string {
	// `+` is considered like an encoded space, and will not be properly encoded in parameters.
	// This is an issue for ISO 8601 date for example.
	// As a workaround, we encode manually this character. We don't do it in the full URI,
	// because it could contain some legitimate +-as-space characters.
	for k, v := range parameters {
		parameters[k] = strings.Replace(v, "+", "~~+~~", -1)
	}

	href, _ := NewHREF(expandRegex.ReplaceAllStringSubmatchFunc(u.uri, func(s []string) string {
		if len(s) != 3 {
			return ""
		}
		if s[1] == "" {
			return expandSimpleString(s[2], parameters)
		} else {
			return expandFormStyle(s[2], parameters)
		}
	}), "").PercentEncodedString()

	return strings.ReplaceAll(strings.ReplaceAll(href, "~~%20~~", "%2B"), "~~+~~", "%2B")

}

func (u URITemplate) Description() string {
	return u.uri
}
