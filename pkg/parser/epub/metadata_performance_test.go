package epub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A subtitle can double as the publication title when no main title exists,
// but editing either output field must not change the other one.
func TestMetadataTitleSubtitleTranslationsIndependent(t *testing.T) {
	adapter := PubMetadataAdapter{
		metadataAdapter: metadataAdapter{
			items: map[string][]MetadataItem{
				VocabularyDCTerms + "title": {{
					property: VocabularyDCTerms + "title",
					value:    "Subtitle only",
					children: map[string][]MetadataItem{
						VocabularyMeta + "title-type": {{value: "subtitle"}},
					},
				}},
			},
		},
	}
	metadata := adapter.Metadata()
	require.NotNil(t, metadata.LocalizedSubtitle)
	assert.Equal(t, "Subtitle only", metadata.LocalizedTitle.String())
	assert.Equal(t, "Subtitle only", metadata.LocalizedSubtitle.String())

	metadata.LocalizedTitle.SetDefaultTranslation("Edited title")
	assert.Equal(t, "Subtitle only", metadata.LocalizedSubtitle.String())
	metadata.LocalizedSubtitle.SetTranslation("fr", "Sous-titre")
	assert.NotContains(t, metadata.LocalizedTitle.Translations, "fr")

	// The optimization seeds only the value receiver's local copy. Later
	// conversions must continue to produce fresh, independent metadata.
	fresh := adapter.Metadata()
	assert.Equal(t, manifest.NewLocalizedStringFromString("Subtitle only"), fresh.LocalizedTitle)
	assert.Equal(t, manifest.NewLocalizedStringFromString("Subtitle only"), *fresh.LocalizedSubtitle)
}
