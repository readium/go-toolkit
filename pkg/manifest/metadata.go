package manifest

import (
	"strings"
	"time"
)

// ReadingProgression
// This is not a proper enum replacement! Use the validator to enforce the values
type ReadingProgression string

const (
	AUTO ReadingProgression = "auto"
	LTR                     = "ltr"
	RTL                     = "rtl"
	TTB                     = "ttb"
	BTT                     = "btt"
)

// Metadata for the default context in WebPub
type Metadata struct {
	Identifier         string                  `json:"identifier"` // Could be omitempty since it's optional
	Type               string                  `json:"@type,omitempty"`
	LocalizedTitle     LocalizedString         `json:"title" validate:"required"`
	LocalizedSubtitle  *LocalizedString        `json:"subtitle,omitempty"`
	LocalizedSortAs    *LocalizedString        `json:"sortAs,omitempty"`
	Modified           *time.Time              `json:"modified,omitempty"`
	Published          *time.Time              `json:"published,omitempty"`
	Languages          []string                `json:"language,omitempty" validate:"BCP47"` // TODO validator
	Subjects           []Subject               `json:"subject,omitempty"`
	Authors            []Contributor           `json:"author,omitempty"`
	Translators        []Contributor           `json:"translator,omitempty"`
	Editors            []Contributor           `json:"editor,omitempty"`
	Artists            []Contributor           `json:"artist,omitempty"`
	Illustrators       []Contributor           `json:"illustrator,omitempty"`
	Letterer           []Contributor           `json:"letterer,omitempty"`
	Pencilers          []Contributor           `json:"penciler,omitempty"`
	Colorists          []Contributor           `json:"colorist,omitempty"`
	Inkers             []Contributor           `json:"inker,omitempty"`
	Narrators          []Contributor           `json:"narrator,omitempty"`
	Contributors       []Contributor           `json:"contributor,omitempty"`
	Publishers         []Contributor           `json:"publisher,omitempty"`
	Imprints           []Contributor           `json:"imprint,omitempty"`
	ReadingProgression ReadingProgression      `json:"readingProgression,omitempty" validate:"readingProgression"` // TODO validator.
	Description        string                  `json:"description,omitempty"`
	Duration           *float64                `json:"duration,omitempty" validator:"positive"` // TODO validator
	NumberOfPages      *uint                   `json:"numberOfPages,omitempty"`
	BelongsTo          map[string][]Collection `json:"belongsTo,omitempty"`
	// TODO think of a way to replicate https://github.com/readium/r2-shared-kotlin/blob/develop/r2-shared/src/main/java/org/readium/r2/shared/publication/Metadata.kt#L125

	OtherMetadata map[string]interface{} `json:"-"` //Extension point for other metadata
}

func (m Metadata) Title() string {
	return m.LocalizedTitle.String()
}

func (m Metadata) Subtitle() string {
	return m.LocalizedSubtitle.String()
}

func (m Metadata) SortAs() string {
	return m.LocalizedSortAs.String()
}

func (m Metadata) BelongsToCollections() []Collection {
	btc, ok := m.BelongsTo["collection"]
	if !ok {
		return nil
	}
	return btc
}

func (m Metadata) BelongsToSeries() []Collection {
	bts, ok := m.BelongsTo["series"]
	if !ok {
		return nil
	}
	return bts
}

func (m Metadata) EffectiveReadingProgression() ReadingProgression {
	if m.ReadingProgression != "" && m.ReadingProgression != AUTO {
		return m.ReadingProgression
	}

	// The following is based off of:
	// https://github.com/readium/readium-css/blob/develop/docs/CSS16-internationalization.md#missing-page-progression-direction

	if len(m.Languages) != 1 {
		return LTR
	}

	language := ""
	if len(m.Languages) > 0 {
		language = strings.ToLower(m.Languages[0])
	}

	if language == "zh-hant" || language == "zh-tw" {
		return RTL
	}

	language = strings.SplitN(language, "-", 2)[0]
	if "ar" == language || "fa" == language || "he" == language {
		return RTL
	}

	return LTR
}

// Encryption contains metadata from encryption xml
type Encryption struct {
	Scheme         string `json:"scheme,omitempty"`
	Profile        string `json:"profile,omitempty"`
	Algorithm      string `json:"algorithm,omitempty"`
	Compression    string `json:"compression,omitempty"`
	OriginalLength int    `json:"original-length,omitempty"`
}

// Collection construct used for collection/serie metadata
type Collection struct {
	Name       string  `json:"name"`
	SortAs     string  `json:"sortAs,omitempty"`
	Identifier string  `json:"identifier,omitempty"`
	Position   float32 `json:"position,omitempty"`
}
