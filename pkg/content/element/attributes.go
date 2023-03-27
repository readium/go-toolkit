package element

type AttributeKey string

const AcessibilityLabelKey AttributeKey = "accessibilityLabel"
const LanguageKey AttributeKey = "language"

// An attribute is an arbitrary key-value metadata pair.
type Attribute[T any] struct {
	Key   AttributeKey
	Value T
}

// An object associated with a list of attributes.
type AttributesHolder struct {
	attributes []Attribute[any] // Associated list of attributes.
}

func (ah AttributesHolder) Language() string {
	v := ah.Attribute(LanguageKey)
	if v != nil {
		return v.Value.(string)
	}
	return ""
}

func (ah AttributesHolder) AccessibilityLabel() string {
	v := ah.Attribute(AcessibilityLabelKey)
	if v != nil {
		return v.Value.(string)
	}
	return ""
}

// Gets the first attribute with the given [key].
func (ah AttributesHolder) Attribute(key AttributeKey) *Attribute[any] {
	for _, at := range ah.attributes {
		if at.Key == key {
			return &at
		}
	}
	return nil
}

// Gets all the attributes with the given [key].
func (ah AttributesHolder) Attributes(key AttributeKey) []Attribute[any] {
	var result []Attribute[any]
	for _, at := range ah.attributes {
		if at.Key == key {
			result = append(result, at)
		}
	}
	return result
}
