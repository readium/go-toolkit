package manifest

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// TODO replace with generic
type Strings []string

func (s Strings) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal(s)
}

// Metadata for the default context in WebPub
type Metadata struct {
	Identifier         string                  `json:"identifier,omitempty"`
	Type               string                  `json:"@type,omitempty"`
	LocalizedTitle     LocalizedString         `json:"title" validate:"required"`
	LocalizedSubtitle  *LocalizedString        `json:"subtitle,omitempty"`
	LocalizedSortAs    *LocalizedString        `json:"sortAs,omitempty"`
	Modified           *time.Time              `json:"modified,omitempty"`
	Published          *time.Time              `json:"published,omitempty"`
	Languages          Strings                 `json:"language,omitempty" validate:"BCP47"` // TODO validator
	Subjects           []Subject               `json:"subject,omitempty"`
	Authors            Contributors            `json:"author,omitempty"`
	Translators        Contributors            `json:"translator,omitempty"`
	Editors            Contributors            `json:"editor,omitempty"`
	Artists            Contributors            `json:"artist,omitempty"`
	Illustrators       Contributors            `json:"illustrator,omitempty"`
	Letterers          Contributors            `json:"letterer,omitempty"`
	Pencilers          Contributors            `json:"penciler,omitempty"`
	Colorists          Contributors            `json:"colorist,omitempty"`
	Inkers             Contributors            `json:"inker,omitempty"`
	Narrators          Contributors            `json:"narrator,omitempty"`
	Contributors       Contributors            `json:"contributor,omitempty"`
	Publishers         Contributors            `json:"publisher,omitempty"`
	Imprints           Contributors            `json:"imprint,omitempty"`
	ReadingProgression ReadingProgression      `json:"readingProgression,omitempty" validate:"readingProgression"` // TODO validator.
	Description        string                  `json:"description,omitempty"`
	Duration           *float64                `json:"duration,omitempty" validator:"positive"` // TODO validator
	NumberOfPages      *uint                   `json:"numberOfPages,omitempty"`
	BelongsTo          map[string][]Collection `json:"belongsTo,omitempty"`
	Presentation       *Presentation           `json:"presentation,omitempty"`

	OtherMetadata map[string]interface{} `json:"-"` // Extension point for other metadata. TODO implement
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
	if m.ReadingProgression != "" && m.ReadingProgression != Auto {
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
	if language == "ar" || language == "fa" || language == "he" {
		return RTL
	}

	return LTR
}

func MetadataFromJSON(rawJson map[string]interface{}, normalizeHref LinkHrefNormalizer) (*Metadata, error) {
	if rawJson == nil {
		return nil, nil
	}

	title, err := LocalizedStringFromJSON(rawJson["title"])
	if err != nil || title == nil {
		// Warning: [title] is required
		return nil, errors.Wrap(err, "failed parsing 'title'")
	}

	metadata := &Metadata{
		Identifier:         parseOptString(rawJson["identifier"]),
		Type:               parseOptString(rawJson["@type"]),
		LocalizedTitle:     *title,
		Modified:           parseOptTime(rawJson["modified"]),
		Published:          parseOptTime(rawJson["published"]),
		ReadingProgression: ReadingProgression(parseOptString(rawJson["readingProgression"])),
		Description:        parseOptString(rawJson["description"]),
	}

	// LocalizedSubtitle
	ls, ok := rawJson["subtitle"]
	if ok {
		localizedSubtitle, err := LocalizedStringFromJSON(ls)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing Metadata 'subtitle' as LocalizedString")
		}
		metadata.LocalizedSubtitle = localizedSubtitle
	}

	// LocalizedSortAs
	lsr, ok := rawJson["sortAs"]
	if ok {
		localizedSortAs, err := LocalizedStringFromJSON(lsr)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing Metadata 'sortAs' as LocalizedString")
		}
		metadata.LocalizedSortAs = localizedSortAs
	}

	// Languages
	languages, err := parseSliceOrString(rawJson["language"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'language'")
	}
	metadata.Languages = languages

	// Subjects
	subjects, err := SubjectFromJSONArray(rawJson["subjects"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'subjects'")
	}
	metadata.Subjects = subjects

	// Contributors
	contributors, err := ContributorFromJSONArray(rawJson["contributor"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'contributor'")
	}
	metadata.Contributors = contributors

	// Publishers
	contributors, err = ContributorFromJSONArray(rawJson["publisher"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'publisher'")
	}
	metadata.Publishers = contributors

	// Imprints
	contributors, err = ContributorFromJSONArray(rawJson["imprint"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'imprint'")
	}
	metadata.Imprints = contributors

	// Authors
	contributors, err = ContributorFromJSONArray(rawJson["author"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'author'")
	}
	metadata.Authors = contributors

	// Translators
	contributors, err = ContributorFromJSONArray(rawJson["translator"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'translator'")
	}
	metadata.Translators = contributors

	// Editors
	contributors, err = ContributorFromJSONArray(rawJson["editor"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'editor'")
	}
	metadata.Editors = contributors

	// Artists
	contributors, err = ContributorFromJSONArray(rawJson["artist"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'artist'")
	}
	metadata.Artists = contributors

	// Illustrators
	contributors, err = ContributorFromJSONArray(rawJson["illustrator"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'illustrator'")
	}
	metadata.Illustrators = contributors

	// Letterers
	contributors, err = ContributorFromJSONArray(rawJson["letterer"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'letterer'")
	}
	metadata.Letterers = contributors

	// Pencilers
	contributors, err = ContributorFromJSONArray(rawJson["penciler"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'penciler'")
	}
	metadata.Pencilers = contributors

	// Colorists
	contributors, err = ContributorFromJSONArray(rawJson["colorist"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'colorist'")
	}
	metadata.Colorists = contributors

	// Inkers
	contributors, err = ContributorFromJSONArray(rawJson["inker"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'inker'")
	}
	metadata.Inkers = contributors

	// Narrators
	contributors, err = ContributorFromJSONArray(rawJson["narrator"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'narrator'")
	}
	metadata.Narrators = contributors

	// Duration
	duration, ok := rawJson["duration"].(float64)
	if ok {
		metadata.Duration = &duration
	}

	// NumberOfPages
	numberOfPages, ok := rawJson["numberOfPages"].(uint)
	if ok {
		metadata.NumberOfPages = &numberOfPages
	}

	// BelongsTo
	belongsToRaw, ok := rawJson["belongsTo"].(map[string]interface{})
	if !ok {
		belongsToRaw, _ = rawJson["belongs_to"].(map[string]interface{})
	}
	if belongsToRaw != nil {
		belongsTo := make(map[string][]Collection)
		for k, v := range belongsToRaw {
			if v == nil {
				continue
			}
			cl, err := ContributorFromJSONArray(v, normalizeHref)
			if err != nil {
				return nil, errors.Wrapf(err, "failed parsing 'belongsTo.%s'", k)
			}
			belongsTo[k] = cl
		}
		metadata.BelongsTo = belongsTo
	}

	// Delete above vals so that we can put everything else in OtherMetadata
	for _, v := range []string{
		"title", "subtitle", "sortAs", "identifier", "@type", "modified", "published", "readingProgression", "description", "subjects", "language",
		"contributor", "publisher", "imprint", "author", "translator", "editor", "artist", "illustrator", "letterer", "penciler", "colorist", "inker", "narrator",
		"duration", "numberOfPages", "belongsTo", "belongs_to",
	} {
		delete(rawJson, v)
	}

	// Now all we have left is everything else!
	metadata.OtherMetadata = rawJson

	return metadata, nil
}

func (m *Metadata) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fm, err := MetadataFromJSON(object, LinkHrefNormalizerIdentity)
	if err != nil {
		return err
	}
	*m = *fm
	return nil
}

// TODO Metadata MarshalJSON to handle OtherMetadata
