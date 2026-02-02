package epub

import (
	"context"
	"testing"
	"time"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadMetadata(ctx context.Context, name string) (*manifest.Metadata, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/package/"+name+".opf"), map[string]string{
		NamespaceOPF:                         "opf",
		NamespaceDC:                          "dc",
		VocabularyDCTerms:                    "dcterms",
		"http://www.idpf.org/2013/rendition": "rendition",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	d, err := ParsePackageDocument(n, url.MustURLFromString(""))
	if err != nil {
		return nil, err
	}

	manifest := PublicationFactory{
		FallbackTitle:   "fallback title",
		PackageDocument: *d,
	}.Create()

	/*if manifest.Metadata.Identifier == "9782346140824" {
		mnod := n.SelectElement(
			"/" + NSSelect(NamespaceOPF, "package") + "/" + NSSelect(NamespaceOPF, "metadata"),
		)
		mtit := mnod.SelectElement("/dc:title")
		println("DATA", mtit.InnerText())
		println(mtit.OutputXML(true))
	}*/

	return &manifest.Metadata, nil
}

func TestMetadataContributorDCCreatorDefaultsToAuthor(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Author 1"),
	}
	assert.Contains(t, m2.Authors, contributor)
	assert.Contains(t, m3.Authors, contributor)
}

func TestMetadataContributorDCPublisherIsPublisher(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Publisher 1"),
	}
	assert.Contains(t, m2.Publishers, contributor)
	assert.Contains(t, m3.Publishers, contributor)
}

func TestMetadataContributorDCContributorDefaultsToContributor(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Contributor 1"),
	}
	assert.Contains(t, m2.Contributors, contributor)
	assert.Contains(t, m3.Contributors, contributor)
}

func TestMetadataContributorUnknownRolesIgnored(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Contributor 2"),
		Roles:         manifest.Strings{"unknown"},
	}
	assert.Contains(t, m2.Contributors, contributor)
	assert.Contains(t, m3.Contributors, contributor)
}

func TestMetadataContributorFileAsParsed(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

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
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	assert.Contains(t, m3.Contributors, manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			manifest.UndefinedLanguage: "Contributor 4",
			"fr":                       "Contributeur 4 en français",
		}),
	})
}

func TestMetadataContributorOnlyFirstRoleConsidered(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Cameleon"),
	}

	assert.Contains(t, m3.Authors, contributor)
	assert.NotContains(t, m3.Publishers, contributor)
}

func TestMetadataContributorMediaOverlaysNarrator(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	assert.Contains(t, m3.Narrators, manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Media Overlays Narrator"),
	})
}

func TestMetadataContributorAuthor(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Author 2"),
	}

	assert.Contains(t, m2.Authors, contributor)
	assert.Contains(t, m3.Authors, contributor)
}

func TestMetadataContributorPublisher(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Publisher 2"),
	}

	assert.Contains(t, m2.Publishers, contributor)
	assert.Contains(t, m3.Publishers, contributor)
}

func TestMetadataContributorTranslator(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Translator"),
	}

	assert.Contains(t, m2.Translators, contributor)
	assert.Contains(t, m3.Translators, contributor)
}

func TestMetadataContributorArtist(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Artist"),
	}

	assert.Contains(t, m2.Artists, contributor)
	assert.Contains(t, m3.Artists, contributor)
}

func TestMetadataContributorIllustrator(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Illustrator"),
	}

	assert.Contains(t, m2.Illustrators, contributor)
	assert.Contains(t, m3.Illustrators, contributor)
}

func TestMetadataContributorColorist(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Colorist"),
	}

	assert.Contains(t, m2.Colorists, contributor)
	assert.Contains(t, m3.Colorists, contributor)
}

func TestMetadataContributorNarrator(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	contributor := manifest.Contributor{
		LocalizedName: manifest.NewLocalizedStringFromString("Narrator"),
	}

	assert.Contains(t, m2.Narrators, contributor)
	assert.Contains(t, m3.Narrators, contributor)
}

func TestMetadataContributorsNoMoreThanNeeded(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

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
	m2, err := loadMetadata(t.Context(), "titles-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "titles-epub3")
	require.NoError(t, err)

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
	m3, err := loadMetadata(t.Context(), "titles-epub3")
	require.NoError(t, err)

	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en-GB": "Alice returns to the magical world from her childhood adventure",
		"fr":    "Alice retourne dans le monde magique des aventures de son enfance",
	}), *m3.LocalizedSubtitle)
}

