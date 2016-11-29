package models

import "time"

// Publication publication struct
type Publication struct {
	Context   []string `json:"@context,omitempty"`
	Metadata  Metadata `json:"metadata"`
	Links     []Link   `json:"links"`
	Spine     []Link   `json:"spine"`
	Resources []Link   `json:"resources,omitempty"`

	TOC       []Link `json:"toc,omitempty"`
	PageList  []Link `json:"page-list,omitempty"`
	Landmarks []Link `json:"landmarks,omitempty"`
	LOI       []Link `json:"loi,omitempty"`
	LOA       []Link `json:"loa,omitempty"`
	LOV       []Link `json:"lov,omitempty"`
	LOT       []Link `json:"lot,omitempty"`
}

// Metadata metadata struct
type Metadata struct {
	RDFType         string        `json:"@type,omitempty"`
	Title           string        `json:"title"`
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
	Identifier      string        `json:"identifier"`
	Language        []string      `json:"language,omitempty"`
	Modified        *time.Time    `json:"modified,omitempty"`
	PublicationDate *time.Time    `json:"published,omitempty"`
	Description     string        `json:"description,omitempty"`
	Direction       string        `json:"direction,omitempty"`
	Rendition       Rendition     `json:"rendition,omitempty"`
	Source          string        `json:"source,omitempty"`
	EpubType        []string      `json:"epub-type,omitempty"`
	Right           string        `json:"right,omitempty"`
	Subject         []Subject     `json:"subject,omitempty"`
}

// Link link struct
type Link struct {
	Href       string         `json:"href"`
	TypeLink   string         `json:"type"`
	Rel        []string       `json:"rel,omitempty"`
	Height     int            `json:"height,omitempty"`
	Width      int            `json:"width,omitempty"`
	Title      string         `json:"title,omitempty"`
	Properties []string       `json:"properties,omitempty"`
	Duration   *time.Duration `json:"duration,omitempty"`
	Templated  bool           `json:"templated,omitempty"`
}

// Contributor shared Contributor struct
type Contributor struct {
	Name       string `json:"name"`
	SortAs     string `json:"sort_as,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	Role       string `json:"role,omitempty"`
}

// Rendition rendition
type Rendition struct {
	Flow        string `json:"flow,omitempty"`
	Layout      string `json:"layout,omitempty"`
	Orientation string `json:"orientation,omitempty"`
	Spread      string `json:"spread,omitempty"`
}

// Subject subject
type Subject struct {
	Name   string `json:"name"`
	SortAs string `json:"sort_as,omitempty"`
	Scheme string `json:"scheme,omitempty"`
	Code   string `json:"code,omitempty"`
}

// BelongsTo belongs to
type BelongsTo struct {
	Series     []Collection `json:"series,omitempty"`
	Collection []Collection `json:"collection,omitempty"`
}

// Collection shared Collection struct
type Collection struct {
	Name       string  `json:"name"`
	SortAs     string  `json:"sort_as,omitempty"`
	Identifier string  `json:"identifier,omitempty"`
	Position   float32 `json:"position,omitempty"`
}

func (publication *Publication) linkCover() {
	// returns the link object for the cover
}

func (publication *Publication) linkNavDoc() {

}
