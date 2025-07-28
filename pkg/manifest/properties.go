package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Properties associated with a linked resource
type Properties map[string]interface{}

// Properties should be immutable, therefore these functions have been removed.
// The code is left here in case it's useful in a future implementation.

/*func (p *Properties) Add(newProperties Properties) Properties {
	if *p == nil {
		*p = make(Properties)
	}
	for k, v := range newProperties {
		(*p)[k] = v
	}
	return *p
}

func (p *Properties) Delete(key string) Properties {
	if p == nil {
		p = &Properties{}
	}
	delete(*p, key)
	return *p
}*/

func (p *Properties) Get(key string) interface{} {
	if p != nil {
		return (*p)[key]
	}
	return nil
}

func (p Properties) GetString(key string) string {
	if p == nil {
		return ""
	}
	v, ok := p[key]
	if !ok {
		return ""
	}
	cv, ok := v.(string)
	if !ok {
		return ""
	}
	return cv
}

func (p Properties) GetBool(key string) *bool {
	if p == nil {
		return nil
	}
	v, ok := p[key]
	if !ok {
		return nil
	}
	cv, ok := v.(bool)
	if !ok {
		return nil
	}
	return &cv
}

type Page string // Indicates how the linked resource should be displayed in a reading environment that displays synthetic spreads.
const (
	PageNone   Page = ""
	PageLeft   Page = "left"
	PageRight  Page = "right"
	PageCenter Page = "center"
)

// Indicates how the linked resource should be displayed in a reading environment that displays synthetic spreads.
func (p Properties) Page() Page {
	v := p.GetString("page")
	if v == "" {
		return PageNone
	}
	return Page(v)
}

// Indicates that a resource is encrypted/obfuscated and provides relevant information for decryption.
func (p Properties) Encryption() *Encryption {
	v := p.Get("encrypted")
	if v == nil {
		return nil
	}
	mp, ok := v.(map[string]interface{})
	if mp == nil || !ok {
		return nil
	}
	enc, err := EncryptionFromJSON(mp)
	if err != nil {
		return nil
	}
	return enc
}

// Identifies content contained in the linked resource, that cannot be strictly identified using a media type.
func (p Properties) Contains() []string {
	v := p.Get("contains")
	if v == nil {
		return nil
	}
	cv, ok := v.([]string)
	if !ok {
		return nil
	}
	return cv // Maybe TODO: it's a set
}

func (p Properties) Hash() HashList {
	v := p.Get("hash")
	if v == nil {
		return nil
	}
	cv, ok := v.([]interface{})
	if !ok {
		return nil
	}
	hashes, err := HashListFromJSONArray(cv)
	if err != nil {
		return nil
	}
	return hashes
}

func PropertiesFromJSON(rawJson interface{}) (Properties, error) {
	if rawJson == nil {
		return make(Properties), nil
	}

	properties, ok := rawJson.(map[string]interface{})
	if !ok {
		return nil, errors.New("Properties has invalid JSON object")
	}
	return properties, nil
}

func (p *Properties) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}
	pr, err := PropertiesFromJSON(d)
	if err != nil {
		return err
	}
	*p = pr
	return nil
}