func TestMetadataNoAccessibility(t *testing.T) {
	m, err := loadMetadata(t.Context(), "version-default")
	require.NoError(t, err)
	assert.Nil(t, m.Accessibility)
}

func TestMetadataEPUB2Accessibility(t *testing.T) {
	m, err := loadMetadata(t.Context(), "accessibility-epub2")
	require.NoError(t, err)
	e := manifest.NewA11y()
	e.ConformsTo = []manifest.A11yProfile{manifest.EPUBA11y11WCAG21AA, manifest.EPUBA11y11WCAG20AAA, manifest.EPUBA11y10WCAG20A}
	e.Certification = &manifest.A11yCertification{
		CertifiedBy: "Accessibility Testers Group",
		Credential:  "DAISY OK",
		Report:      "https://example.com/a11y-report/",
	}
	e.Summary = "The publication contains structural and page navigation."
	e.AccessModes = []manifest.A11yAccessMode{manifest.A11yAccessModeTextual, manifest.A11yAccessModeVisual}
	e.AccessModesSufficient = [][]manifest.A11yPrimaryAccessMode{
		{manifest.A11yPrimaryAccessModeTextual},
		{manifest.A11yPrimaryAccessModeTextual, manifest.A11yPrimaryAccessModeVisual},
	}
	e.Features = []manifest.A11yFeature{manifest.A11yFeatureStructuralNavigation, manifest.A11yFeatureAlternativeText}
	e.Hazards = []manifest.A11yHazard{manifest.A11yHazardMotionSimulation, manifest.A11yHazardNoSoundHazard}
	e.Exemptions = []manifest.A11yExemption{manifest.A11yExemptionEAAMicroenterprise, manifest.A11yExemptionEAAFundamentalAlteration, manifest.A11yExemptionEAADisproportionateBurden}
	assert.Equal(t, &e, m.Accessibility)
	assert.Nil(t, m.OtherMetadata["accessibility"])
}

func TestMetadataEPUB2TDM(t *testing.T) {
	m, err := loadMetadata(t.Context(), "tdm-epub2")
	require.NoError(t, err)
	assert.Equal(t, &manifest.TDM{
		Policy:      "https://provider.com/policies/policy.json",
		Reservation: manifest.TDMReservationAll,
	}, m.TDM)
}

func TestMetadataEPUB3Accessibility(t *testing.T) {
	m, err := loadMetadata(t.Context(), "accessibility-epub3")
	require.NoError(t, err)
	e := manifest.NewA11y()
	e.ConformsTo = []manifest.A11yProfile{manifest.EPUBA11y11WCAG21AA, manifest.EPUBA11y11WCAG20AAA, manifest.EPUBA11y10WCAG20A}
	e.Certification = &manifest.A11yCertification{
		CertifiedBy: "Accessibility Testers Group",
		Credential:  "DAISY OK",
		Report:      "https://example.com/a11y-report/",
	}
	e.Summary = "The publication contains structural and page navigation."
	e.AccessModes = []manifest.A11yAccessMode{manifest.A11yAccessModeTextual, manifest.A11yAccessModeVisual}
	e.AccessModesSufficient = [][]manifest.A11yPrimaryAccessMode{
		{manifest.A11yPrimaryAccessModeTextual},
		{manifest.A11yPrimaryAccessModeTextual, manifest.A11yPrimaryAccessModeVisual},
	}
	e.Features = []manifest.A11yFeature{manifest.A11yFeatureStructuralNavigation, manifest.A11yFeatureAlternativeText}
	e.Hazards = []manifest.A11yHazard{manifest.A11yHazardMotionSimulation, manifest.A11yHazardNoSoundHazard}
	e.Exemptions = []manifest.A11yExemption{manifest.A11yExemptionEAAMicroenterprise, manifest.A11yExemptionEAAFundamentalAlteration, manifest.A11yExemptionEAADisproportionateBurden}
	assert.Equal(t, &e, m.Accessibility)
	assert.Nil(t, m.OtherMetadata["accessibility"])
}

