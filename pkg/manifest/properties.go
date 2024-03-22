package manifest

import (
	"encoding/json"
	"sync"

	"github.com/pkg/errors"
)

type Properties struct {
	properties map[string]interface{}
	mutext     *sync.RWMutex
}

func (p *Properties) Add(newProperties map[string]interface{}) Properties {
	if p == nil {
		p = &Properties{}
	}
	if p.properties == nil || p.mutext == nil {
		p.properties = make(map[string]interface{})
		p.mutext = &sync.RWMutex{}
	}
	p.mutext.Lock()
	defer p.mutext.Unlock()

	for k, v := range newProperties {
		p.properties[k] = v
	}
	return *p
}

func (p *Properties) Delete(key string) Properties {
	if p == nil {
		p = &Properties{}
	}
	if p.properties == nil || p.mutext == nil {
		p.properties = make(map[string]interface{})
		p.mutext = &sync.RWMutex{}
	}
	p.mutext.Lock()
	defer p.mutext.Unlock()

	delete(p.properties, key)
	return *p
}

func (p Properties) Get(key string) interface{} {
	if p.properties != nil && p.mutext != nil {
		p.mutext.RLock()
		defer p.mutext.RUnlock()

		return p.properties[key]
	}
	return nil
}

func (p Properties) Length() int {
	if p.properties == nil || p.mutext == nil {
		return 0
	}
	p.mutext.RLock()
	defer p.mutext.RUnlock()

	return len(p.properties)
}

func (p Properties) GetString(key string) string {
	if p.properties == nil || p.mutext == nil {
		return ""
	}
	p.mutext.RLock()
	defer p.mutext.RUnlock()

	v, ok := p.properties[key]
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
	if p.properties == nil || p.mutext == nil {
		return nil
	}
	p.mutext.RLock()
	defer p.mutext.RUnlock()

	v, ok := p.properties[key]
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
	if p.properties == nil {
		return nil
	}
	p.mutext.RLock()
	defer p.mutext.RUnlock()

	v, ok := p.properties["contains"]
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
		return Properties{}, nil
	}

	properties, ok := rawJson.(map[string]interface{})
	if !ok {
		return Properties{}, errors.New("Properties has invalid JSON object")
	}
	if len(properties) > 0 {
		return Properties{
			properties: properties,
			mutext:     &sync.RWMutex{},
		}, nil
	}
	return Properties{}, nil
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

func (p Properties) MarshalJSON() ([]byte, error) {
	if p.properties == nil || p.mutext == nil {
		return json.Marshal(nil)
	}
	p.mutext.RLock()
	defer p.mutext.RUnlock()
	return json.Marshal(p.properties)
}
