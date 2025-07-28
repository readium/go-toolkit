package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// AltIdentifier
// https://github.com/readium/webpub-manifest/tree/master/contexts/default#identifier
// https://github.com/readium/webpub-manifest/blob/master/schema/altIdentifier.schema.json
type AltIdentifier struct {
	Value  string `json:"value"`
	Scheme string `json:"scheme,omitempty"`
}

// Parses an [AltIdentifier] from its RWPM JSON representation.
// A altIdentifier can be parsed from a single string, or an object.
func AltIdentifierFromJSON(rawJson any) (*AltIdentifier, error) {
	if rawJson == nil {
		return nil, nil
	}
	switch rjs := rawJson.(type) {
	case string:
		return &AltIdentifier{Value: rjs}, nil
	case map[string]any:
		n := AltIdentifier{
			Value:  parseOptString(rjs["value"]),
			Scheme: parseOptString(rjs["scheme"]),
		}
		if n.Value == "" {
			return nil, errors.New("AltIdentifier must have a non-empty 'value' field")
		}

		return &n, nil
	default:
		return nil, errors.New("AltIdentifier has invalid JSON object")
	}
}

// Creates a list of [AltIdentifier] from its RWPM JSON representation.
func AltIdentifierFromJSONArray(rawJsonArray any) ([]AltIdentifier, error) {
	var altIdentifiers []AltIdentifier
	switch rjx := rawJsonArray.(type) {
	case []any:
		altIdentifiers = make([]AltIdentifier, 0, len(rjx))
		for i, entry := range rjx {
			ri, err := AltIdentifierFromJSON(entry)
			if err != nil {
				return nil, errors.Wrapf(err, "failed unmarshalling AltIdentifier at position %d", i)
			}
			if ri == nil {
				continue
			}
			altIdentifiers = append(altIdentifiers, *ri)
		}
	default:
		i, err := AltIdentifierFromJSON(rjx)
		if err != nil {
			return nil, err
		}
		if i != nil {
			altIdentifiers = []AltIdentifier{*i}
		}
	}
	return altIdentifiers, nil
}

func (s *AltIdentifier) UnmarshalJSON(data []byte) error {
	var object any
	err := json.Unmarshal(data, &object)
	if err != nil {
		return err
	}
	fs, err := AltIdentifierFromJSON(object)
	if err != nil {
		return err
	}
	*s = *fs
	return nil
}

func (s AltIdentifier) MarshalJSON() ([]byte, error) {
	if s.Scheme == "" {
		// If scheme is empty, AltIdentifier can be just a string value
		return json.Marshal(s.Value)
	}
	type alias AltIdentifier // Prevent infinite recursion
	return json.Marshal(alias(s))
}