func TestMetadataEPUB3AccessibilityRefines(t *testing.T) {
	m, err := loadMetadata(t.Context(), "accessibility-refines")
	require.NoError(t, err)
	e := manifest.NewA11y()
	e.Summary = "This publication conforms to WCAG 2.2 Level AA."
	e.ConformsTo = []manifest.A11yProfile{manifest.EPUBA11y11WCAG22AA}
	e.Certification = &manifest.A11yCertification{
		CertifiedBy: "Standard Ebooks",
	}
	e.AccessModes = manifest.A11yAccessModesFromStrings([]string{"textual"})
	e.AccessModesSufficient = [][]manifest.A11yPrimaryAccessMode{
		a11yAccessModesSufficient("textual"),
	}
	e.Features = valuesToA11yFeatures([]string{"readingOrder", "structuralNavigation", "tableOfContents", "unlocked"})
	e.Hazards = valuesToA11yHazards([]string{"none"})
	assert.Equal(t, &e, m.Accessibility)
	assert.Nil(t, m.OtherMetadata["accessibility"])
}

func TestMetadataEPUB3TDM(t *testing.T) {
	m, err := loadMetadata(t.Context(), "tdm-epub3")
	require.NoError(t, err)
	assert.Equal(t, &manifest.TDM{
		Policy:      "https://provider.com/policies/policy.json",
		Reservation: manifest.TDMReservationAll,
	}, m.TDM)
}

func TestMetadataTitleFileAs(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "titles-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "titles-epub3")
	require.NoError(t, err)

	assert.Equal(t, "Adventures", m2.SortAs())
	assert.Equal(t, "Adventures", m3.SortAs())
}

func TestMetadataTitleMainTakesPrecedence(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "title-main-precedence")
	require.NoError(t, err)

	assert.Equal(t, "Main title takes precedence", m3.Title())
}

func TestMetadataTitleSelectedSubtitleHasLowestDisplaySeqProperty(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "title-multiple-subtitles")
	require.NoError(t, err)

	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en": "Subtitle 2",
	}), *m3.LocalizedSubtitle)
}

func TestMetadataSubjectLocalized(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "subjects-complex")
	require.NoError(t, err)

	assert.Len(t, m3.Subjects, 1)
	assert.Equal(t, manifest.NewLocalizedStringFromStrings(map[string]string{
		"en": "FICTION / Occult & Supernatural",
		"fr": "FICTION / Occulte & Surnaturel",
	}), m3.Subjects[0].LocalizedName)
}

func TestMetadataSubjectFileAs(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "subjects-complex")
	require.NoError(t, err)

	assert.Len(t, m3.Subjects, 1)
	assert.Equal(t, "occult", m3.Subjects[0].SortAs())
}

func TestMetadataSubjectCodeAndScheme(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "subjects-complex")
	require.NoError(t, err)

	assert.Len(t, m3.Subjects, 1)
	assert.Equal(t, "BISAC", m3.Subjects[0].Scheme)
	assert.Equal(t, "FIC024000", m3.Subjects[0].Code)
}

func TestMetadataSubjectCommaSeparatedSplit(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "subjects-single")
	require.NoError(t, err)

	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("apple")})
	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("banana")})
	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("pear")})
}

func TestMetadataSubjectCommaSeparatedMultipleNotSplit(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "subjects-multiple")
	require.NoError(t, err)

	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("fiction")})
	assert.Contains(t, m3.Subjects, manifest.Subject{LocalizedName: manifest.NewLocalizedStringFromString("apple; banana,  pear")})
}

func TestMetadataDatePublished(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "dates-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "dates-epub3")
	require.NoError(t, err)

	tx, err := time.Parse(time.RFC3339, "1865-07-04T00:00:00Z")
	require.NoError(t, err)

	assert.Equal(t, &tx, m2.Published)
	assert.Equal(t, &tx, m3.Published)

	// Non-ISO date
	m3notiso, err := loadMetadata(t.Context(), "dates-epub3-notiso")
	require.NoError(t, err)
	assert.Equal(t, time.Date(1865, time.January, 1, 0, 0, 0, 0, time.UTC), *m3notiso.Published)
}

func TestMetadataDateModified(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "dates-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "dates-epub3")
	require.NoError(t, err)

	tx, err := time.Parse(time.RFC3339, "2012-04-02T12:47:00Z")
	require.NoError(t, err)

	assert.Equal(t, &tx, m2.Modified)
	assert.Equal(t, &tx, m3.Modified)

	// Non-ISO date
	m3notiso, err := loadMetadata(t.Context(), "dates-epub3-notiso")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2012, time.April, 1, 0, 0, 0, 0, time.UTC), *m3notiso.Modified)
}

