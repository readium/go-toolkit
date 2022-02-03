package mediatype

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding/unicode"
)

func TestMediatypeErrorForInvalidTypes(t *testing.T) {
	_, err := NewOfString("application")
	assert.Error(t, err, "parser should return error because MediaType doesn't have 2 components")
	_, err = NewOfString("application/atom+xml/extra")
	assert.Error(t, err, "parser should return error because MediaType doesn't have 2 components")
}

func TestMediatypeToString(t *testing.T) {
	mt, err := NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	// Note there is a space between the mediatype semicolon and params. This is the behavior
	// of Go's mime formatter, and differs from the Kotlin implementation
	assert.Equal(t, "application/atom+xml; profile=opds-catalog", mt.String(), "MediaType should render to this string")
}

func TestMediatypeToStringIsNormalized(t *testing.T) {
	mt, err := NewOfString("APPLICATION/ATOM+XML;PROFILE=OPDS-CATALOG   ;   a=0")
	assert.NoError(t, err)
	assert.Equal(t, "application/atom+xml; a=0; profile=OPDS-CATALOG", mt.String(), "MediaType should have the correct final casing")

	mt, err = NewOfString("application/atom+xml;a=0;b=1")
	assert.NoError(t, err)
	assert.Equal(t, "application/atom+xml; a=0; b=1", mt.String(), "MediaType should output as it was input")

	mt, err = NewOfString("application/atom+xml;b=1;a=0")
	assert.NoError(t, err)
	assert.Equal(t, "application/atom+xml; a=0; b=1", mt.String(), "MediaType should have alphabetically sorted parameters")
}

func TestMediatypeGetType(t *testing.T) {
	mt, err := NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	assert.Equal(t, "application", mt.Type, "MediaType type should be equal to \"application\"")

	mt, err = NewOfString("*/jpeg")
	assert.NoError(t, err)
	assert.Equal(t, "*", mt.Type, "MediaType type should be equal to \"*\"")
}

func TestMediatypeGetSubtype(t *testing.T) {
	mt, err := NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	assert.Equal(t, "atom+xml", mt.SubType, "MediaType subtype should be equal to \"atom+xml\"")

	mt, err = NewOfString("image/*")
	assert.NoError(t, err)
	assert.Equal(t, "*", mt.SubType, "MediaType subtype should be equal to \"*\"")
}

func TestMediatypeGetParameters(t *testing.T) {
	mt, err := NewOfString("application/atom+xml;type=entry;profile=opds-catalog")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"type": "entry", "profile": "opds-catalog"}, mt.Parameters, "MediaType parameters should match the given map")
}

func TestMediatypeGetEmptyParameters(t *testing.T) {
	mt, err := NewOfString("application/atom+xml")
	assert.NoError(t, err)
	assert.True(t, len(mt.Parameters) == 0, "MediaType should have no parameters in its map")
}

func TestMediatypeGetParametersWithWhitespaces(t *testing.T) {
	mt, err := NewOfString("application/atom+xml    ;    type=entry   ;    profile=opds-catalog   ")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"type": "entry", "profile": "opds-catalog"}, mt.Parameters, "MediaType parameters should match the given map")
}

func TestMediatypeGetStructuredSyntaxSuffix(t *testing.T) {
	mt, err := NewOfString("foo/bar")
	assert.NoError(t, err)
	assert.Empty(t, mt.StructuredSyntaxSuffix(), "MediaType should have no structured syntax suffix")

	mt, err = NewOfString("application/zip")
	assert.NoError(t, err)
	assert.Empty(t, mt.StructuredSyntaxSuffix(), "MediaType should have no structured syntax suffix")

	mt, err = NewOfString("application/epub+zip")
	assert.NoError(t, err)
	assert.Equal(t, "+zip", mt.StructuredSyntaxSuffix(), "structured syntax suffix should be \"+zip\"")

	mt, err = NewOfString("foo/bar+json+zip")
	assert.NoError(t, err)
	assert.Equal(t, "+zip", mt.StructuredSyntaxSuffix(), "structured syntax suffix should be \"+zip\"")
}

func TestMediatypeGetCharset(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.Nil(t, mt.Charset(), "MediaType should have no charset")

	mt, err = NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.Equal(t, unicode.UTF8, mt.Charset(), "charset should be utf-8")

	mt, err = NewOfString("text/html;charset=utf-16")
	assert.NoError(t, err)
	assert.Equal(t, unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), mt.Charset(), "charset should be utf-16 le (ignore bom)")
}

