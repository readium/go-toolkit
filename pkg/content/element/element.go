package element

import (
	"encoding/json"
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
	Attributes() AttributesHolder

	Locator() manifest.Locator // Locator targeting this element in the Publication.
}

func ElementToMap(e Element) map[string]interface{} {
	res := make(map[string]interface{})
	res["locator"] = e.Locator()
	if l := e.Language(); l != "" {
		res["language"] = l
	}
	if l := e.AccessibilityLabel(); l != "" {
		res["accessibilityLabel"] = l
	}
	return res
}

// An element which can be represented as human-readable text.
type TextualElement interface {
	// AttributesHolder
	Language() string
	AccessibilityLabel() string
	Attributes() AttributesHolder

	// Element
	Locator() manifest.Locator // Locator targeting this element in the Publication.

	Text() string // Human-readable text representation for this element.
}

// An element referencing an embedded external resource.
type EmbeddedElement interface {
	// AttributesHolder
	Language() string
	AccessibilityLabel() string
	Attributes() AttributesHolder

	// Element
	Locator() manifest.Locator // Locator targeting this element in the Publication.

	EmbeddedLink() manifest.Link // Referenced resource in the publication.
}

// An audio clip.
type AudioElement struct {
	locator      manifest.Locator
	embeddedLink manifest.Link
	AttributesHolder
}

// Implements Element
func (e AudioElement) Locator() manifest.Locator {
	return e.locator
}

// Implements EmbeddedElement
func (e AudioElement) EmbeddedLink() manifest.Link {
	e.embeddedLink.Href = strings.TrimPrefix(e.embeddedLink.Href, "/")
	return e.embeddedLink
}

// Implements TextualElement
func (e AudioElement) Text() string {
	return e.AccessibilityLabel()
}

func (e AudioElement) MarshalJSON() ([]byte, error) {
	res := ElementToMap(e)
	res["text"] = e.Text()
	res["link"] = e.EmbeddedLink()
	res["@type"] = "Video"
	return json.Marshal(res)
}

func NewAudioElement(locator manifest.Locator, embeddedLink manifest.Link, attributes []Attribute[any]) AudioElement {
	return AudioElement{
		AttributesHolder: AttributesHolder{
			attributes: attributes,
		},
		locator:      locator,
		embeddedLink: embeddedLink,
	}
}

// A video clip.
type VideoElement struct {
	locator      manifest.Locator
	embeddedLink manifest.Link
	AttributesHolder
}

// Implements Element
func (e VideoElement) Locator() manifest.Locator {
	return e.locator
}

// Implements EmbeddedElement
func (e VideoElement) EmbeddedLink() manifest.Link {
	e.embeddedLink.Href = strings.TrimPrefix(e.embeddedLink.Href, "/")
	return e.embeddedLink
}

// Implements TextualElement
func (e VideoElement) Text() string {
	return e.AccessibilityLabel()
}

func (e VideoElement) MarshalJSON() ([]byte, error) {
	res := ElementToMap(e)
	res["text"] = e.Text()
	res["link"] = e.EmbeddedLink()
	res["@type"] = "Video"
	return json.Marshal(res)
}

func NewVideoElement(locator manifest.Locator, embeddedLink manifest.Link, attributes []Attribute[any]) VideoElement {
	return VideoElement{
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
	e.embeddedLink.Href = strings.TrimPrefix(e.embeddedLink.Href, "/")
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

func (e ImageElement) MarshalJSON() ([]byte, error) {
	res := ElementToMap(e)
	res["text"] = e.Text()
	res["link"] = e.EmbeddedLink()
	res["@type"] = "Image"
	return json.Marshal(res)
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
type TextSegment struct {
	AttributesHolder                  // Attributes associated with this segment, e.g. language.
	Locator          manifest.Locator // Locator to the segment of text.
	Text             string           // Text in the segment.
}

// A text element.
type TextElement struct {
	AttributesHolder
	locator  manifest.Locator
	role     TextRole
	segments []TextSegment
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

func (e TextElement) MarshalJSON() ([]byte, error) {
	res := ElementToMap(e)
	res["role"] = e.role.Role()
	textElements := make([]interface{}, len(e.segments))
	for i, s := range e.segments {
		te := map[string]interface{}{
			"locator": s.Locator,
			"text":    s.Text,
		}
		if l := s.Language(); l != "" {
			te["language"] = l
		}
		if l := s.AccessibilityLabel(); l != "" {
			te["accessibilityLabel"] = l
		}
		textElements[i] = te
	}
	res["text"] = textElements
	res["@type"] = "Text"
	return json.Marshal(res)
}

func NewTextElement(locator manifest.Locator, role TextRole, segments []TextSegment, attributes []Attribute[any]) TextElement {
	return TextElement{
		AttributesHolder: AttributesHolder{
			attributes: attributes,
		},
		locator:  locator,
		role:     role,
		segments: segments,
	}
}
