package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubjectUnmarshalJSONString(t *testing.T) {
	s, err := SubjectFromJSON("Fantasy")
	assert.NoError(t, err)

	assert.Equal(t, &Subject{
		LocalizedName: NewLocalizedStringFromString("Fantasy"),
	}, s, "parsed JSON string should be equal to string")
}

func TestSubjectUnmarshalMinimalJSON(t *testing.T) {
	var s Subject
	assert.NoError(t, json.Unmarshal([]byte(`{"name":"Science Fiction"}`), &s))

	assert.Equal(t, &Subject{
		LocalizedName: NewLocalizedStringFromString("Science Fiction"),
	}, &s, "parsed JSON object should be equal to Subject object")
}

func TestSubjectUnmarshalFullJSON(t *testing.T) {
	var s Subject
	assert.NoError(t, json.Unmarshal([]byte(`{
		"name": "Science Fiction",
		"sortAs": "science-fiction",
		"scheme": "http://scheme",
		"code": "CODE",
		"links": [
			{"href": "pub1"},
			{"href": "pub2"}
		]
	}`), &s))

	lsa := NewLocalizedStringFromString("science-fiction")
	assert.Equal(t, &Subject{
		LocalizedName:   NewLocalizedStringFromString("Science Fiction"),
		LocalizedSortAs: &lsa,
		Scheme:          "http://scheme",
		Code:            "CODE",
		Links: []Link{
			{Href: MustNewHREFFromString("pub1", false)},
			{Href: MustNewHREFFromString("pub2", false)},
		},
	}, &s, "parsed JSON object should be equal to Subject object")
}

func TestSubjectUnmarshalNilJSON(t *testing.T) {
	s, err := SubjectFromJSON(nil)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestSubjectUnmarshalRequiresName(t *testing.T) {
	var s Subject
	assert.Error(t, json.Unmarshal([]byte(`{"sortAs": "science-fiction"}`), &s), "name is required for Subject objects")
}

func TestSubjectUnmarshalJSONArray(t *testing.T) {
	var ss []Subject
	assert.NoError(t, json.Unmarshal([]byte(`[
		"Fantasy",
		{
			"name": "Science Fiction",
			"scheme": "http://scheme"
		}
	]`), &ss))

	assert.Equal(t, []Subject{
		{LocalizedName: NewLocalizedStringFromString("Fantasy")},
		{
			LocalizedName: NewLocalizedStringFromString("Science Fiction"),
			Scheme:        "http://scheme",
		},
	}, ss, "parsed JSON array should be equal to Subject slice")
}

func TestSubjectUnmarshalNilJSONArray(t *testing.T) {
	ss, err := SubjectFromJSONArray(nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ss))
}

func TestSubjectUnmarshalJSONArrayString(t *testing.T) {
	ss, err := SubjectFromJSONArray("Fantasy")
	assert.NoError(t, err)
	assert.Equal(t, []Subject{
		{LocalizedName: NewLocalizedStringFromString("Fantasy")},
	}, ss, "parsed JSON object should be equal to Subject object")
}

// func TestSubjectUnmarshalJSONArraySingle(t *testing.T)

func TestSubjectNameFromDefaultTranslation(t *testing.T) {
	assert.Equal(t, "Hello world", Subject{
		LocalizedName: NewLocalizedStringFromStrings(map[string]string{
			"en": "Hello world",
			"fr": "Salut le monde",
		}),
	}.Name(), "'Hello World' should be the default translation of the Subject")
}

func TestSubjectMinimalJSON(t *testing.T) {
	bin, err := json.Marshal(Subject{
		LocalizedName: NewLocalizedStringFromString("Science Fiction"),
	})
	assert.NoError(t, err)
	assert.JSONEq(t, string(bin), `"Science Fiction"`)
}

func TestSubjectFullJSON(t *testing.T) {
	lsa := NewLocalizedStringFromString("science-fiction")
	bin, err := json.Marshal(Subject{
		LocalizedName:   NewLocalizedStringFromString("Science Fiction"),
		LocalizedSortAs: &lsa,
		Scheme:          "http://scheme",
		Code:            "CODE",
		Links: []Link{
			{Href: MustNewHREFFromString("pub1", false)},
			{Href: MustNewHREFFromString("pub2", false)},
		},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, string(bin), `{
		"name": "Science Fiction",
		"sortAs": "science-fiction",
		"scheme": "http://scheme",
		"code": "CODE",
		"links": [
			{"href": "pub1"},
			{"href": "pub2"}
		]
	}`)
}

func TestSubjectJSONArray(t *testing.T) {
	bin, err := json.Marshal([]Subject{{
		LocalizedName: NewLocalizedStringFromString("Fantasy"),
	}, {
		LocalizedName: NewLocalizedStringFromString("Science Fiction"),
		Scheme:        "http://scheme",
	}})
	assert.NoError(t, err)
	assert.JSONEq(t, string(bin), `[
		"Fantasy",
		{
			"name": "Science Fiction",
			"scheme": "http://scheme"
		}
	]`)
}
