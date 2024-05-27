package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

// Contributor
// https://github.com/readium/webpub-manifest/tree/master/contexts/default#contributors
// https://github.com/readium/webpub-manifest/schema/contributor-object.schema.json
type Contributor struct {
	LocalizedName   LocalizedString  `json:"name" validate:"required"` // The name of the contributor.
	LocalizedSortAs *LocalizedString `json:"sortAs,omitempty"`         // The string used to sort the name of the contributor.
	Identifier      string           `json:"identifier,omitempty"`     // An unambiguous reference to this contributor.
	Roles           Strings          `json:"role,omitempty"`           // The roles of the contributor in the making of the publication.
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

// Parses a [Contributor] from its RWPM JSON representation.
// A contributor can be parsed from a single string, or a full-fledged object.
// The [links]' href and their children's will be normalized recursively using the provided [normalizeHref] closure.
func ContributorFromJSON(rawJson interface{}, normalizeHref LinkHrefNormalizer) (*Contributor, error) {
	if rawJson == nil {
		return nil, nil
	}

	c := new(Contributor)
	switch dd := rawJson.(type) {
	case string: // Just a single string Contributor
		c.LocalizedName = NewLocalizedStringFromString(dd)
	case map[string]interface{}: // Actual object Contributor
		// LocalizedName
		nr, ok := dd["name"]
		if !ok {
			// No name means the Contributor is invalid
			return nil, errors.New("Contributor has no 'name'")
		}
		localizedName, err := LocalizedStringFromJSON(nr)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing Contributor 'name' as LocalizedString")
		}
		c.LocalizedName = *localizedName

		// LocalizedSortAs
		lsr, ok := dd["sortAs"]
		if ok {
			localizedSortAs, err := LocalizedStringFromJSON(lsr)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing Contributor 'sortAs' as LocalizedString")
			}
			c.LocalizedSortAs = localizedSortAs
		}

		// Roles
		roles, err := parseSliceOrString(dd["role"], true)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing Contributor 'sortAs' as LocalizedString")
		}
		c.Roles = roles

		// Links
		rawLinks, ok := dd["links"].([]interface{})
		if ok {
			links, err := LinksFromJSONArray(rawLinks, normalizeHref)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshalling 'links'")
			}
			c.Links = links
		}

		// Identifier
		c.Identifier = parseOptString(dd["identifier"])

		// Position
		position, ok := dd["position"].(float64)
		if ok { // Need to do this because default is not 0, but nil
			c.Position = &position
		}

	default:
		return nil, errors.New("Contributor has invalid JSON object")
	}
	return c, nil
}

func ContributorFromJSONArray(rawJsonArray interface{}, normalizeHref LinkHrefNormalizer) ([]Contributor, error) {
	var contributors []Contributor
	switch rjx := rawJsonArray.(type) {
	case []interface{}:
		contributors = make([]Contributor, 0, len(rjx))
		for i, entry := range rjx {
			rc, err := ContributorFromJSON(entry, normalizeHref)
			if err != nil {
				return nil, errors.Wrapf(err, "failed unmarshalling Contributor at position %d", i)
			}
			if rc == nil {
				continue
			}
			contributors = append(contributors, *rc)
		}
	default:
		c, err := ContributorFromJSON(rjx, normalizeHref)
		if err != nil {
			return nil, err
		}
		if c != nil {
			contributors = []Contributor{*c}
		}
	}
	return contributors, nil
}

func (c Contributor) MarshalJSON() ([]byte, error) {
	if c.LocalizedSortAs == nil && c.Identifier == "" && len(c.Roles) == 0 && c.Position == nil && c.Links == nil && len(c.LocalizedName.Translations) == 1 {
		// If everything but name is empty, and there's just one name, Contributor can be just a name
		return json.Marshal(c.LocalizedName)
	}
	type alias Contributor // Prevent infinite recursion
	return json.Marshal(alias(c))
}

func (c *Contributor) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}
	fc, err := ContributorFromJSON(d, LinkHrefNormalizerIdentity)
	if err != nil {
		return err
	}
	*c = *fc
	return nil
}

// TODO replace with generic
type Contributors []Contributor

func (c Contributors) MarshalJSON() ([]byte, error) {
	if len(c) == 0 {
		return []byte("null"), nil
	}

	// De-duplicate contributors before marshalling
	marshalled, err := extensions.DeduplicateAndMarshalJSON([]Contributor(c))
	if err != nil {
		return nil, err
	}
	if len(marshalled) == 1 {
		return json.Marshal(marshalled[0])
	}
	return json.Marshal(marshalled)
}