func TestMediatypeAllLowercased(t *testing.T) {
	mt, err := NewOfString("APPLICATION/ATOM+XML;PROFILE=OPDS-CATALOG")
	assert.NoError(t, err)
	assert.Equal(t, "application", mt.Type, "type should be lowercased")
	assert.Equal(t, "atom+xml", mt.SubType, "subtype should be lowercased")
	assert.Equal(t, map[string]string{"profile": "OPDS-CATALOG"}, mt.Parameters, "parameter keys should be lowercased")
}

func TestMediatypeChartsetValueIsUppercased(t *testing.T) {
	mt, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.Equal(t, "UTF-8", mt.Parameters["charset"], "charset value should be uppercased")
}

func TestMediatypeCharsetValueCanonicalized(t *testing.T) {
	mt, err := NewOfString("text/html;charset=ascii")
	assert.NoError(t, err)
	assert.Equal(t, "WINDOWS-1252", mt.Parameters["charset"], "charset should be WINDOWS-1252, the ascii equivalent")

	mt, err = NewOfString("text/html;charset=unknown")
	assert.NoError(t, err)
	assert.Equal(t, "UNKNOWN", mt.Parameters["charset"], "charset should be unknown")
}

func TestMediatypeCanonicalize(t *testing.T) {
	mt1, err := New("text/html", "", "html")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.Equal(t, &mt1, mt2.CanonicalMediaType())

	mt1, err = NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	mt2, err = NewOfString("application/atom+xml;profile=opds-catalog;charset=utf-8")
	assert.NoError(t, err)
	assert.Equal(t, &mt1, mt2.CanonicalMediaType())

	mt1, err = NewOfString("application/unknown;charset=utf-8")
	assert.NoError(t, err)
	mt2, err = NewOfString("application/unknown;charset=utf-8")
	assert.NoError(t, err)
	assert.Equal(t, &mt1, mt2.CanonicalMediaType())
}

func TestMediatypeEquality(t *testing.T) {
	mt1, err := NewOfString("application/atom+xml")
	assert.NoError(t, err)
	mt2, err := NewOfString("application/atom+xml")
	assert.NoError(t, err)
	assert.Equal(t, mt1, mt2)

	mt1, err = NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	mt2, err = NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	assert.Equal(t, mt1, mt2)

	mt1, err = NewOfString("application/atom+xml")
	assert.NoError(t, err)
	mt2, err = NewOfString("application/atom")
	assert.NoError(t, err)
	assert.NotEqual(t, mt1, mt2)

	mt1, err = NewOfString("application/atom+xml")
	assert.NoError(t, err)
	mt2, err = NewOfString("text/atom+xml")
	assert.NoError(t, err)
	assert.NotEqual(t, mt1, mt2)

	mt1, err = NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	mt2, err = NewOfString("application/atom+xml")
	assert.NoError(t, err)
	assert.NotEqual(t, mt1, mt2)

	// Using the [Equal] function
	mt1, err = NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt2, err = NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt1.Equal(&HTML))
	assert.True(t, mt2.Equal(&HTML))
	assert.False(t, mt1.Equal(&mt2))
	assert.True(t, mt1.Equal(&mt1))
}

// More specifically, equality ignores case of type, subtype and parameter names (but not parameter values!)
func TestMediatypeEqualityIgnoresCases(t *testing.T) {
	mt1, err := NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	mt2, err := NewOfString("APPLICATION/ATOM+XML;PROFILE=opds-catalog")
	assert.NoError(t, err)
	assert.Equal(t, mt1, mt2)

	mt1, err = NewOfString("application/atom+xml;profile=opds-catalog")
	assert.NoError(t, err)
	mt2, err = NewOfString("APPLICATION/ATOM+XML;PROFILE=OPDS-CATALOG")
	assert.NoError(t, err)
	assert.NotEqual(t, mt1, mt2)
}

func TestMediatypeEqualityIgnoresParameterOrder(t *testing.T) {
	mt1, err := NewOfString("application/atom+xml;type=entry;profile=opds-catalog")
	assert.NoError(t, err)
	mt2, err := NewOfString("application/atom+xml;profile=opds-catalog;type=entry")
	assert.NoError(t, err)
	assert.Equal(t, mt1, mt2)
}

func TestMediatypeEqualityIgnoresCharsetCase(t *testing.T) {
	mt1, err := NewOfString("application/atom+xml;charset=utf-8")
	assert.NoError(t, err)
	mt2, err := NewOfString("application/atom+xml;charset=UTF-8")
	assert.NoError(t, err)
	assert.Equal(t, mt1, mt2, "charset parameter should be case-insensitive")
}

