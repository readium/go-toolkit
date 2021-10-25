package epub

import (
	"regexp"
	"strings"
)

var PACKAGE_RESERVED_PREFIXES = map[string]string{
	"dcterms":   VOCABULARY_DCTERMS,
	"media":     VOCABULARY_MEDIA,
	"rendition": VOCABULARY_RENDITION,
	"a11y":      VOCABULARY_A11Y,
	"marc":      VOCABULARY_MARC,
	"onix":      VOCABULARY_ONIX,
	"schema":    VOCABULARY_SCHEMA,
	"xsd":       VOCABULARY_XSD,
}

var CONTENT_RESERVED_PREFIXES = map[string]string{
	"msv":   VOCABULARY_MSV,
	"prism": VOCABULARY_PRISM,
}

type DefaultVocab int

const (
	META DefaultVocab = iota + 1
	LINK
	ITEM
	ITEMREF
	TYPE
)

var DEFAULT_VOCAB = map[DefaultVocab]string{
	META:    VOCABULARY_META,
	LINK:    VOCABULARY_LINK,
	ITEM:    VOCABULARY_ITEM,
	ITEMREF: VOCABULARY_ITEMREF,
	TYPE:    VOCABULARY_TYPE,
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
