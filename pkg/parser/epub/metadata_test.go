package epub

import (
	"os"
	"testing"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func loadMetadata(name string) (*manifest.Metadata, error) {
	r, err := os.Open("./testdata/package/" + name + ".opf")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	n, err := xmlquery.Parse(r)
	if err != nil {
		return nil, err
	}

	d, err := ParsePackageDocument(n, "")
	if err != nil {
		return nil, err
	}

	manifest := PublicationFactory{
		FallbackTitle:   "fallback title",
		PackageDocument: *d,
	}.Create()

	return &manifest.Metadata, nil
}

func TestMetadataContributorDCCreatorDefaultsToAuthor(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Author 1"),
	}
	assert.Contains(t, m2.Authors, contributor)
	assert.Contains(t, m3.Authors, contributor)
}

func TestMetadataContributorDCPublisherIsPublisher(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Publisher 1"),
	}
	assert.Contains(t, m2.Publishers, contributor)
	assert.Contains(t, m3.Publishers, contributor)
}

func TestMetadataContributorDCContributorDefaultsToContributor(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Contributor 1"),
	}
	assert.Contains(t, m2.Contributors, contributor)
	assert.Contains(t, m3.Contributors, contributor)
}

func TestMetadataContributorUnknownRolesIgnored(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Contributor 2"),
		Roles:         manifest.Strings{"unknown"},
	}
	assert.Contains(t, m2.Contributors, contributor)
	assert.Contains(t, m3.Contributors, contributor)
}

func TestMetadataContributorFileAsParsed(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	lsa := manifest.NewLocalizedStringFromString("Sorting Key")
	contributor := manifest.Contributor{
		LocalizedName:   manifest.NewLocalizedStringFromString("Contributor 3"),
		LocalizedSortAs: &lsa,
	}
	assert.Contains(t, m2.Contributors, contributor)
	assert.Contains(t, m3.Contributors, contributor)
}

func TestMetadataContributorLocalizedParsed(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	assert.Contains(t, m3.Contributors, manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			manifest.UndefinedLanguage: "Contributor 4",
			"fr":                       "Contributeur 4 en français",
		}),
	})
}

func TestMetadataContributorOnlyFirstRoleConsidered(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Cameleon"),
	}

	assert.Contains(t, m3.Authors, contributor)
	assert.NotContains(t, m3.Publishers, contributor)
}

func TestMetadataContributorMediaOverlaysNarrator(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	assert.Contains(t, m3.Narrators, manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Media Overlays Narrator"),
	})
}

func TestMetadataContributorAuthor(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Author 2"),
	}

	assert.Contains(t, m2.Authors, contributor)
	assert.Contains(t, m3.Authors, contributor)
}

func TestMetadataContributorPublisher(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Publisher 2"),
	}

	assert.Contains(t, m2.Publishers, contributor)
	assert.Contains(t, m3.Publishers, contributor)
}

func TestMetadataContributorTranslator(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Translator"),
	}

	assert.Contains(t, m2.Translators, contributor)
	assert.Contains(t, m3.Translators, contributor)
}

func TestMetadataContributorArtist(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Artist"),
	}

	assert.Contains(t, m2.Artists, contributor)
	assert.Contains(t, m3.Artists, contributor)
}

func TestMetadataContributorIllustrator(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Illustrator"),
	}

	assert.Contains(t, m2.Illustrators, contributor)
	assert.Contains(t, m3.Illustrators, contributor)
}

func TestMetadataContributorColorist(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Colorist"),
	}

	assert.Contains(t, m2.Colorists, contributor)
	assert.Contains(t, m3.Colorists, contributor)
}

func TestMetadataContributorNarrator(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Narrator"),
	}

	assert.Contains(t, m2.Narrators, contributor)
	assert.Contains(t, m3.Narrators, contributor)
}

