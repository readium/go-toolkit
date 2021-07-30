package pub

import (
	"encoding/json"
	"errors"
)

// Contributor
// https://github.com/readium/webpub-manifest/tree/master/contexts/default#contributors
// https://readium.org/webpub-manifest/schema/contributor-object.schema.json
type Contributor struct {
	LocalizedName   LocalizedString  `json:"name" validate:"required"` // The name of the contributor.
	LocalizedSortAs *LocalizedString `json:"sortAs,omitempty"`         // The string used to sort the name of the contributor.
	Identifier      string           `json:"identifier,omitempty"`     // An unambiguous reference to this contributor.
	Roles           string           `json:"role,omitempty"`           // The roles of the contributor in the publication making.
	Position        *float64         `json:"position,omitempty"`       // The position of the publication in this collection/series, when the contributor represents a collection. TODO validator
	Links           []Link           `json:"links,omitempty"`          // Used to retrieve similar publications for the given contributor.
}

func (c Contributor) Name() string {
	return c.LocalizedName.String()
}

func (c Contributor) SortAs() string {
	if c.LocalizedSortAs == nil {
		return ""
	}
	return c.LocalizedSortAs.String()
}

func (c Contributor) MarshalJSON() ([]byte, error) {
	if c.LocalizedSortAs == nil && c.Identifier == "" && c.Roles == "" && c.Position == nil && c.Links == nil && len(c.LocalizedName.translations) == 1 {
		// If everything but name is empty, and there's just one name, contributor can be just a name
		return json.Marshal(c.LocalizedName)
	}
	return json.Marshal(c)
}

func (c *Contributor) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}
	switch d.(type) {
	case string: // Just a single string Contributor
		c.LocalizedName = NewLocalizedStringFromString(d.(string))
	case map[string]interface{}: // Actual object Contributor
		type CNT *Contributor // Prevent infinite recursion
		cnt := CNT(c)
		return json.Unmarshal(data, cnt)
	default:
		return errors.New("Contributor has invalid JSON object")
	}
	return nil
}
