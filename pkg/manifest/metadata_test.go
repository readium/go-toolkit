package manifest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	ToolkitVersionKey = "" // Prevent injection of version during testing
}

func TestMetadataUnmarshalMinimalJSON(t *testing.T) {
	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{"title": "Title"}`), &m))
	assert.Equal(t, Metadata{LocalizedTitle: NewLocalizedStringFromString("Title")}, m, "parsed JSON object should be equal to Metadata object")
}

func TestMetadataUnmarshalFullJSON(t *testing.T) {
	var m Metadata
	lst := NewLocalizedStringFromStrings(map[string]string{
		"en": "Subtitle",
		"fr": "Sous-titre",
	})
	lsa := NewLocalizedStringFromString("sort key")
	modified, err := time.Parse(time.RFC3339Nano, "2001-01-01T12:36:27.000Z")
	require.NoError(t, err)
	published, err := time.Parse(time.RFC3339Nano, "2001-01-02T12:36:27.000Z")
	require.NoError(t, err)
	duration := float64(4.24)
	numberOfPages := uint(240)
	a11y := NewA11y()
	a11y.ConformsTo = []A11yProfile{EPUBA11y10WCAG20A}

	require.NoError(t, json.Unmarshal([]byte(`{
		"identifier": "1234",
		"altIdentifier": ["https://example.com/identifier/12345", {
			"value": "123456789",
			"scheme": "https://example.com/identifierScheme"
		}],
		"@type": "epub",
		"conformsTo": [
			"https://readium.org/webpub-manifest/profiles/epub",
			"https://readium.org/webpub-manifest/profiles/pdf"
		],
		"title": {"en": "Title", "fr": "Titre"},
		"subtitle": {"en": "Subtitle", "fr": "Sous-titre"},
		"accessibility": {"conformsTo": "http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a"},
		"tdm": {"policy": "https://provider.com/policies/policy.json", "reservation": "all"},
		"modified": "2001-01-01T12:36:27.000Z",
		"published": "2001-01-02T12:36:27.000Z",
		"language": ["en", "fr"],
		"sortAs": "sort key",
		"subject": ["Science Fiction", "Fantasy"],
		"author": "Author",
		"translator": "Translator",
		"editor": "Editor",
		"artist": "Artist",
		"illustrator": "Illustrator",
		"letterer": "Letterer",
		"penciler": "Penciler",
		"colorist": "Colorist",
		"inker": "Inker",
		"narrator": "Narrator",
		"contributor": "Contributor",
		"publisher": "Publisher",
		"imprint": "Imprint",
		"readingProgression": "rtl",
		"layout": "fixed",
		"description": "Description",
		"duration": 4.24,
		"numberOfPages": 240,
		"belongsTo": {
			"collection": "Collection",
			"series": "Series",
			"schema:Periodical": "Periodical",
			"schema:Newspaper": [ "Newspaper 1", "Newspaper 2" ]
		},
		"other-metadata1": "value",
		"other-metadata2": [42]
	}`), &m))

	assert.Equal(t, Metadata{
		Identifier: "1234",
		AltIdentifiers: []AltIdentifier{
			{
				Value: "https://example.com/identifier/12345",
			},
			{
				Value:  "123456789",
				Scheme: "https://example.com/identifierScheme",
			},
		},
		Type:       "epub",
		ConformsTo: Profiles{ProfileEPUB, ProfilePDF},
		LocalizedTitle: NewLocalizedStringFromStrings(map[string]string{
			"en": "Title",
			"fr": "Titre",
		}),
		LocalizedSubtitle: &lst,
		Accessibility:     &a11y,
		TDM: &TDM{
			Policy:      "https://provider.com/policies/policy.json",
			Reservation: TDMReservationAll,
		},
		Modified:           &modified,
		Published:          &published,
		Languages:          []string{"en", "fr"},
		LocalizedSortAs:    &lsa,
		Subjects:           []Subject{{LocalizedName: NewLocalizedStringFromString("Science Fiction")}, {LocalizedName: NewLocalizedStringFromString("Fantasy")}},
		Authors:            Contributors{{LocalizedName: NewLocalizedStringFromString("Author")}},
		Translators:        Contributors{{LocalizedName: NewLocalizedStringFromString("Translator")}},
		Editors:            Contributors{{LocalizedName: NewLocalizedStringFromString("Editor")}},
		Artists:            Contributors{{LocalizedName: NewLocalizedStringFromString("Artist")}},
		Illustrators:       Contributors{{LocalizedName: NewLocalizedStringFromString("Illustrator")}},
		Letterers:          Contributors{{LocalizedName: NewLocalizedStringFromString("Letterer")}},
		Pencilers:          Contributors{{LocalizedName: NewLocalizedStringFromString("Penciler")}},
		Colorists:          Contributors{{LocalizedName: NewLocalizedStringFromString("Colorist")}},
		Inkers:             Contributors{{LocalizedName: NewLocalizedStringFromString("Inker")}},
		Narrators:          Contributors{{LocalizedName: NewLocalizedStringFromString("Narrator")}},
		Contributors:       Contributors{{LocalizedName: NewLocalizedStringFromString("Contributor")}},
		Publishers:         Contributors{{LocalizedName: NewLocalizedStringFromString("Publisher")}},
		Imprints:           Contributors{{LocalizedName: NewLocalizedStringFromString("Imprint")}},
		ReadingProgression: RTL,
		Layout:             LayoutNone, // Not fixed, because layout got corrected
		Description:        "Description",
		Duration:           &duration,
		NumberOfPages:      &numberOfPages,
		BelongsTo: map[string]Collections{
			"schema:Periodical": {{LocalizedName: NewLocalizedStringFromString("Periodical")}},
			"schema:Newspaper": {
				{LocalizedName: NewLocalizedStringFromString("Newspaper 1")},
				{LocalizedName: NewLocalizedStringFromString("Newspaper 2")},
			},
			"collection": {{LocalizedName: NewLocalizedStringFromString("Collection")}},
			"series":     {{LocalizedName: NewLocalizedStringFromString("Series")}},
		},
		OtherMetadata: map[string]interface{}{
			"other-metadata1": "value",
			"other-metadata2": []interface{}{float64(42)},
		},
	}, m, "parsed JSON object should be equal to Metadata object")
}

func TestMetadataUnmarshalNilJSON(t *testing.T) {
	s, err := MetadataFromJSON(nil)
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestMetadataUnmarshalJSONSingleProfile(t *testing.T) {
	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{
		"title": "Title",
		"conformsTo": "https://readium.org/webpub-manifest/profiles/epub"
	}`), &m))
	assert.Equal(t, Metadata{
		LocalizedTitle: NewLocalizedStringFromString("Title"),
		ConformsTo:     Profiles{ProfileEPUB},
	}, m, "single profile in Metadata should parse correctly")
}

