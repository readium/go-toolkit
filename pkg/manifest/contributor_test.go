package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContributorUnmarshalJSONString(t *testing.T) {
	c1 := Contributor{
		LocalizedName: NewLocalizedStringFromString("John Smith"),
	}
	var c2 Contributor
	assert.NoError(t, json.Unmarshal([]byte(`"John Smith"`), &c2))
	assert.Equal(t, c1, c2, "unmarshalled JSON string should be equal to string")
}

func TestContributorUnmarshalMinimalJSON(t *testing.T) {
	c1 := Contributor{
		LocalizedName: NewLocalizedStringFromString("John Smith"),
	}
	var c2 Contributor
	assert.NoError(t, json.Unmarshal([]byte(`{"name": "John Smith"}`), &c2))
	assert.Equal(t, c1, c2, "unmarshalled JSON object should be equal to Contributor object")
}

func TestContributorUnmarshalFullJSON(t *testing.T) {
	sortAs := NewLocalizedStringFromString("greenwood")
	position := float64(4.0)
	c1 := Contributor{
		LocalizedName:   NewLocalizedStringFromString("Colin Greenwood"),
		LocalizedSortAs: &sortAs,
		Identifier:      "colin",
		Roles:           []string{"bassist"},
		Position:        &position,
		Links: []Link{
			{
				Href: MustNewHREFFromString("http://link1", false),
			},
			{
				Href: MustNewHREFFromString("http://link2", false),
			},
		},
	}
	var c2 Contributor
	assert.NoError(t, json.Unmarshal([]byte(`{
		"name": "Colin Greenwood",
		"identifier": "colin",
		"sortAs": "greenwood",
		"role": "bassist",
		"position": 4,
		"links": [
			{"href": "http://link1"},
			{"href": "http://link2"}
		]
	}`), &c2))
	assert.Equal(t, c1, c2, "unmarshalled JSON object should be equal to Contributor object")
}

func TestContributorUnmarshalJSONWithDuplicateRoles(t *testing.T) {
	c1 := Contributor{
		LocalizedName: NewLocalizedStringFromString("Thom Yorke"),
		Roles:         []string{"singer", "guitarist"},
	}
	var c2 Contributor
	assert.NoError(t, json.Unmarshal([]byte(`{
		"name": "Thom Yorke",
		"role": ["singer", "guitarist", "guitarist"]
	}`), &c2))
	assert.Equal(t, c1, c2, "unmarshalled JSON object should be equal to Contributor object")
}

func TestContributorUnmarshalRequiresName(t *testing.T) {
	var c Contributor
	assert.Error(t, json.Unmarshal([]byte(`{"identifier": "loremipsonium"}`), &c), "Contributor is invalid because it has no name")
	/*
		assert.NoError(t, json.Unmarshal([]byte(`{"identifier": "loremipsonium"}`), &c))
		assert.Equal(t, c, Contributor{}, "unmarshalled JSON object should be empty Contributor")
	*/
}

func TestContributorNameFromDefaultTranslation(t *testing.T) {
	c := Contributor{
		LocalizedName: LocalizedString{
			Translations: map[string]string{
				"en": "Jonny Greenwood",
				"fr": "Jean Boisvert",
			},
		},
	}
	assert.Equal(t, c.Name(), "Jonny Greenwood", "Contributor's name should be equal to \"Jonny Greenwood\"")
}

func TestContributorMinimalJSON(t *testing.T) {
	l := Contributor{LocalizedName: NewLocalizedStringFromString("Colin Greenwood")}
	s, err := json.Marshal(l)
	assert.NoError(t, err)
	assert.JSONEq(t, `"Colin Greenwood"`, string(s), "JSON of Contributor with one name should be equal to JSON representation")
}

func TestContributorMinimalJSONWithLocalizedName(t *testing.T) {
	n := LocalizedString{}
	n.SetTranslation("en", "Colin Greenwood")
	n.SetTranslation("fr", "Colain Grinwoud")
	l := Contributor{LocalizedName: n}
	s, err := json.Marshal(l)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"name": {"fr": "Colain Grinwoud", "en": "Colin Greenwood"}}`, string(s), "JSON of Contributor with one name should be equal to JSON representation")
}

func TestContributorFullJSON(t *testing.T) {
	pos := float64(4.0)
	sortAs := NewLocalizedStringFromString("greenwood")
	l := Contributor{
		LocalizedName:   NewLocalizedStringFromString("Colin Greenwood"),
		LocalizedSortAs: &sortAs,
		Identifier:      "colin",
		Roles:           []string{"bassist"},
		Position:        &pos,
		Links: []Link{
			{
				Href: MustNewHREFFromString("http://link1", true),
			},
			{
				Href: MustNewHREFFromString("http://link2", false),
			},
		},
	}
	s, err := json.Marshal(l)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"name": "Colin Greenwood",
		"identifier": "colin",
		"sortAs": "greenwood",
		"role": "bassist",
		"position": 4.0,
		"links": [
			{"href": "http://link1", "templated": true},
			{"href": "http://link2"}
		]
	}`, string(s), "JSON of Contributor with all fields filled should be equal to JSON representation")
}