func TestMetadataConformsToProfileEPUB(t *testing.T) {
	m2, err := loadMetadata(t.Context(), "contributors-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "contributors-epub3")
	require.NoError(t, err)

	assert.Contains(t, m2.ConformsTo, manifest.ProfileEPUB)
	assert.Contains(t, m3.ConformsTo, manifest.ProfileEPUB)
}

func TestMetadataUniqueIdentifierParsed(t *testing.T) {
	m3, err := loadMetadata(t.Context(), "identifier-unique")
	require.NoError(t, err)

	assert.Equal(t, "urn:uuid:2", m3.Identifier)
}

func TestMetadataLayout(t *testing.T) {
	m3, err := loadMetadata(t.Context(), "presentation-metadata")
	require.NoError(t, err)
	assert.Equal(t, manifest.LayoutFixed, m3.Layout)
	assert.Equal(t, "scrolled-doc", m3.OtherMetadata["http://www.idpf.org/vocab/rendition/#flow"])
	assert.Empty(t, m3.OtherMetadata["http://www.idpf.org/vocab/rendition/#layout"])
	assert.Equal(t, "landscape", m3.OtherMetadata["http://www.idpf.org/vocab/rendition/#orientation"])
	assert.Equal(t, "both", m3.OtherMetadata["http://www.idpf.org/vocab/rendition/#spread"])
}

func TestMetadataCoverLink(t *testing.T) {
	// Note: not using loadMetadata
	m2, err := loadPackageDoc(t.Context(), "cover-epub2")
	require.NoError(t, err)
	m3, err := loadPackageDoc(t.Context(), "cover-epub3")
	require.NoError(t, err)
	mm, err := loadPackageDoc(t.Context(), "cover-mix")
	require.NoError(t, err)

	expected := &manifest.Link{
		Href:      manifest.MustNewHREFFromString("OEBPS/cover.jpg", false),
		MediaType: &mediatype.JPEG,
		Rels:      []string{"cover"},
	}
	assert.Equal(t, m2.Resources.FirstWithRel("cover"), expected)
	assert.Equal(t, m3.Resources.FirstWithRel("cover"), expected)
	assert.Equal(t, mm.Resources.FirstWithRel("cover"), expected)
}

func TestMetadataCrossRefinings(t *testing.T) {
	_, err := loadPackageDoc(t.Context(), "meta-termination")
	assert.NoError(t, err)
}

func TestMetadataOtherMetadata(t *testing.T) {
	m3, err := loadMetadata(t.Context(), "meta-others")
	require.NoError(t, err)

	assert.Equal(t, m3.OtherMetadata, map[string]interface{}{
		VocabularyDCTerms + "source": []interface{}{
			"Feedbooks",
			map[string]interface{}{"@value": "Web", "http://my.url/#scheme": "http"},
			"Internet",
		},
		"http://idpf.org/epub/vocab/package/meta/#Sigil%20version": "1.9.20",
		"http://www.idpf.org/2007/opf#version":                     "3.0",
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
	m3, err := loadMetadata(t.Context(), "collections-epub3")
	require.NoError(t, err)

	assert.Contains(t, m3.BelongsToCollections(), manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Collection B",
		}),
	})
}

func TestMetadataCollectionsWithUnknownTypeInBelongsTo(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "collections-epub3")
	require.NoError(t, err)

	assert.Contains(t, m3.BelongsToCollections(), manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Collection A",
		}),
	})
}

func TestMetadataCollectionLocalizedSeries(t *testing.T) {
	// EPUB 3 only
	m3, err := loadMetadata(t.Context(), "collections-epub3")
	require.NoError(t, err)

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
	m2, err := loadMetadata(t.Context(), "collections-epub2")
	require.NoError(t, err)
	m3, err := loadMetadata(t.Context(), "collections-epub3")
	require.NoError(t, err)

	expected := manifest.Collection{
		LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
			"en": "Series B",
		}),
		Position: floatP(1.5),
	}

	assert.Contains(t, m2.BelongsToSeries(), expected)
	assert.Contains(t, m3.BelongsToSeries(), expected)
}
