package element

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
)

// Note: We can't embed structs/interfaces in the interfaces otherwise they become
// "non-basic", meaning we then can't use them as returns for other interfaces like
// [Iterator], where it's the return type of many of the functions. Maybe we should
// rethink this approach with all the interfaces later when not copying the kotlin.

// Represents a single semantic content element part of a publication.
type Element interface {
	// AttributesHolder
	Language() string
	AccessibilityLabel() string
	Attribute(key AttributeKey) *Attribute[any]
	Attributes(key AttributeKey) []Attribute[any]

	Locator() manifest.Locator // Locator targeting this element in the Publication.
}

// An element which can be represented as human-readable text.
type TextualElement interface {
	// AttributesHolder
	Language() string
	AccessibilityLabel() string
	Attribute(key AttributeKey) *Attribute[any]
	Attributes(key AttributeKey) []Attribute[any]

	// Element
	Locator() manifest.Locator // Locator targeting this element in the Publication.

	Text() string // Human-readable text representation for this element.
}

// An element referencing an embedded external resource.
type EmbeddedElement interface {
	// AttributesHolder
	Language() string
	AccessibilityLabel() string
	Attribute(key AttributeKey) *Attribute[any]
	Attributes(key AttributeKey) []Attribute[any]

	// Element
	Locator() manifest.Locator // Locator targeting this element in the Publication.

	EmbeddedLink() manifest.Link // Referenced resource in the publication.
}

// An audio or video clip.
// Used for both audio and video the avoid code duplication since they're the same at the moment.
// TODO: Should we separate anyway?
type AVElement struct {
	locator      manifest.Locator
	embeddedLink manifest.Link
	AttributesHolder
}

// Implements Element
func (e AVElement) Locator() manifest.Locator {
	return e.locator
}

// Implements EmbeddedElement
func (e AVElement) EmbeddedLink() manifest.Link {
	return e.embeddedLink
}

// Implements TextualElement
func (e AVElement) Text() string {
	return e.AccessibilityLabel()
}

func NewAVElement(locator manifest.Locator, embeddedLink manifest.Link, attributes []Attribute[any]) AVElement {
	return AVElement{
		AttributesHolder: AttributesHolder{
			attributes: attributes,
		},
		locator:      locator,
		embeddedLink: embeddedLink,
	}
}

// A bitmap image.
// The caption is a short piece of text associated with the image.
type ImageElement struct {
	locator      manifest.Locator
	embeddedLink manifest.Link
	caption      string
	AttributesHolder
}

// Implements Element
func (e ImageElement) Locator() manifest.Locator {
	return e.locator
}

// Implements EmbeddedElement
func (e ImageElement) EmbeddedLink() manifest.Link {
	return e.embeddedLink
}

// Implements TextualElement
func (e ImageElement) Text() string {
	if e.caption != "" {
		// The caption might be a better text description than the accessibility label, when available.
		return e.caption
	}
	return e.AccessibilityLabel()
}

func NewImageElement(locator manifest.Locator, embeddedLink manifest.Link, caption string, attributes []Attribute[any]) ImageElement {
	return ImageElement{
		AttributesHolder: AttributesHolder{
			attributes: attributes,
		},
		caption:      caption,
		locator:      locator,
		embeddedLink: embeddedLink,
	}
}

// Ranged portion of text with associated attributes.
type TextSegement struct {
	AttributesHolder                  // Attributes associated with this segment, e.g. language.
	Locator          manifest.Locator // Locator to the segment of text.
	Text             string           // Text in the segment.
}

// A text element.
type TextElement struct {
	AttributesHolder
	locator  manifest.Locator
	role     TextRole
	segments []TextSegement
}

// Implements TextualElement
func (e TextElement) Text() string {
	var sb strings.Builder
	for _, v := range e.segments {
		sb.WriteString(v.Text)
	}
	return sb.String()
}

// Implements Element
func (e TextElement) Locator() manifest.Locator {
	return e.locator
}

func (e TextElement) Role() TextRole {
	return e.role
}
