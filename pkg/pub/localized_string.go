package pub

import "encoding/json"

// TODO replace with LocalizedString
// MultiLanguage store the a basic string when we only have one lang
// Store in a hash by language for multiple string representation
type MultiLanguage struct {
	SingleString string
	MultiString  map[string]string
}

// MarshalJSON overwrite json marshalling for MultiLanguage
// when we have an entry in the Multi fields we use it
// otherwise we use the single string
func (m MultiLanguage) MarshalJSON() ([]byte, error) {
	if len(m.MultiString) > 0 {
		return json.Marshal(m.MultiString)
	}
	return json.Marshal(m.SingleString)
}

func (m MultiLanguage) String() string {
	if len(m.MultiString) > 0 {
		for _, s := range m.MultiString {
			return s
		}
	}
	return m.SingleString
}