func TestMetadataContributorsNoMoreThanNeeded(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	assert.Len(t, m2.Authors, 2)
	assert.Len(t, m2.Publishers, 2)
	assert.Len(t, m2.Translators, 1)
	assert.Len(t, m2.Editors, 1)
	assert.Len(t, m2.Artists, 1)
	assert.Len(t, m2.Illustrators, 1)
	assert.Len(t, m2.Colorists, 1)
	assert.Len(t, m2.Narrators, 1)
	assert.Len(t, m2.Contributors, 3)

	assert.Len(t, m3.Authors, 3)
	assert.Len(t, m3.Publishers, 2)
	assert.Len(t, m3.Translators, 1)
	assert.Len(t, m3.Editors, 1)
	assert.Len(t, m3.Artists, 1)
	assert.Len(t, m3.Illustrators, 1)
	assert.Len(t, m3.Colorists, 1)
	assert.Len(t, m3.Narrators, 2)
	assert.Len(t, m3.Contributors, 4)
}

func TestMetadataTitleParsed(t *testing.T) {
	m2, err := loadMetadata("titles-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("titles-epub3")
	assert.NoError(t, err)

	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en": "Alice's Adventures in Wonderland",
	}), m2.LocalizedTitle)
	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en": "Alice's Adventures in Wonderland",
		"fr": "Les Aventures d'Alice au pays des merveilles",
	}), m3.LocalizedTitle)
}

func TestMetadataTitleSubtitleParsed(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("titles-epub3")
	assert.NoError(t, err)

	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en-GB": "Alice returns to the magical world from her childhood adventure",
		"fr":    "Alice retourne dans le monde magique des aventures de son enfance",
	}), *m3.LocalizedSubtitle)
}

func TestMetadataTitleFileAs(t *testing.T) {
	m2, err := loadMetadata("titles-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("titles-epub3")
	assert.NoError(t, err)

	assert.Equal(t, "Adventures", m2.SortAs())
	assert.Equal(t, "Adventures", m3.SortAs())
}

func TestMetadataTitleMainTakesPrecedence(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("title-main-precedence")
	assert.NoError(t, err)

	assert.Equal(t, "Main title takes precedence", m3.Title())
}

func TestMetadataTitleSelectedSubtitleHasLowestDisplaySeqProperty(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("title-multiple-subtitles")
	assert.NoError(t, err)

	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en": "Subtitle 2",
	}), *m3.LocalizedSubtitle)
}

func TestMetadataSubjectLocalized(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("subjects-complex")
	assert.NoError(t, err)

	assert.Len(t, m3.Subjects, 1)
	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en": "FICTION / Occult & Supernatural",
		"fr": "FICTION / Occulte & Surnaturel",
	}), m3.Subjects[0].LocalizedName)
}

func TestMetadataSubjectFileAs(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("subjects-complex")
	assert.NoError(t, err)

	assert.Len(t, m3.Subjects, 1)
	assert.Equal(t, "occult", m3.Subjects[0].SortAs())
}

func TestMetadataSubjectCodeAndScheme(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("subjects-complex")
	assert.NoError(t, err)

	assert.Len(t, m3.Subjects, 1)
	assert.Equal(t, "BISAC", m3.Subjects[0].Scheme)
	assert.Equal(t, "FIC024000", m3.Subjects[0].Code)
}

func TestMetadataSubjectCommaSeparatedSplit(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("subjects-single")
	assert.NoError(t, err)

	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("apple")})
	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("banana")})
	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("pear")})
}

func TestMetadataSubjectCommaSeparatedMultipleNotSplit(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("subjects-multiple")
	assert.NoError(t, err)

	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("fiction")})
	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("apple; banana,  pear")})
}