func TestMetadataUnmarshalJSONSingleLanguage(t *testing.T) {
	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{
		"title": "Title",
		"language": "fr"
	}`), &m))
	assert.Equal(t, Metadata{
		LocalizedTitle: NewLocalizedStringFromString("Title"),
		Languages:      []string{"fr"},
	}, m, "single language in Metadata should parse correctly")
}

func TestMetadataUnmarshalJSONRequiresTitle(t *testing.T) {
	var m Metadata
	assert.Error(t, json.Unmarshal([]byte(`{"duration": "4.24"}`), &m))
}

func TestMetadataUnmarshalJSONDurationPositive(t *testing.T) {
	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{"title": "Title", "duration": -20}`), &m))
	assert.Equal(t, Metadata{
		LocalizedTitle: NewLocalizedStringFromString("Title"),
	}, m)
}

func TestMetadataUnmarshalJSONNumberOfPagesPositive(t *testing.T) {
	var m Metadata
	require.NoError(t, json.Unmarshal([]byte(`{"title": "Title", "numberOfPages": -20}`), &m))
	assert.Equal(t, Metadata{
		LocalizedTitle: NewLocalizedStringFromString("Title"),
	}, m)
}

func TestMetadataMinimalJSON(t *testing.T) {
	b, err := json.Marshal(Metadata{
		LocalizedTitle: NewLocalizedStringFromString("Title"),
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"title": "Title"}`, string(b))
}

func TestMetadataFullJSON(t *testing.T) {
	lst := NewLocalizedStringFromStrings(map[string]string{
		"en": "Subtitle",
		"fr": "Sous-titre",
	})
	modified, err := time.Parse(time.RFC3339Nano, "2001-01-01T12:36:27.123Z")
	require.NoError(t, err)
	published, err := time.Parse(time.RFC3339Nano, "2001-01-02T12:36:27.000Z")
	require.NoError(t, err)
	lsa := NewLocalizedStringFromStrings(map[string]string{
		"en": "sort key",
		"fr": "clé de tri",
	})
	duration := float64(4.24)
	numberOfPages := uint(240)
	a11y := NewA11y()
	a11y.ConformsTo = []A11yProfile{EPUBA11y10WCAG20AA}

	b, err := json.Marshal(Metadata{
		Identifier: "1234",
		AltIdentifiers: []AltIdentifier{
			{
				Value: "https://example.com/identifier/12345",
			},
			{
				Value:  "123456789",
				Scheme: "https://example.com/identifierScheme",
			},
		},
		Type:       "epub",
		ConformsTo: Profiles{ProfileEPUB, ProfilePDF},
		LocalizedTitle: NewLocalizedStringFromStrings(map[string]string{
			"en": "Title",
			"fr": "Titre",
		}),
		LocalizedSubtitle: &lst,
		Accessibility:     &a11y,
		TDM: &TDM{
			Policy:      "https://provider.com/policies/policy.json",
			Reservation: TDMReservationAll,
		},
		Modified:           &modified,
		Published:          &published,
		Languages:          []string{"en", "fr"},
		LocalizedSortAs:    &lsa,
		Subjects:           []Subject{{LocalizedName: NewLocalizedStringFromString("Science Fiction")}, {LocalizedName: NewLocalizedStringFromString("Fantasy")}},
		Authors:            Contributors{{LocalizedName: NewLocalizedStringFromString("Author")}},
		Translators:        Contributors{{LocalizedName: NewLocalizedStringFromString("Translator")}},
		Editors:            Contributors{{LocalizedName: NewLocalizedStringFromString("Editor")}},
		Artists:            Contributors{{LocalizedName: NewLocalizedStringFromString("Artist")}},
		Illustrators:       Contributors{{LocalizedName: NewLocalizedStringFromString("Illustrator")}},
		Letterers:          Contributors{{LocalizedName: NewLocalizedStringFromString("Letterer")}},
		Pencilers:          Contributors{{LocalizedName: NewLocalizedStringFromString("Penciler")}},
		Colorists:          Contributors{{LocalizedName: NewLocalizedStringFromString("Colorist")}},
		Inkers:             Contributors{{LocalizedName: NewLocalizedStringFromString("Inker")}},
		Narrators:          Contributors{{LocalizedName: NewLocalizedStringFromString("Narrator")}},
		Contributors:       Contributors{{LocalizedName: NewLocalizedStringFromString("Contributor")}},
		Publishers:         Contributors{{LocalizedName: NewLocalizedStringFromString("Publisher")}},
		Imprints:           Contributors{{LocalizedName: NewLocalizedStringFromString("Imprint")}},
		ReadingProgression: RTL,
		Layout:             LayoutFixed,
		Description:        "Description",
		Duration:           &duration,
		NumberOfPages:      &numberOfPages,
		BelongsTo: map[string]Collections{
			"schema:Periodical": {{LocalizedName: NewLocalizedStringFromString("Periodical")}},
			"schema:Newspaper": {
				{LocalizedName: NewLocalizedStringFromString("Newspaper 1")},
				{LocalizedName: NewLocalizedStringFromString("Newspaper 2")},
			},
			"collection": {{LocalizedName: NewLocalizedStringFromString("Collection")}},
			"series":     {{LocalizedName: NewLocalizedStringFromString("Series")}},
		},
		OtherMetadata: map[string]interface{}{
			"other-metadata1": "value",
			"other-metadata2": []interface{}{float64(42)},
		},
	})
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"identifier": "1234",
		"altIdentifier": ["https://example.com/identifier/12345", {
			"value": "123456789",
			"scheme": "https://example.com/identifierScheme"
		}],
		"@type": "epub",
		"conformsTo": [
			"https://readium.org/webpub-manifest/profiles/epub",
			"https://readium.org/webpub-manifest/profiles/pdf"
		],
		"title": {"en": "Title", "fr": "Titre"},
		"subtitle": {"en": "Subtitle", "fr": "Sous-titre"},
		"accessibility": {"conformsTo": ["http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aa"]},
		"tdm": {"policy": "https://provider.com/policies/policy.json", "reservation": "all"},
		"modified": "2001-01-01T12:36:27.123Z",
		"published": "2001-01-02T12:36:27Z",
		"language": ["en", "fr"],
		"sortAs": {"en": "sort key", "fr": "clé de tri"},
		"subject": [
			"Science Fiction",
			"Fantasy"
		],
		"author": "Author",
		"translator": "Translator",
		"editor": "Editor",
		"artist": "Artist",
		"illustrator": "Illustrator",
		"letterer": "Letterer",
		"penciler": "Penciler",
		"colorist": "Colorist",
		"inker": "Inker",
		"narrator": "Narrator",
		"contributor": "Contributor",
		"publisher": "Publisher",
		"imprint": "Imprint",
		"readingProgression": "rtl",
		"description": "Description",
		"duration": 4.24,
		"numberOfPages": 240,
		"belongsTo": {
			"collection": "Collection",
			"series": "Series",
			"schema:Periodical": "Periodical",
			"schema:Newspaper": [ "Newspaper 1", "Newspaper 2" ]
		},
		"other-metadata1": "value",
		"other-metadata2": [42]
	}`, string(b))
}

