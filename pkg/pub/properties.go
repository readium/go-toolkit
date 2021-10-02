package pub

// TODO JSON marshal/unmarshal logic

type Properties map[string]interface{}

func (p *Properties) Add(newProperties map[string]interface{}) {
	if *p == nil {
		*p = make(Properties)
	}
	for k, v := range newProperties {
		(*p)[k] = v
	}
}

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
