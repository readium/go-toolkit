package element

type AttributeKey string

const AcessibilityLabelAttributeKey AttributeKey = "accessibilityLabel"
const LanguageAttributeKey AttributeKey = "language"

// An attribute is an arbitrary key-value metadata pair.
type Attribute[T any] struct {
	Key   AttributeKey
	Value T
}

func NewAttribute(key AttributeKey, value any) Attribute[any] {
	return Attribute[any]{
		Key:   key,
		Value: value,
	}
}

// An object associated with a list of attributes.
type AttributesHolder struct {
	attributes []Attribute[any] // Associated list of attributes.
}

func NewAttributesHolder(attributes []Attribute[any]) AttributesHolder {
	return AttributesHolder{
		attributes: attributes,
	}
}

func (ah AttributesHolder) Language() string {
	v := ah.GetFirst(LanguageAttributeKey)
	if v != nil {
		return v.Value.(string)
	}
	return ""
}

func (ah AttributesHolder) AccessibilityLabel() string {
	v := ah.GetFirst(AcessibilityLabelAttributeKey)
	if v != nil {
		return v.Value.(string)
	}
	return ""
}

// Gets the first attribute with the given [key].
func (ah AttributesHolder) GetFirst(key AttributeKey) *Attribute[any] {
	for _, at := range ah.attributes {
		if at.Key == key {
			return &at
		}
	}
	return nil
}

// Gets all the attributes with the given [key].
func (ah AttributesHolder) Get(key AttributeKey) AttributesHolder {
	var result []Attribute[any]
	for _, at := range ah.attributes {
		if at.Key == key {
			result = append(result, at)
		}
	}
	return NewAttributesHolder(result)
}

func (ah AttributesHolder) Attributes() AttributesHolder {
	return NewAttributesHolder(ah.attributes)
}
