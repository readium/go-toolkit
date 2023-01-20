package manifest

import (
	"encoding/json"
	"errors"
)

// Encryption contains metadata from encryption xml
type Encryption struct {
	Scheme         string `json:"scheme,omitempty"`
	Profile        string `json:"profile,omitempty"`
	Algorithm      string `json:"algorithm,omitempty"`
	Compression    string `json:"compression,omitempty"`
	OriginalLength int64  `json:"originalLength,omitempty"`
}

func EncryptionFromJSON(rawJson map[string]interface{}) (*Encryption, error) {
	if rawJson == nil {
		return nil, nil
	}

	algorithm, ok := rawJson["algorithm"].(string)
	if !ok || algorithm == "" {
		return nil, errors.New("[algorithm] is required") // TODO warning
	}

	e := new(Encryption)
	e.Algorithm = algorithm
	e.Compression = parseOptString(rawJson["compression"])
	e.OriginalLength = int64(parseOptFloat64(rawJson["originalLength"]))
	if e.OriginalLength == 0 {
		e.OriginalLength = int64(parseOptFloat64(rawJson["original-length"]))
	}
	e.Profile = parseOptString(rawJson["profile"])
	e.Scheme = parseOptString(rawJson["scheme"])

	return e, nil
}

func (e *Encryption) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}

	mp, ok := d.(map[string]interface{})
	if !ok {
		return errors.New("encryption object not a map with string keys")
	}

	fe, err := EncryptionFromJSON(mp)
	if err != nil {
		return err
	}
	*e = *fe
	return nil
}

func (m Encryption) ToMap() map[string]interface{} {
	mp := make(map[string]interface{})
	mp["algorithm"] = m.Algorithm
	if m.Compression != "" {
		mp["compression"] = m.Compression
	}
	if m.OriginalLength != 0 {
		mp["originalLength"] = m.OriginalLength
	}
	if m.Profile != "" {
		mp["profile"] = m.Profile
	}
	if m.Scheme != "" {
		mp["scheme"] = m.Scheme
	}
	return mp
}