func TestMediatypeContainsEqual(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt1.Contains(&mt2))
}

func TestMediatypeContainsParametersMatching(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=ascii")
	assert.NoError(t, err)
	assert.False(t, mt1.Contains(&mt2), "MediaTypes with different charsets should not be equal")

	mt2, err = NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt1.Contains(&mt2), "MediaType with/without charset should not be interchangeable")
}

func TestMediatypeContainsIgnoresParameterOrder(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8;type=entry")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;type=entry;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt1.Contains(&mt2), "MediaTypes should ignore parameter order")
}

func TestMediatypeContainsIgnoresExtraParameters(t *testing.T) {
	mt1, err := NewOfString("text/html")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt1.Contains(&mt2), "MediaType contains should ignore extra parameters")
}

func TestMediatypeContainsSupportsWildcards(t *testing.T) {
	mt1, err := NewOfString("*/*")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt1.Contains(&mt2), "wildcards should contain anything")

	mt1, err = NewOfString("text/*")
	assert.NoError(t, err)
	assert.True(t, mt1.Contains(&mt2), "text/* should contain text/html")

	mt2, err = NewOfString("application/zip")
	assert.NoError(t, err)
	assert.False(t, mt1.Contains(&mt2))
}

func TestMediatypeContainsFromString(t *testing.T) {
	mt, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt.ContainsFromString("text/html;charset=utf-8"))
}

func TestMediatypeMatchesEqual(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt1.Matches(&mt2), "two identical MediaTypes should match")
}

func TestMediatypeMatchesParametersMatching(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=ascii")
	assert.NoError(t, err)
	assert.False(t, mt1.Matches(&mt2), "MediaTypes with different charsets should not match")
}

func TestMediatypeMatchesIgnoresParameterOrder(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8;type=entry")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;type=entry;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt1.Matches(&mt2), "MediaType matches should ignore parameter order")
}

func TestMediatypeMatchesIgnoresExtraParameters(t *testing.T) {
	mt1, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8;extra=param")
	assert.NoError(t, err)
	assert.True(t, mt1.Matches(&mt2), "MediaType matches should ignore extra parameters")
	assert.True(t, mt2.Matches(&mt1), "MediaType matches should ignore extra parameters")
}

func TestMediatypeMatchesSupportsWildcards(t *testing.T) {
	mt1, err := NewOfString("*/*")
	assert.NoError(t, err)
	mt2, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt2.Matches(&mt1), "anything should match with a wildcard MediaType")
	assert.True(t, mt1.Matches(&mt2), "anything should match with a wildcard MediaType")

	mt1, err = NewOfString("text/*")
	assert.NoError(t, err)
	assert.True(t, mt2.Matches(&mt1), "text/html should match text/*")
	assert.True(t, mt1.Matches(&mt2), "text/html should match text/*")

	mt2, err = NewOfString("application/zip")
	assert.NoError(t, err)
	assert.False(t, mt2.Matches(&mt1))
	assert.False(t, mt1.Matches(&mt2))
}

func TestMediatypeMatchesFromString(t *testing.T) {
	mt, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt.MatchesFromString("text/html;charset=utf-8"))
}

func TestMediatypeMatchesAny(t *testing.T) {
	mt1, err := NewOfString("text/html")
	assert.NoError(t, err)
	mt2, err := NewOfString("application/zip")
	assert.NoError(t, err)
	mt3, err := NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	mt4, err := NewOfString("text/plain;charset=utf-8")
	assert.NoError(t, err)

	assert.True(t, mt1.Matches(&mt2, &mt3))
	assert.False(t, mt1.Matches(&mt2, &mt4))
	assert.True(t, mt1.MatchesFromString("application/zip", "text/html;charset=utf-8"))
	assert.False(t, mt1.MatchesFromString("application/zip", "text/plain;charset=utf-8"))
}

