package manifest

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/util"
)

// TODO replace with generic
type Strings []string

func (s Strings) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	type alias Strings
	return json.Marshal(alias(s))
}

// Metadata for the default context in WebPub
type Metadata struct {
	Identifier         string                 `json:"identifier,omitempty"`
	Type               string                 `json:"@type,omitempty"`
	ConformsTo         Profiles               `json:"conformsTo,omitempty"`
	LocalizedTitle     LocalizedString        `json:"title" validate:"required"`
	LocalizedSubtitle  *LocalizedString       `json:"subtitle,omitempty"`
	LocalizedSortAs    *LocalizedString       `json:"sortAs,omitempty"`
	Accessibility      *A11y                  `json:"accessibility,omitempty"`
	Modified           *time.Time             `json:"modified,omitempty"`
	Published          *time.Time             `json:"published,omitempty"`
	Languages          Strings                `json:"language,omitempty" validate:"BCP47"` // TODO validator
	Subjects           []Subject              `json:"subject,omitempty"`
	Authors            Contributors           `json:"author,omitempty"`
	Translators        Contributors           `json:"translator,omitempty"`
	Editors            Contributors           `json:"editor,omitempty"`
	Artists            Contributors           `json:"artist,omitempty"`
	Illustrators       Contributors           `json:"illustrator,omitempty"`
	Letterers          Contributors           `json:"letterer,omitempty"`
	Pencilers          Contributors           `json:"penciler,omitempty"`
	Colorists          Contributors           `json:"colorist,omitempty"`
	Inkers             Contributors           `json:"inker,omitempty"`
	Narrators          Contributors           `json:"narrator,omitempty"`
	Contributors       Contributors           `json:"contributor,omitempty"`
	Publishers         Contributors           `json:"publisher,omitempty"`
	Imprints           Contributors           `json:"imprint,omitempty"`
	ReadingProgression ReadingProgression     `json:"readingProgression,omitempty" validate:"readingProgression"` // TODO validator.
	Description        string                 `json:"description,omitempty"`
	Duration           *float64               `json:"duration,omitempty" validator:"positive"` // TODO validator
	NumberOfPages      *uint                  `json:"numberOfPages,omitempty"`
	BelongsTo          map[string]Collections `json:"belongsTo,omitempty"`
	Presentation       *Presentation          `json:"presentation,omitempty"`

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

const InferredAccessibilityMetadataKey = "https://readium.org/webpub-manifest#inferredAccessibility"

// InferredAccessibility returns the accessibility metadata inferred from the
// manifest and stored in OtherMetadata.
func (m Metadata) InferredAccessibility() *A11y {
	var a11y *A11y
	if a11yJSON, ok := m.OtherMetadata[InferredAccessibilityMetadataKey].(map[string]interface{}); ok {
		a11y, _ = A11yFromJSON(a11yJSON)
	}
	return a11y
}

// SetOtherMetadata marshalls the value to a JSON map before storing it in
// OtherMetadata under the given key.
func (m Metadata) SetOtherMetadata(key string, value interface{}) error {
	value, err := toJSONMap(value)
	if err != nil {
		return err
	}
	m.OtherMetadata[key] = value
	return nil
}

func toJSONMap(value interface{}) (map[string]interface{}, error) {
	if value, ok := value.(util.JSONMappable); ok {
		return value.JSONMap()
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]interface{}
	err = json.Unmarshal(bytes, &object)
	if err != nil {
		return nil, err
	}
	return object, nil
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

	var a11y *A11y
	if a11yJSON, ok := rawJson["accessibility"].(map[string]interface{}); ok {
		a11y, err = A11yFromJSON(a11yJSON)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing 'accessibility'")
		}
	}

	metadata := &Metadata{
		Identifier:         parseOptString(rawJson["identifier"]),
		Type:               parseOptString(rawJson["@type"]),
		LocalizedTitle:     *title,
		Accessibility:      a11y,
		Modified:           parseOptTime(rawJson["modified"]),
		Published:          parseOptTime(rawJson["published"]),
		ReadingProgression: ReadingProgression(parseOptString(rawJson["readingProgression"])),
		Description:        parseOptString(rawJson["description"]),
	}

	// ConformsTo
	conformsTo, err := parseSliceOrString(rawJson["conformsTo"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'conformsTo'")
	}
	if len(conformsTo) > 0 {
		metadata.ConformsTo = Profiles(profilesFromStrings(conformsTo))
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
	subjects, err := SubjectFromJSONArray(rawJson["subject"], normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing 'subject'")
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
	if ok && duration >= 0 {
		metadata.Duration = &duration
	}

	// NumberOfPages
	numberOfPages, ok := rawJson["numberOfPages"].(float64)
	if ok && numberOfPages >= 0 {
		nop := uint(numberOfPages)
		metadata.NumberOfPages = &nop
	}

	// BelongsTo
	belongsToRaw, ok := rawJson["belongsTo"].(map[string]interface{})
	if !ok {
		belongsToRaw, _ = rawJson["belongs_to"].(map[string]interface{})
	}
	if belongsToRaw != nil {
		belongsTo := make(map[string]Collections)
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

	// Presentation
	// TODO custom presentation unmarshalling

	// Delete above vals so that we can put everything else in OtherMetadata
	for _, v := range []string{
		"@type",
		"accessibility",
		"artist",
		"author",
		"belongsTo",
		"belongs_to",
		"colorist",
		"conformsTo",
		"contributor",
		"description",
		"duration",
		"editor",
		"identifier",
		"illustrator",
		"imprint",
		"inker",
		"language",
		"letterer",
		"modified",
		"narrator",
		"numberOfPages",
		"penciler",
		"presentation",
		"published",
		"publisher",
		"readingProgression",
		"sortAs",
		"subject",
		"subtitle",
		"title",
		"translator",
	} {
		delete(rawJson, v)
	}

	// Now all we have left is everything else!
	if len(rawJson) > 0 {
		metadata.OtherMetadata = rawJson
	}

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

func (m Metadata) MarshalJSON() ([]byte, error) {
	j := make(map[string]interface{})
	if m.OtherMetadata != nil {
		for k, v := range m.OtherMetadata {
			j[k] = v
		}
	}

	if m.Presentation != nil {
		j["presentation"] = m.Presentation
	}

	if m.Identifier != "" {
		j["identifier"] = m.Identifier
	}
	if m.Type != "" {
		j["@type"] = m.Type
	}
	if len(m.ConformsTo) > 0 {
		j["conformsTo"] = m.ConformsTo
	}
	j["title"] = m.LocalizedTitle
	if m.LocalizedSubtitle != nil {
		j["subtitle"] = *m.LocalizedSubtitle
	}
	if m.Accessibility != nil {
		j["accessibility"] = *m.Accessibility
	}
	if m.Modified != nil {
		j["modified"] = *m.Modified
	}
	if m.Published != nil {
		j["published"] = *m.Published
	}
	if len(m.Languages) > 0 {
		j["language"] = m.Languages
	}
	if m.LocalizedSortAs != nil {
		j["sortAs"] = m.LocalizedSortAs
	}
	if len(m.Subjects) > 0 {
		j["subject"] = m.Subjects
	}
	if len(m.Authors) > 0 {
		j["author"] = m.Authors
	}
	if len(m.Translators) > 0 {
		j["translator"] = m.Translators
	}
	if len(m.Editors) > 0 {
		j["editor"] = m.Editors
	}
	if len(m.Artists) > 0 {
		j["artist"] = m.Artists
	}
	if len(m.Illustrators) > 0 {
		j["illustrator"] = m.Illustrators
	}
	if len(m.Letterers) > 0 {
		j["letterer"] = m.Letterers
	}
	if len(m.Pencilers) > 0 {
		j["penciler"] = m.Pencilers
	}
	if len(m.Colorists) > 0 {
		j["colorist"] = m.Colorists
	}
	if len(m.Inkers) > 0 {
		j["inker"] = m.Inkers
	}
	if len(m.Narrators) > 0 {
		j["narrator"] = m.Narrators
	}
	if len(m.Contributors) > 0 {
		j["contributor"] = m.Contributors
	}
	if len(m.Publishers) > 0 {
		j["publisher"] = m.Publishers
	}
	if len(m.Imprints) > 0 {
		j["imprint"] = m.Imprints
	}
	if m.ReadingProgression != "" && m.ReadingProgression != Auto {
		j["readingProgression"] = m.ReadingProgression
	}
	if m.Description != "" {
		j["description"] = m.Description
	}
	if m.Duration != nil {
		j["duration"] = m.Duration
	}
	if m.NumberOfPages != nil {
		j["numberOfPages"] = m.NumberOfPages
	}
	if len(m.BelongsTo) > 0 {
		j["belongsTo"] = m.BelongsTo
	}

	return json.Marshal(j)
}
