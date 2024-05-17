package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

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

// Specifies whether or not the parts of a linked resource that flow out of the viewport are clippped.
func (p Properties) Clipped() *bool {
	return p.GetBool("clipped")
}

// Suggested method for constraining a resource inside the viewport.
func (p Properties) Fit() Fit {
	return Fit(p.GetString("fit"))
}

// Suggested orientation for the device when displaying the linked resource.
func (p Properties) Orientation() Orientation {
	return Orientation(p.GetString("orientation"))
}

// Suggested method for handling overflow while displaying the linked resource.
func (p Properties) Overflow() Overflow {
	return Overflow(p.GetString("overflow"))
}

// Indicates how the linked resource should be displayed in a reading environment that displays synthetic spreads.
func (p Properties) Page() Page {
	return Page(p.GetString("page"))
}

// Indicates the condition to be met for the linked resource to be rendered within a synthetic spread.
func (p Properties) Spread() Spread {
	return Spread(p.GetString("spread"))
}

// Hints how the layout of the resource should be presented.
func (p Properties) Layout() EPUBLayout {
	return EPUBLayout(p.GetString("layout"))
}

func (p Properties) Encryption() *Encryption {
	mp, ok := p.Get("encrypted").(map[string]interface{})
	if mp == nil || !ok {
		return nil
	}

	enc, err := EncryptionFromJSON(mp)
	if err != nil {
		return nil
	}
	return enc
}

func (p Properties) Contains() []string {
	if p == nil {
		return nil
	}
	v, ok := p["contains"]
	if !ok {
		return nil
	}
	cv, ok := v.([]string)
	if !ok {
		return nil
	}
	return cv // Maybe TODO: it's a set
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
