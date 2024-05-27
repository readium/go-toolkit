package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// One or more alternative expressions of the location.
// https://github.com/readium/architecture/tree/master/models/locators#the-location-object
type Locations struct {
	Fragments        []string               `json:"fragments,omitempty"`        // Contains one or more fragment in the resource referenced by the [Locator].
	Progression      *float64               `json:"progression,omitempty"`      // Progression in the resource expressed as a percentage (between 0 and 1).
	Position         *uint                  `json:"position,omitempty"`         // An index in the publication (>= 1).
	TotalProgression *float64               `json:"totalProgression,omitempty"` // Progression in the publication expressed as a percentage (between 0 and 1).
	OtherLocations   map[string]interface{} // Additional locations for extensions.
}

func LocationsFromJSON(rawJson map[string]interface{}) (l Locations, err error) {
	if rawJson == nil {
		return
	}

	// Fragments
	fragments, err := parseSliceOrString(rawJson["fragments"], false)
	if err != nil {
		err = errors.Wrap(err, "failed parsing 'fragments'")
		return
	}
	if len(fragments) == 0 {
		fragments, err = parseSliceOrString(rawJson["fragment"], false)
		if err != nil {
			err = errors.Wrap(err, "failed parsing 'fragment'")
			return
		}
	}
	l.Fragments = fragments

	// Progression
	rawProgression, ok := rawJson["progression"]
	if ok {
		progression := parseOptFloat64(rawProgression)
		if progression >= 0.0 && progression <= 1.0 {
			l.Progression = &progression
		}
	}

	// Position
	rawPositions, ok := rawJson["position"]
	if ok {
		position := float64ToUint(parseOptFloat64(rawPositions))
		if position > 0 {
			l.Position = &position
		}
	}

	// TotalProgression
	rawTotalProgress, ok := rawJson["totalProgression"]
	if ok {
		totalProgression := parseOptFloat64(rawTotalProgress)
		if totalProgression >= 0.0 && totalProgression <= 1.0 {
			l.TotalProgression = &totalProgression
		}
	}

	// Delete above vals so that we can put everything else in OtherLocations
	for _, v := range []string{
		"fragments", "fragment", "progression", "position", "totalProgression",
	} {
		delete(rawJson, v)
	}

	// Now all we have left is everything else!
	if len(rawJson) > 0 {
		l.OtherLocations = rawJson
	}

	return l, nil
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
	*l = fl
	return nil
}

func (l Locations) MarshalJSON() ([]byte, error) {
	j := make(map[string]interface{})
	if l.OtherLocations != nil {
		for k, v := range l.OtherLocations {
			j[k] = v
		}
	}

	if len(l.Fragments) > 0 {
		j["fragments"] = l.Fragments
	}
	if l.Progression != nil {
		j["progression"] = *l.Progression
	}
	if l.Position != nil {
		j["position"] = *l.Position
	}
	if l.TotalProgression != nil {
		j["totalProgression"] = *l.TotalProgression
	}

	return json.Marshal(j)
}

// HTML extensions for [Locations]

func (l Locations) CSSSelector() string {
	if v, ok := l.OtherLocations["cssSelector"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// TODO partialCfi and domRange getters

// Textual context of the locator.
// A Locator Text Object contains multiple text fragments, useful to give a context to the [Locator] or for highlights.
// https://github.com/readium/architecture/tree/master/models/locators#the-text-object
type Text struct {
	Before    string `json:"before,omitempty"`    // The text before the locator.
	Highlight string `json:"highlight,omitempty"` // The text at the locator.
	After     string `json:"after,omitempty"`     // The text after the locator.
}

func TextFromJSON(rawJson map[string]interface{}) (t Text) {
	if rawJson == nil {
		return
	}

	t.Before = parseOptString(rawJson["before"])
	t.Highlight = parseOptString(rawJson["highlight"])
	t.After = parseOptString(rawJson["after"])
	return
}

// Locator provides a precise location in a publication in a format that can be stored and shared.
//
// There are many different use cases for locators:
//   - getting back to the last position in a publication
//   - bookmarks
//   - highlights & annotations
//   - search results
//   - human-readable (and shareable) reference in a publication
//
// https://github.com/readium/architecture/tree/master/locators
type Locator struct {
	Href      string    `json:"href"`
	Type      string    `json:"type"`
	Title     string    `json:"title,omitempty"`
	Locations Locations `json:"locations,omitempty"`
	Text      Text      `json:"text,omitempty"`
}

func LocatorFromJSON(rawJson map[string]interface{}) (Locator, error) {
	if rawJson == nil {
		return Locator{}, nil
	}

	locator := Locator{
		Href:  parseOptString(rawJson["href"]),
		Type:  parseOptString(rawJson["type"]),
		Title: parseOptString(rawJson["title"]),
	}
	if locator.Href == "" || locator.Type == "" {
		return Locator{}, errors.New("'href' and 'type' are required")
	}

	if rawLocations, ok := rawJson["locations"].(map[string]interface{}); ok {
		locations, err := LocationsFromJSON(rawLocations)
		if err != nil {
			return Locator{}, err
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
	*l = fl
	return nil
}

func (l Locator) MarshalJSON() ([]byte, error) {
	j := make(map[string]interface{})
	j["href"] = l.Href
	j["type"] = l.Type
	if l.Title != "" {
		j["title"] = l.Title
	}

	ll := l.Locations
	if len(ll.Fragments) > 0 || len(ll.OtherLocations) > 1 || ll.Position != nil || ll.Progression != nil || ll.TotalProgression != nil {
		j["locations"] = ll
	}

	if l.Text.After != "" || l.Text.Before != "" || l.Text.Highlight != "" {
		j["text"] = l.Text
	}

	return json.Marshal(j)
}
