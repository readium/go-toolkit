package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Subject
// https://github.com/readium/webpub-manifest/tree/master/contexts/default#subjects
// https://github.com/readium/webpub-manifest/blob/master/schema/subject-object.schema.json
type Subject struct {
	LocalizedName   LocalizedString  `json:"name" validate:"required"`
	LocalizedSortAs *LocalizedString `json:"sortAs,omitempty"`
	Scheme          string           `json:"scheme,omitempty"`
	Code            string           `json:"code,omitempty"`
	Links           LinkList         `json:"links,omitempty"`
}

func (s Subject) Name() string {
	return s.LocalizedName.String()
}

func (s Subject) SortAs() string {
	if s.LocalizedSortAs == nil {
		return ""
	}
	return s.LocalizedSortAs.String()
}

// Parses a [Subject] from its RWPM JSON representation.
// A subject can be parsed from a single string, or a full-fledged object.
// The [links]' href and their children's will be normalized recursively using the provided [normalizeHref] closure.
func SubjectFromJSON(rawJson interface{}) (*Subject, error) {
	if rawJson == nil {
		return nil, nil
	}
	switch rjs := rawJson.(type) {
	case string:
		localizedName, err := LocalizedStringFromJSON(rjs)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing Subject as LocalizedString")
		}
		return &Subject{LocalizedName: *localizedName}, nil
	case map[string]interface{}:
		localizedName, err := LocalizedStringFromJSON(rjs["name"])
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing Subject 'name' as LocalizedString")
		}

		s := &Subject{
			LocalizedName: *localizedName,
			Scheme:        parseOptString(rjs["scheme"]),
			Code:          parseOptString(rjs["code"]),
		}

		// sortAs
		lsr, ok := rjs["sortAs"]
		if ok {
			localizedSortAs, err := LocalizedStringFromJSON(lsr)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing Subject 'sortAs' as LocalizedString")
			}
			s.LocalizedSortAs = localizedSortAs
		}

		// links
		lln, ok := rjs["links"].([]interface{})
		if ok {
			links, err := LinksFromJSONArray(lln)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing Subject 'links'")
			}
			s.Links = links
		}

		return s, nil
	default:
		return nil, errors.New("Subject has invalid JSON object")
	}
}

// Creates a list of [Subject] from its RWPM JSON representation.
// The [links]' href and their children's will be normalized recursively using the provided [normalizeHref] closure.
func SubjectFromJSONArray(rawJsonArray interface{}) ([]Subject, error) {
	var subjects []Subject
	switch rjx := rawJsonArray.(type) {
	case []interface{}:
		subjects = make([]Subject, 0, len(rjx))
		for i, entry := range rjx {
			rs, err := SubjectFromJSON(entry)
			if err != nil {
				return nil, errors.Wrapf(err, "failed unmarshalling Subject at position %d", i)
			}
			if rs == nil {
				continue
			}
			subjects = append(subjects, *rs)
		}
	default:
		s, err := SubjectFromJSON(rjx)
		if err != nil {
			return nil, err
		}
		if s != nil {
			subjects = []Subject{*s}
		}
	}
	return subjects, nil
}

func (s *Subject) UnmarshalJSON(data []byte) error {
	var object interface{}
	err := json.Unmarshal(data, &object)
	if err != nil {
		return err
	}
	fs, err := SubjectFromJSON(object)
	if err != nil {
		return err
	}
	*s = *fs
	return nil
}

func (s Subject) MarshalJSON() ([]byte, error) {
	if s.LocalizedSortAs == nil && s.Scheme == "" && s.Code == "" && len(s.Links) == 0 {
		// If everything but name is empty, Subject can be just a name
		return json.Marshal(s.LocalizedName)
	}
	type alias Subject // Prevent infinite recursion
	return json.Marshal(alias(s))
}
