package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// One or more alternative expressions of the location.
// https://github.com/readium/architecture/tree/master/models/locators#the-location-object
type Locations struct {
	Fragments        []string               `json:"fragments,omitempty"`        // Contains one or more fragment in the resource referenced by the [Locator].
	Progression      float64                `json:"progression,omitempty"`      // Progression in the resource expressed as a percentage (between 0 and 1).
	Position         uint                   `json:"position,omitempty"`         // An index in the publication (>= 1).
	TotalProgression float64                `json:"totalProgression,omitempty"` // Progression in the publication expressed as a percentage (between 0 and 1).
	OtherLocations   map[string]interface{} `json:"otherLocations"`             // Additional locations for extensions.
}

func LocationsFromJSON(rawJson map[string]interface{}) (*Locations, error) {
	if rawJson == nil {
		return nil, nil
	}

	locations := &Locations{}

	// Fragments
	fragments, err := parseSliceOrString(rawJson["fragments"], false)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'fragments'")
	}
	if len(fragments) == 0 {
		fragments, err = parseSliceOrString(rawJson["fragment"], false)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing 'fragment'")
		}
	}
	locations.Fragments = fragments

	// Progression
	progression := parseOptFloat64(rawJson["progression"])
	if progression >= 0.0 && progression <= 1.0 {
		locations.Progression = progression
	}

	// Position
	position := parseOptFloat64(rawJson["position"])
	if position > 0 {
		locations.Position = float64ToUint(position)
	}

	// TotalProgression
	totalProgression := parseOptFloat64(rawJson["totalProgression"])
	if totalProgression >= 0.0 && totalProgression <= 1.0 {
		locations.TotalProgression = totalProgression
	}

	// Delete above vals so that we can put everything else in OtherLocations
	for _, v := range []string{
		"fragments", "fragment", "progression", "position", "totalProgression",
	} {
		delete(rawJson, v)
	}

	// Now all we have left is everything else!
	if len(rawJson) > 0 {
		locations.OtherLocations = rawJson
	}

	return locations, nil
}

func (l *Locations) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fl, err := LocationsFromJSON(object)
	if err != nil {
		return err
	}
	*l = *fl
	return nil
}

func (l Locations) MarshalJSON() ([]byte, error) {
	j := l.OtherLocations
	if j == nil {
		j = make(map[string]interface{})
	}

	if len(l.Fragments) > 0 {
		j["fragments"] = l.Fragments
	}
	if l.Progression != 0.0 {
		j["progression"] = l.Progression
	}
	if l.Position > 0 {
		j["position"] = l.Position
	}
	if l.TotalProgression > 0 {
		j["totalProgression"] = l.TotalProgression
	}

	return json.Marshal(j)
}

// Textual context of the locator.
// A Locator Text Object contains multiple text fragments, useful to give a context to the [Locator] or for highlights.
// https://github.com/readium/architecture/tree/master/models/locators#the-text-object
type Text struct {
	Before    string `json:"before,omitempty"`    // The text before the locator.
	Highlight string `json:"highlight,omitempty"` // The text at the locator.
	After     string `json:"after,omitempty"`     // The text after the locator.
}

func TextFromJSON(rawJson map[string]interface{}) *Text {
	if rawJson == nil {
		return nil
	}

	return &Text{
		Before:    parseOptString(rawJson["before"]),
		Highlight: parseOptString(rawJson["highlight"]),
		After:     parseOptString(rawJson["after"]),
	}
}

// Locator provides a precise location in a publication in a format that can be stored and shared.
//
// There are many different use cases for locators:
//  - getting back to the last position in a publication
//  - bookmarks
//  - highlights & annotations
//  - search results
//  - human-readable (and shareable) reference in a publication
//
// https://github.com/readium/architecture/tree/master/locators
type Locator struct {
	Href      string     `json:"href"`
	Type      string     `json:"type"`
	Title     string     `json:"title,omitempty"`
	Locations *Locations `json:"locations,omitempty"`
	Text      *Text      `json:"text,omitempty"`
}

func LocatorFromJSON(rawJson map[string]interface{}) (*Locator, error) {
	if rawJson == nil {
		return nil, nil
	}

	locator := &Locator{
		Href:  parseOptString(rawJson["href"]),
		Type:  parseOptString(rawJson["type"]),
		Title: parseOptString(rawJson["title"]),
	}
	if locator.Href == "" || locator.Type == "" {
		return nil, errors.New("'href' and 'type' are required")
	}

	if rawLocations, ok := rawJson["locations"].(map[string]interface{}); ok {
		locations, err := LocationsFromJSON(rawLocations)
		if err != nil {
			return nil, err
		}
		locator.Locations = locations
	}

	if rawText, ok := rawJson["text"].(map[string]interface{}); ok {
		locator.Text = TextFromJSON(rawText)
	}

	return locator, nil
}

func (l *Locator) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fl, err := LocatorFromJSON(object)
	if err != nil {
		return err
	}
	*l = *fl
	return nil
}
