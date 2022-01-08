package epub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyDataTypeParseSinglePrefix(t *testing.T) {
	prefixes := parsePrefixes("foaf: http://xmlns.com/foaf/spec/")
	assert.Len(t, prefixes, 1)
	if assert.Contains(t, prefixes, "foaf") {
		assert.Equal(t, "http://xmlns.com/foaf/spec/", prefixes["foaf"])
	}
}

func TestPropertyDataTypeMultiplePrefixes(t *testing.T) {
	prefixes := parsePrefixes("foaf: http://xmlns.com/foaf/spec/ dbp: http://dbpedia.org/ontology/")
	assert.Len(t, prefixes, 2)
	if assert.Contains(t, prefixes, "foaf") {
		assert.Equal(t, "http://xmlns.com/foaf/spec/", prefixes["foaf"])
	}
	if assert.Contains(t, prefixes, "dbp") {
		assert.Equal(t, "http://dbpedia.org/ontology/", prefixes["dbp"])
	}
}

func TestPropertyDataTypeSpaceBetweenPrefixAndIrisOmittable(t *testing.T) {
	prefixes := parsePrefixes("foaf: http://xmlns.com/foaf/spec/ dbp:http://dbpedia.org/ontology/")
	assert.Len(t, prefixes, 2)
	if assert.Contains(t, prefixes, "foaf") {
		assert.Equal(t, "http://xmlns.com/foaf/spec/", prefixes["foaf"])
	}
	if assert.Contains(t, prefixes, "dbp") {
		assert.Equal(t, "http://dbpedia.org/ontology/", prefixes["dbp"])
	}
}

func TestPropertyDataTypePrefixesSeparatableByLines(t *testing.T) {
	prefixes := parsePrefixes(`foaf: http://xmlns.com/foaf/spec/
	dbp: http://dbpedia.org/ontology/`)
	assert.Len(t, prefixes, 2)
	if assert.Contains(t, prefixes, "foaf") {
		assert.Equal(t, "http://xmlns.com/foaf/spec/", prefixes["foaf"])
	}
	if assert.Contains(t, prefixes, "dbp") {
		assert.Equal(t, "http://dbpedia.org/ontology/", prefixes["dbp"])
	}
}

func TestPropertyDataParsePrefixesEmpty(t *testing.T) {
	assert.Empty(t, parsePrefixes(""))
}

func TestPropertyDataResolvePropertyDefaultVocabularies(t *testing.T) {
	assert.Equal(
		t,
		"http://idpf.org/epub/vocab/package/item/#nav",
		resolveProperty("nav", PackageReservedPrefixes, DefaultVocabItem),
	)
}

func TestPropertyDataResolvePropertyPrefixMapPriority(t *testing.T) {
	assert.Equal(
		t,
		"http://www.idpf.org/epub/vocab/overlays/#narrator",
		resolveProperty("media:narrator", PackageReservedPrefixes, DefaultVocabMeta),
	)
}

func TestPropertyDataParsePropertiesWhitespace(t *testing.T) {
	properties := `
	rendition:flow-auto        rendition:layout-pre-paginated             
		 rendition:orientation-auto
	`
	assert.Equal(
		t,
		parseProperties(properties),
		[]string{
			"rendition:flow-auto",
			"rendition:layout-pre-paginated",
			"rendition:orientation-auto",
		},
	)
}

func TestPropertyDataParsePropertiesEmpty(t *testing.T) {
	assert.Empty(t, parseProperties(""))
}