func TestMetadataDatePublished(t *testing.T) {
	m2, err := loadMetadata("dates-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("dates-epub3")
	assert.NoError(t, err)

	tx, err := time.Parse(time.RFC3339, "1865-07-04T00:00:00Z")
	assert.NoError(t, err)

	assert.Equal(t, &tx, m2.Published)
	assert.Equal(t, &tx, m3.Published)
}

func TestMetadataDateModified(t *testing.T) {
	m2, err := loadMetadata("dates-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("dates-epub3")
	assert.NoError(t, err)

	tx, err := time.Parse(time.RFC3339, "2012-04-02T12:47:00Z")
	assert.NoError(t, err)

	assert.Equal(t, &tx, m2.Modified)
	assert.Equal(t, &tx, m3.Modified)
}

func TestMetadataConformsToProfileEPUB(t *testing.T) {
	m2, err := loadMetadata("contributors-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("contributors-epub3")
	assert.NoError(t, err)

	assert.Contains(t, m2.ConformsTo, manifest.ProfileEPUB)
	assert.Contains(t, m3.ConformsTo, manifest.ProfileEPUB)
}

func TestMetadataUniqueIdentifierParsed(t *testing.T) {
	m3, err := loadMetadata("identifier-unique")
	assert.NoError(t, err)

	assert.Equal(t, "urn:uuid:2", m3.Identifier)
}

func TestMetadataRenditionProperties(t *testing.T) {
	m3, err := loadMetadata("presentation-metadata")
	assert.NoError(t, err)
	if assert.NotNil(t, m3.Presentation) {
		assert.Equal(t, false, *m3.Presentation.Continuous)
		assert.Equal(t, manifest.OverflowScrolled, *m3.Presentation.Overflow)
		assert.Equal(t, manifest.SpreadBoth, *m3.Presentation.Spread)
		assert.Equal(t, manifest.OrientationLandscape, *m3.Presentation.Orientation)
		assert.Equal(t, manifest.EPUBLayoutFixed, *m3.Presentation.Layout)
	}
}

func TestMetadataCoverLink(t *testing.T) {
	// Note: not using loadMetadata
	m2, err := loadPackageDoc("cover-epub2")
	assert.NoError(t, err)
	m3, err := loadPackageDoc("cover-epub3")
	assert.NoError(t, err)
	mm, err := loadPackageDoc("cover-mix")
	assert.NoError(t, err)

	expected := &manifest.Link{
		Href: "/OEBPS/cover.jpg",
		Type: "image/jpeg",
		Rels: []string{"cover"},
	}
	assert.Equal(t, m2.Resources.FirstWithRel("cover"), expected)
	assert.Equal(t, m3.Resources.FirstWithRel("cover"), expected)
	assert.Equal(t, mm.Resources.FirstWithRel("cover"), expected)
}

func TestMetadataCrossRefinings(t *testing.T) {
	_, err := loadPackageDoc("meta-termination")
	assert.NoError(t, err)
}

func TestMetadataOtherMetadata(t *testing.T) {
	m3, err := loadMetadata("meta-others")
	assert.NoError(t, err)

	assert.Equal(t, m3.OtherMetadata, map[string]interface{}{
		VocabularyDCTerms + "source": []interface{}{
			"Feedbooks",
			map[string]interface{}{"@value": "Web", "http://my.url/#scheme": "http"},
			"Internet",
		},
		"http://my.url/#property0": map[string]interface{}{
			"@value": "refines0",
			"http://my.url/#property1": map[string]interface{}{
				"@value":                   "refines1",
				"http://my.url/#property2": "refines2",
				"http://my.url/#property3": "refines3",
			},
		},
	})
}

func TestMetadataCollectionBasic(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("collections-epub3")
	assert.NoError(t, err)

	assert.Contains(t, m3.BelongsToCollections(), manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Collection B",
		}),
	})
}

func TestMetadataCollectionsWithUnknownTypeInBelongsTo(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("collections-epub3")
	assert.NoError(t, err)

	assert.Contains(t, m3.BelongsToCollections(), manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Collection A",
		}),
	})
}

func TestMetadataCollectionLocalizedSeries(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata("collections-epub3")
	assert.NoError(t, err)

	assert.Contains(t, m3.BelongsToSeries(), manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Series A",
			"fr": "Série A",
		}),
		Identifier: "ser-a",
		Position:   floatP(2.0),
	})
}

func TestMetadataCollectionSeriesWithPosition(t *testing.T) {
	m2, err := loadMetadata("collections-epub2")
	assert.NoError(t, err)
	m3, err := loadMetadata("collections-epub3")
	assert.NoError(t, err)

	expected := manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Series B",
		}),
		Position: floatP(1.5),
	}

	assert.Contains(t, m2.BelongsToSeries(), expected)
	assert.Contains(t, m3.BelongsToSeries(), expected)
}
