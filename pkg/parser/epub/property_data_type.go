package epub

import (
	"net/url"
	"regexp"
	"strings"
)

var PackageReservedPrefixes = map[string]string{
	"dcterms":   VocabularyDCTerms,
	"media":     VocabularyMedia,
	"rendition": VocabularyRendition,
	"a11y":      VocabularyA11Y,
	"marc":      VocabularyMARC,
	"onix":      VocabularyONIX,
	"schema":    VocabularySchema,
	"xsd":       VocabularyXSD,
	"tdm":       VocabularyTDM,
}

var ContentReservedPrefixes = map[string]string{
	"msv":   VocabularyMSV,
	"prism": VocabularyPRISM,
}

type DefaultVocab int

const (
	DefaultVocabMeta = iota
	DefaultVocabLink
	DefaultVocabItem
	DefaultVocabItemref
	DefaultVocabType
)

var DefaultVocabMap = map[DefaultVocab]string{
	DefaultVocabMeta:    VocabularyMeta,
	DefaultVocabLink:    VocabularyLink,
	DefaultVocabItem:    VocabularyItem,
	DefaultVocabItemref: VocabularyItemref,
	DefaultVocabType:    VocabularyType,
}

func resolveProperty(property string, prefixMap map[string]string, defaultVocab DefaultVocab) string {
	st := strings.SplitN(property, ":", 2)
	s := make([]string, 0, len(st))
	for _, v := range st {
		if v != "" {
			s = append(s, v)
		}
	}
	if len(s) == 1 {
		name := url.PathEscape(s[0])
		return DefaultVocabMap[defaultVocab] + name
	} else {
		pmm, ok := prefixMap[s[0]]
		if ok && len(s) == 2 {
			lc := pmm[len(pmm)-1]
			if lc != '#' && lc != '/' { // Namespace URI doesn't end with '/' or '#'
				pmm += "#"
			}
			name := url.PathEscape(s[1])
			return pmm + name
		} else {
			return property
		}
	}
}

var prefixMatcher = regexp.MustCompile(`\s*(\w+):\s*(\S+)`)

func parsePrefixes(prefixes string) map[string]string {
	p := make(map[string]string)
	matches := prefixMatcher.FindAllStringSubmatch(prefixes, -1)
	for _, match := range matches {
		p[match[1]] = match[2]
	}
	return p
}

func parseProperties(raw string) []string {
	return strings.Fields(raw)
}

// collapseWhitespace trims leading/trailing ASCII whitespace and replaces every
// internal run of it with a single space. It reproduces the previous
// regexp.ReplaceAllString(`\s+`, " ") + TrimSpace behavior — deliberately ASCII
// only (the same set RE2's \s matches), so non-ASCII spaces such as the U+3000
// ideographic space common in CJK titles are preserved. Scanning byte-wise is
// safe because UTF-8 continuation bytes are always >= 0x80 and never collide
// with ASCII whitespace.
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\f', '\r':
			if b.Len() > 0 {
				pendingSpace = true
			}
		default:
			if pendingSpace {
				b.WriteByte(' ')
				pendingSpace = false
			}
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
