package manifest

// TODO JSON marshal/unmarshal logic

type Properties map[string]interface{}

func (p Properties) Add(newProperties map[string]interface{}) {
	if p == nil {
		p = make(Properties)
	}
	for k, v := range newProperties {
		p[k] = v
	}
}

func (p Properties) Get(key string) interface{} {
	if p != nil {
		return p[key]
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
func (p Properties) Layout() EpubLayout {
	return EpubLayout(p.GetString("layout"))
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
