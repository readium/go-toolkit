package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocatorUnmarshalMinimalJSON(t *testing.T) {
	var l Locator
	assert.NoError(t, json.Unmarshal([]byte(`{
		"href": "http://locator",
		"type": "text/html"
	}`), &l))
	assert.Equal(t, Locator{
		Href: "http://locator",
		Type: "text/html",
	}, l)
}

func TestLocatorUnmarshalJSON(t *testing.T) {
	var l Locator
	assert.NoError(t, json.Unmarshal([]byte(`{
		"href": "http://locator",
		"type": "text/html",
		"title": "My Locator",
		"locations": {
			"position": 42
		},
		"text": {
			"highlight": "Excerpt"
		}
	}`), &l))
	assert.Equal(t, Locator{
		Href:      "http://locator",
		Type:      "text/html",
		Title:     "My Locator",
		Locations: &Locations{Position: 42},
		Text:      &Text{Highlight: "Excerpt"},
	}, l)
}

func TestLocatorUnmarshalInvalidJSON(t *testing.T) {
	var l Locator
	assert.Error(t, json.Unmarshal([]byte(`{"invalid": "object"}`), &l), "parsing should fail")
}

func TestLocatorMinimalJSON(t *testing.T) {
	s, err := json.Marshal(&Locator{
		Href: "http://locator",
		Type: "text/html",
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"href": "http://locator",
		"type": "text/html"
	}`, string(s), "JSON objects should be equal")
}

func TestLocatorJSON(t *testing.T) {
	s, err := json.Marshal(&Locator{
		Href:  "http://locator",
		Type:  "text/html",
		Title: "My Locator",
		Locations: &Locations{
			Position: 42,
		},
		Text: &Text{
			Highlight: "Excerpt",
		},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"href": "http://locator",
		"type": "text/html",
		"title": "My Locator",
		"locations": {
			"position": 42
		},
		"text": {
			"highlight": "Excerpt"
		}
	}`, string(s), "JSON objects should be equal")
}

func TestLocationsUnmarshalMinimalJSON(t *testing.T) {
	var l Locations
	assert.NoError(t, json.Unmarshal([]byte(`{}`), &l))
	assert.Equal(t, Locations{}, l)
}

func TestLocationsUnmarshalJSON(t *testing.T) {
	var l Locations
	assert.NoError(t, json.Unmarshal([]byte(`{
		"fragments": ["p=4", "frag34"],
		"progression": 0.74,
		"totalProgression": 0.32,
		"position": 42,
		"other": "other-location"
	}`), &l))
	assert.Equal(t, Locations{
		Fragments:        []string{"p=4", "frag34"},
		Progression:      0.74,
		TotalProgression: 0.32,
		Position:         42,
		OtherLocations: map[string]interface{}{
			"other": "other-location",
		},
	}, l)
}

func TestLocationsUnmarshalSingleFragmentJSON(t *testing.T) {
	var l Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"fragment": "frag34"}`), &l))
	assert.Equal(t, Locations{
		Fragments: []string{"frag34"},
	}, l)
}

func TestLocationsUnmarshalIgnoresNegativePosition(t *testing.T) {
	var l1 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"position": 1}`), &l1))
	assert.Equal(t, Locations{Position: 1}, l1)

	var l2 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"position": 0}`), &l2))
	assert.Equal(t, Locations{}, l2)

	var l3 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"position": -1}`), &l3))
	assert.Equal(t, Locations{}, l3)
}

func TestLocationsUnmarshalIgnoresProgressionOutOfRange(t *testing.T) {
	var l1 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"progression": 0.5}`), &l1))
	assert.Equal(t, Locations{Progression: 0.5}, l1)

	var l2 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"progression": 0}`), &l2))
	assert.Equal(t, Locations{Progression: 0.0}, l2)

	var l3 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"progression": 1}`), &l3))
	assert.Equal(t, Locations{Progression: 1.0}, l3)

	var l4 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"progression": -0.5}`), &l4))
	assert.Equal(t, Locations{}, l4)

	var l5 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"progression": 1.2}`), &l5))
	assert.Equal(t, Locations{}, l5)
}

func TestLocationsUnmarshalIgnoresTotalProgressionOutOfRange(t *testing.T) {
	var l1 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"totalProgression": 0.5}`), &l1))
	assert.Equal(t, Locations{TotalProgression: 0.5}, l1)

	var l2 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"totalProgression": 0}`), &l2))
	assert.Equal(t, Locations{TotalProgression: 0.0}, l2)

	var l3 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"totalProgression": 1}`), &l3))
	assert.Equal(t, Locations{TotalProgression: 1.0}, l3)

	var l4 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"totalProgression": -0.5}`), &l4))
	assert.Equal(t, Locations{}, l4)

	var l5 Locations
	assert.NoError(t, json.Unmarshal([]byte(`{"totalProgression": 1.2}`), &l5))
	assert.Equal(t, Locations{}, l5)
}

func TestLocationsMinimalJSON(t *testing.T) {
	s, err := json.Marshal(Locator{})
	assert.NoError(t, err)
	// Note: href and type are not omitted because they are required!
	assert.JSONEq(t, `{"href":"", "type":""}`, string(s), "JSON objects should be equal")
}

func TestLocationsJSON(t *testing.T) {
	s, err := json.Marshal(&Locations{
		Fragments:        []string{"p=4", "frag34"},
		Progression:      0.74,
		Position:         42,
		TotalProgression: 25.32,
		OtherLocations: map[string]interface{}{
			"other": "other-location",
		},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"fragments": ["p=4", "frag34"],
		"progression": 0.74,
		"totalProgression": 25.32,
		"position": 42,
		"other": "other-location"
	}`, string(s), "JSON objects should be equal")
}

func TestTextUnmarshalMinimalJSON(t *testing.T) {
	var tx Text
	assert.NoError(t, json.Unmarshal([]byte(`{}`), &tx))
	assert.Equal(t, Text{}, tx)
}

func TestTextUnmarshalJSON(t *testing.T) {
	var tx Text
	assert.NoError(t, json.Unmarshal([]byte(`{
		"before": "Text before",
		"highlight": "Highlighted text",
		"after": "Text after"
	}`), &tx))
	assert.Equal(t, Text{
		Before:    "Text before",
		Highlight: "Highlighted text",
		After:     "Text after",
	}, tx)
}

func TestTextMinimalJSON(t *testing.T) {
	s, err := json.Marshal(Text{})
	assert.NoError(t, err)
	assert.JSONEq(t, `{}`, string(s), "JSON objects should be equal")
}

func TestTextJSON(t *testing.T) {
	s, err := json.Marshal(Text{
		Before:    "Text before",
		Highlight: "Highlighted text",
		After:     "Text after",
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"before": "Text before",
		"highlight": "Highlighted text",
		"after": "Text after"
	}`, string(s), "JSON objects should be equal")
}
