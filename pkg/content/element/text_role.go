package element

import (
	"fmt"
	"net/url"
)

// Represents a purpose of an element in the broader context of the document.
type TextRole interface {
	Role() string
}

// Title of a section.
type Heading struct {
	Level int // Heading importance, 1 being the highest.
}

func (h Heading) Role() string {
	return fmt.Sprintf("heading-%d", h.Level)
}

// Normal body of content.
type Body struct{}

func (b Body) Role() string {
	return "body"
}

// A footnote at the bottom of a document.
type Footnote struct{}

func (f Footnote) Role() string {
	return "footnote"
}

type Quote struct {
	ReferenceURL   *url.URL // URL to the source for this quote.
	ReferenceTitle string   // Name of the source for this quote.
}

func (q Quote) Role() string {
	return "quote"
}