func TestMediatypeIsZIP(t *testing.T) {
	mt, err := NewOfString("text/plain")
	assert.NoError(t, err)
	assert.False(t, mt.IsZIP())

	mt, err = NewOfString("application/zip")
	assert.NoError(t, err)
	assert.True(t, mt.IsZIP())

	mt, err = NewOfString("application/zip;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt.IsZIP())

	mt, err = NewOfString("application/epub+zip")
	assert.NoError(t, err)
	assert.True(t, mt.IsZIP(), "EPUBs are ZIPs")

	// These media types must be explicitly matched since they don't have any ZIP hint

	mt, err = NewOfString("application/audiobook+lcp")
	assert.NoError(t, err)
	assert.True(t, mt.IsZIP())

	mt, err = NewOfString("application/pdf+lcp")
	assert.NoError(t, err)
	assert.True(t, mt.IsZIP())
}

func TestMediatypeIsJSON(t *testing.T) {
	mt, err := NewOfString("text/plain")
	assert.NoError(t, err)
	assert.False(t, mt.IsJSON())

	mt, err = NewOfString("application/json")
	assert.NoError(t, err)
	assert.True(t, mt.IsJSON())

	mt, err = NewOfString("application/json;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt.IsJSON())

	mt, err = NewOfString("application/opds+json")
	assert.NoError(t, err)
	assert.True(t, mt.IsJSON())
}

func TestMediatypeIsOPDS(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt.IsOPDS())

	for _, r := range []string{
		"application/atom+xml;profile=opds-catalog",
		"application/atom+xml;type=entry;profile=opds-catalog",
		"application/opds+json",
		"application/opds-publication+json",
		"application/opds+json;charset=utf-8",
		"application/opds-authentication+json",
	} {
		mt, err = NewOfString(r)
		assert.NoError(t, err)
		assert.True(t, mt.IsOPDS(), r+" should be an OPDS document")
	}
}

func TestMediatypeIsHTML(t *testing.T) {
	mt, err := NewOfString("application/opds+json")
	assert.NoError(t, err)
	assert.False(t, mt.IsHTML())

	mt, err = NewOfString("text/html")
	assert.NoError(t, err)
	assert.True(t, mt.IsHTML())

	mt, err = NewOfString("application/xhtml+xml")
	assert.NoError(t, err)
	assert.True(t, mt.IsHTML())

	mt, err = NewOfString("text/html;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt.IsHTML())
}

func TestMediatypeIsBitmap(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt.IsBitmap())

	for _, r := range []string{
		"image/bmp",
		"image/gif",
		"image/jpeg",
		"image/png",
		"image/tiff",
		"image/tiff;charset=utf-8",
		"image/webp",
		"image/avif",
		"image/jxl",
	} {
		mt, err = NewOfString(r)
		assert.NoError(t, err)
		assert.True(t, mt.IsBitmap(), r+" should be a bitmap")
	}
}

func TestMediatypeIsAudio(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt.IsAudio())

	mt, err = NewOfString("audio/unknown")
	assert.NoError(t, err)
	assert.True(t, mt.IsAudio())

	mt, err = NewOfString("audio/mpeg;param=value")
	assert.NoError(t, err)
	assert.True(t, mt.IsAudio())
}

func TestMediatypeIsVideo(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt.IsVideo())

	mt, err = NewOfString("video/unknown")
	assert.NoError(t, err)
	assert.True(t, mt.IsVideo())

	mt, err = NewOfString("video/mpeg;param=value")
	assert.NoError(t, err)
	assert.True(t, mt.IsVideo())
}

func TestMediatypeIsRWPM(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt.IsRwpm())

	mt, err = NewOfString("application/audiobook+json")
	assert.NoError(t, err)
	assert.True(t, mt.IsRwpm())

	mt, err = NewOfString("application/divina+json")
	assert.NoError(t, err)
	assert.True(t, mt.IsRwpm())

	mt, err = NewOfString("application/webpub+json")
	assert.NoError(t, err)
	assert.True(t, mt.IsRwpm())

	mt, err = NewOfString("application/webpub+json;charset=utf-8")
	assert.NoError(t, err)
	assert.True(t, mt.IsRwpm())
}

func TestMediatypeIsPublication(t *testing.T) {
	mt, err := NewOfString("text/html")
	assert.NoError(t, err)
	assert.False(t, mt.IsPublication())

	for _, r := range []string{
		"application/audiobook+zip",
		"application/audiobook+json",
		"application/audiobook+lcp",
		"application/audiobook+json;charset=utf-8",
		"application/divina+zip",
		"application/divina+json",
		"application/webpub+zip",
		"application/webpub+json",
		"application/vnd.comicbook+zip",
		"application/epub+zip",
		"application/lpf+zip",
		"application/pdf",
		"application/pdf+lcp",
		"application/x.readium.w3c.wpub+json",
		"application/x.readium.zab+zip",
	} {
		mt, err = NewOfString(r)
		assert.NoError(t, err)
		assert.True(t, mt.IsPublication(), r+" should be a publication")
	}
}
