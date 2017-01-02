package models

import "time"

// Metadata for the default context in WebPub
type Metadata struct {
	RDFType         string        `json:"@type,omitempty"` //Defaults to schema.org for EBook
	Title           string        `json:"title"`
	Identifier      string        `json:"identifier"`
	Author          []Contributor `json:"author,omitempty"`
	Translator      []Contributor `json:"translator,omitempty"`
	Editor          []Contributor `json:"editor,omitempty"`
	Artist          []Contributor `json:"artist,omitempty"`
	Illustrator     []Contributor `json:"illustrator,omitempty"`
	Letterer        []Contributor `json:"letterer,omitempty"`
	Penciler        []Contributor `json:"penciler,omitempty"`
	Colorist        []Contributor `json:"colorist,omitempty"`
	Inker           []Contributor `json:"inker,omitempty"`
	Narrator        []Contributor `json:"narrator,omitempty"`
	Contributor     []Contributor `json:"contributor,omitempty"`
	Publisher       []Contributor `json:"publisher,omitempty"`
	Imprint         []Contributor `json:"imprint,omitempty"`
	Language        []string      `json:"language,omitempty"`
	Modified        *time.Time    `json:"modified,omitempty"`
	PublicationDate *time.Time    `json:"published,omitempty"`
	Description     string        `json:"description,omitempty"`
	Direction       string        `json:"direction,omitempty"`
	Rendition       *Rendition    `json:"rendition,omitempty"`
	Source          string        `json:"source,omitempty"`
	EpubType        []string      `json:"epub-type,omitempty"`
	Right           string        `json:"rights,omitempty"`
	Subject         []Subject     `json:"subject,omitempty"`

	OtherMetadata []Meta `json:"-"` //Extension point for other metadata
}

// Meta is a generic structure for other metadata
type Meta struct {
	property string
	value    string
	children []Meta
}

// Contributor construct used internally for all contributors
type Contributor struct {
	Name       string `json:"name"`
	SortAs     string `json:"sort_as,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	Role       string `json:"role,omitempty"`
}

// Rendition object for reflow/FXL
type Rendition struct {
	Flow        string `json:"flow,omitempty"`
	Layout      string `json:"layout,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Spread      string `json:"spread,omitempty"`
}

// Subject as based on EPUB 3.1 and WePpub
type Subject struct {
	Name   string `json:"name"`
	SortAs string `json:"sort_as,omitempty"`
	Scheme string `json:"scheme,omitempty"`
	Code   string `json:"code,omitempty"`
}

// BelongsTo is a list of collections/series that a publication belongs to
type BelongsTo struct {
	Series     []Collection `json:"series,omitempty"`
	Collection []Collection `json:"collection,omitempty"`
}

// Collection construct used for collection/serie metadata
type Collection struct {
	Name       string  `json:"name"`
	SortAs     string  `json:"sort_as,omitempty"`
	Identifier string  `json:"identifier,omitempty"`
	Position   float32 `json:"position,omitempty"`
}
