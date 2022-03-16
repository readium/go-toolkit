package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalizedStringUnmarshalJSONString(t *testing.T) {
	var l LocalizedString
	assert.NoError(t, json.Unmarshal([]byte("\"a string\""), &l))
	assert.Equal(t, NewLocalizedStringFromString("a string"), l, "parsed JSON string should be equal to string")
}

func TestLocalizedStringUnmarshalJSONLocalizedStrings(t *testing.T) {
	var l1 LocalizedString
	var l2 LocalizedString
	assert.NoError(t, json.Unmarshal([]byte(`{
		"en": "a string",
		"fr": "une chaîne"
	}`), &l1))
	l2.SetTranslation("en", "a string")
	l2.SetTranslation("fr", "une chaîne")
	assert.Equal(t, l2, l1, "parsed JSON object should be equal to manually created LocalizedString")
}

func TestLocalizedStringUnmarshalInvalidJSON(t *testing.T) {
	var l LocalizedString
	assert.Error(t, json.Unmarshal([]byte(`[1,2]`), &l), "parsing should fail")
}

func TestLocalizedStringUnmarshalNullJSON(t *testing.T) {
	var l LocalizedString
	assert.Error(t, json.Unmarshal(nil, &l), "parsing should fail")
}

func TestLocalizedStringOneTranslationNoLanguage(t *testing.T) {
	l := NewLocalizedStringFromString("a string")
	s, err := json.Marshal(&l)
	assert.NoError(t, err)
	assert.JSONEq(t, `"a string"`, string(s), "JSON of LocalizedString with default language should equal a JSON string of the value")
}

func TestLocalizedStringJSON(t *testing.T) {
	var l LocalizedString
	l.SetTranslation("en", "a string")
	l.SetTranslation("fr", "une chaîne")
	l.SetTranslation(UndefinedLanguage, "Surgh")
	s, err := json.Marshal(&l)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"en": "a string",
		"fr": "une chaîne",
		"und": "Surgh"
	}`, string(s), "JSON objects should be equal")
}

func TestLocalizedStringDefaultTranslation(t *testing.T) {
	var l LocalizedString
	l.SetTranslation("en", "a string")
	l.SetTranslation("fr", "une chaîne")
	assert.Equal(t, "a string", l.DefaultTranslation(), "default translation should be equal to \"a string\"")
}

func TestLocalizedStringFindTranslationByLanguage(t *testing.T) {
	var l LocalizedString
	l.SetTranslation("en", "a string")
	l.SetTranslation("fr", "une chaîne")
	assert.Equal(t, "une chaîne", l.GetOrFallback("fr"), "should be able to find the correct fr translation")
}

func TestLocalizedStringFindTranslationByLanguageDefaultsUndefined(t *testing.T) {
	var l LocalizedString
	l.SetTranslation("foo", "a string")
	l.SetTranslation("bar", "une chaîne")
	l.SetTranslation(UndefinedLanguage, "Surgh")
	assert.Equal(t, "Surgh", l.DefaultTranslation())
}

func TestLocalizedStringFindTranslationByLanguageDefaultsFirstFound(t *testing.T) {
	var l LocalizedString
	l.SetTranslation("fr", "une chaîne")
	assert.Equal(t, "une chaîne", l.DefaultTranslation())
}

func TestLocalizedStringFinfTranslationDefaultsForEmpty(t *testing.T) {
	var l LocalizedString
	assert.Equal(t, l.DefaultTranslation(), "")
}
