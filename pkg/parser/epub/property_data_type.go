package epub

import (
	"regexp"
	"strings"
)

var PACKAGE_RESERVED_PREFIXES = map[string]string{
	"dcterms":   VocabularyDCTerms,
	"media":     VocabularyMedia,
	"rendition": VocabularyRendition,
	"a11y":      VocabularyA11Y,
	"marc":      VocabularyMARC,
	"onix":      VocabularyONIX,
	"schema":    VocabularySchema,
	"xsd":       VocabularyXSD,
}

var CONTENT_RESERVED_PREFIXES = map[string]string{
	"msv":   VocabularyMSV,
	"prism": VocabularyPRISM,
}

type DefaultVocab int

const (
	NoVocab DefaultVocab = iota
	VocabMeta
	VocabLink
	VocabItem
	VocabItemref
	VocabType
)

var DEFAULT_VOCAB = map[DefaultVocab]string{
	VocabMeta:    VocabularyMeta,
	VocabLink:    VocabularyLink,
	VocabItem:    VocabularyItem,
	VocabItemref: VocabularyItemref,
	VocabType:    VocabularyType,
}

func resolveProperty(property string, prefixMap map[string]string, defaultVocab DefaultVocab) string {
	st := strings.SplitN(property, ":", 2)
	s := []string{}
	for _, v := range st {
		if v != "" {
			s = append(s, v)
		}
	}
	if len(s) == 1 && defaultVocab != 0 {
		return DEFAULT_VOCAB[defaultVocab] + s[0]
	} else {
		pmm, ok := prefixMap[s[0]]
		if ok && len(s) == 2 {
			return pmm + s[1]
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

var muchSpaceSuchWowMatcher = regexp.MustCompile(`\s+`)

func parseProperties(raw string) []string {
	vals := muchSpaceSuchWowMatcher.Split(raw, -1)
	s := []string{}
	for _, v := range vals {
		if v != "" {
			s = append(s, v)
		}
	}
	return s
}