func TestMetadataERPFallsBackToLTR(t *testing.T) {
	assert.Equal(t, LTR, Metadata{
		Languages:          []string{},
		ReadingProgression: None,
	}.EffectiveReadingProgression())
}

func TestMetadataERPFallsBackToProvided(t *testing.T) {
	assert.Equal(t, RTL, Metadata{
		Languages:          []string{},
		ReadingProgression: RTL,
	}.EffectiveReadingProgression())
}

func TestMetadataERPWithRTLLanguages(t *testing.T) {
	assert.Equal(t, RTL, Metadata{Languages: []string{"zh-Hant"}, ReadingProgression: None}.EffectiveReadingProgression())
	assert.Equal(t, RTL, Metadata{Languages: []string{"zh-TW"}, ReadingProgression: None}.EffectiveReadingProgression())
	assert.Equal(t, RTL, Metadata{Languages: []string{"ar"}, ReadingProgression: None}.EffectiveReadingProgression())
	assert.Equal(t, RTL, Metadata{Languages: []string{"fa"}, ReadingProgression: None}.EffectiveReadingProgression())
	assert.Equal(t, RTL, Metadata{Languages: []string{"he"}, ReadingProgression: None}.EffectiveReadingProgression())
	assert.Equal(t, LTR, Metadata{Languages: []string{"he"}, ReadingProgression: LTR}.EffectiveReadingProgression())
}

func TestMetadataERPIgnoresMultipleLanguages(t *testing.T) {
	assert.Equal(t, LTR, Metadata{
		Languages:          []string{"ar", "fa"},
		ReadingProgression: None,
	}.EffectiveReadingProgression())
}

func TestMetdataERPIgnoresLanguageCase(t *testing.T) {
	assert.Equal(t, RTL, Metadata{
		Languages:          []string{"AR"},
		ReadingProgression: None,
	}.EffectiveReadingProgression())
}

func TestMetadataERPIgnoresLanguageRegionExceptChinese(t *testing.T) {
	assert.Equal(t, RTL, Metadata{
		Languages:          []string{"ar-foo"},
		ReadingProgression: None,
	}.EffectiveReadingProgression())

	// But not for ZH
	assert.Equal(t, LTR, Metadata{
		Languages:          []string{"zh-foo"},
		ReadingProgression: None,
	}.EffectiveReadingProgression())
}
