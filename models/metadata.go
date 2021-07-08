package models

import (
	"encoding/json"
	"time"
)

// Metadata for the default context in WebPub
type Metadata struct {
	RDFType         string        `json:"@type,omitempty"` //Defaults to schema.org for EBook
	Title           MultiLanguage `json:"title"`
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
	Presentation    *Properties   `json:"presentation,omitempty"`
	Source          string        `json:"source,omitempty"`
	EpubType        []string      `json:"epub-type,omitempty"`
	Rights          string        `json:"rights,omitempty"`
	Subject         []Subject     `json:"subject,omitempty"`
	BelongsTo       *BelongsTo    `json:"belongs_to,omitempty"`
	Duration        int           `json:"duration,omitempty"`

	OtherMetadata []Meta `json:"-"` //Extension point for other metadata
}

// Meta is a generic structure for other metadata
type Meta struct {
	Property string
	Value    interface{}
	Children []Meta
}

// Contributor construct used internally for all contributors
type Contributor struct {
	Name       MultiLanguage `json:"name,omitempty"`
	SortAs     string        `json:"sort_as,omitempty"`
	Identifier string        `json:"identifier,omitempty"`
	Role       string        `json:"role,omitempty"`
}

// Properties object use to link properties
// Use also in Rendition for fxl
type Properties struct {
	Contains     []string   `json:"contains,omitempty"`
	Layout       string     `json:"layout,omitempty"`
	MediaOverlay string     `json:"media-overlay,omitempty"`
	Orientation  string     `json:"orientation,omitempty"`
	Overflow     string     `json:"overflow,omitempty"`
	Page         string     `json:"page,omitempty"`
	Spread       string     `json:"spread,omitempty"`
	Encrypted    *Encrypted `json:"encrypted,omitempty"`
}

// Encrypted contains metadata from encryption xml
type Encrypted struct {
	Scheme         string `json:"scheme,omitempty"`
	Profile        string `json:"profile,omitempty"`
	Algorithm      string `json:"algorithm,omitempty"`
	Compression    string `json:"compression,omitempty"`
	OriginalLength int    `json:"original-length,omitempty"`
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

// MultiLanguage store the a basic string when we only have one lang
// Store in a hash by language for multiple string representation
type MultiLanguage struct {
	SingleString string
	MultiString  map[string]string
}

// MarshalJSON overwrite json marshalling for MultiLanguage
// when we have an entry in the Multi fields we use it
// otherwise we use the single string
func (m MultiLanguage) MarshalJSON() ([]byte, error) {
	if len(m.MultiString) > 0 {
		return json.Marshal(m.MultiString)
	}
	return json.Marshal(m.SingleString)
}

func (m MultiLanguage) String() string {
	if len(m.MultiString) > 0 {
		for _, s := range m.MultiString {
			return s
		}
	}
	return m.SingleString
}
