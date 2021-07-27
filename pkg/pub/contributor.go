package pub

// Contributor
// https://github.com/readium/webpub-manifest/tree/master/contexts/default#contributors
// https://readium.org/webpub-manifest/schema/contributor-object.schema.json
type Contributor struct {
	LocalizedName   MultiLanguage  `json:"name" validate:"required"` // The name of the contributor.
	LocalizedSortAs *MultiLanguage `json:"sortAs,omitempty"`         // The string used to sort the name of the contributor.
	Identifier      string         `json:"identifier,omitempty"`     // An unambiguous reference to this contributor.
	Roles           string         `json:"role,omitempty"`           // The roles of the contributor in the publication making.
	Position        *float64       `json:"position"`                 // The position of the publication in this collection/series, when the contributor represents a collection. TODO validator
	Links           []Link         `json:"links,omitempty"`          // Used to retrieve similar publications for the given contributor.
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
