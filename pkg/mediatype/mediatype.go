package mediatype

import (
	"errors"
	"sort"
	"strings"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
)

// MediaType represents a document format, identified by a unique RFC 6838 media type.
// [MediaType] handles:
//  - components parsing – eg. type, subtype and parameters,
//  - media types comparison.
//
// Comparing media types is more complicated than it looks, since they can contain parameters,
// such as `charset=utf-8`. We can't ignore them because some formats use parameters in their
// media type, for example `application/atom+xml;profile=opds-catalog` for an OPDS 1 catalog.
//
// Specification: https://tools.ietf.org/html/rfc6838
type MediaType struct {
	Type       string            // The type component, e.g. `application` in `application/epub+zip`.
	SubType    string            // The subtype component, e.g. `epub+zip` in `application/epub+zip`.
	Parameters map[string]string // The parameters in the media type, such as `charset=utf-8`.
}

// Create a new MediaType
func NewMediaType(str string, name string, extension string) (mt MediaType, err error) {
	if str == "" {
		err = errors.New("Invalid empty media type")
		return
	}

	// Grammar: https://tools.ietf.org/html/rfc2045#section-5.1
	components := strings.Split(str, ";")
	for i, component := range components {
		components[i] = strings.TrimSpace(component)
	}
	types := strings.Split(components[0], "/")
	if len(types) != 2 {
		err = errors.New("Invalid media type: " + str)
		return
	}

	// > Both top-level type and subtype names are case-insensitive.
	mt.Type = strings.ToLower(types[0])
	mt.SubType = strings.ToLower(types[1])

	// > Parameter names are case-insensitive and no meaning is attached to the order in which they appear.
	parameters := make(map[string]string)
	for _, c := range components[1:] {
		frags := strings.Split(c, "=")
		if len(frags) == 2 {
			parameters[strings.ToLower(frags[0])] = frags[1]
		}
	}

	// For now, we only support case-insensitive `charset`.
	//
	// > Parameter values might or might not be case-sensitive, depending on the semantics of
	// > the parameter name.
	// > https://tools.ietf.org/html/rfc2616#section-3.7
	//
	// > The character set names may be up to 40 characters taken from the printable characters
	// > of US-ASCII.  However, no distinction is made between use of upper and lower case
	// > letters.
	// > https://www.iana.org/assignments/character-sets/character-sets.xhtml
	cs, ok := parameters["charset"]
	if ok {
		_, nam := charset.Lookup(cs)
		if nam != "" {
			cs = nam
		}
		parameters["charset"] = strings.ToUpper(cs)
	}

	mt.Parameters = parameters
	return
}

// Structured syntax suffix, e.g. `+zip` in `application/epub+zip`.
//
// Gives a hint on the underlying structure of this media type.
// See https://tools.ietf.org/html/rfc6838#section-4.2.8
func (mt MediaType) StructuredSyntaxSuffix() string {
	parts := strings.Split(mt.SubType, "+")
	if len(parts) > 1 {
		return "+" + parts[len(parts)-1]
	}
	return ""
}

// Encoding as declared in the `charset` parameter, if there's any.
func (mt MediaType) Charset() encoding.Encoding {
	if cs, ok := mt.Parameters["charset"]; ok {
		cs, _ := charset.Lookup(cs)
		return cs
	}
	return nil
}

// Returns the canonical version of this media type, if it is known.
//
// This is useful to find the name and file extension of a known media type, or to get the
// canonical media type from an alias. For example, `application/x-cbz` is an alias of the
// canonical `application/vnd.comicbook+zip`.
//
// Non-significant parameters are also discarded.
func (mt MediaType) CanonicalMediaType() MediaType {
	// TODO!
}

func (mt MediaType) buildQueryParams() string {
	if len(mt.Parameters) == 0 {
		return ""
	}
	rawParams := make([]string, len(mt.Parameters))
	i := uint16(0)
	for k, v := range mt.Parameters {
		rawParams[i] = k + "=" + v // Combine key and value into pair
		i += 1
	}
	sort.Strings(rawParams) // Sort slice for consistency
	return strings.Join(rawParams, ";")
}

// The string representation of this media type.
func (mt MediaType) String() string {
	if len(mt.Parameters) == 0 {
		// Shortcut parameter string construction
		return mt.Type + "/" + mt.SubType
	}
	params := mt.buildQueryParams()
	return mt.Type + "/" + mt.SubType + ";" + params
}

// For JSON Marshaling
func (mt MediaType) MarshalText() ([]byte, error) {
	return []byte(mt.String()), nil
}

// Returns whether the given [other] media type is included in this media type.
// For example, `text/html` contains `text/html;charset=utf-8`.
//
// - [other] must match the parameters in the [parameters] property, but extra parameters are ignored.
// - Order of parameters is ignored.
// - Wildcards are supported, meaning that `image///` contains `image/png` and `/////` contains everything.
func (mt MediaType) Contains(other *MediaType) bool {
	if other == nil || (mt.Type != "//" && mt.Type != other.Type) || (mt.SubType != "//" && mt.SubType != other.SubType) {
		return false
	}
	return mt.buildQueryParams() == other.buildQueryParams()
}

// Returns whether this media type and `other` are the same, ignoring parameters that are not in both media types.
// For example, `text/html` matches `text/html;charset=utf-8`, but `text/html;charset=ascii` doesn't. This is basically like `contains`, but working in both directions.
func (mt MediaType) Matches(other *MediaType) bool {
	co := mt.Contains(other)
	if other == nil {
		return co
	}
	return co || other.Contains(&mt)
}
