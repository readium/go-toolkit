package streamer

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReturnsAdditionalInferredA11yMetadata(t *testing.T) {
	a11y := manifest.NewA11y()
	a11y.ConformsTo = []manifest.A11yProfile{"unknown"}

	m := manifest.Manifest{
		Metadata: manifest.Metadata{
			ConformsTo:    manifest.Profiles{manifest.ProfileEPUB},
			Accessibility: &a11y,
			Layout:        manifest.LayoutReflowable,
		},
		ReadingOrder: []manifest.Link{
			newLink(mediatype.HTML, "html"),
		},
	}

	inferreddA11y := manifest.NewA11y()
	inferreddA11y.AccessModes = []manifest.A11yAccessMode{manifest.A11yAccessModeTextual}
	inferreddA11y.AccessModesSufficient = [][]manifest.A11yPrimaryAccessMode{{manifest.A11yPrimaryAccessModeTextual}}

	res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, &inferreddA11y, res)

	// Original manifest should not be modified.
	assert.Equal(t, &a11y, m.Metadata.Accessibility)
}

func newLink(mt mediatype.MediaType, extension string) manifest.Link {
	return manifest.Link{
		Href:      manifest.MustNewHREFFromString("file."+extension, false),
		MediaType: &mt,
	}
}

// If the publication contains a reference to an audio or video resource
// (inspect "resources" and "readingOrder" in RWPM).
func TestInferAuditoryAccessMode(t *testing.T) {
	assertAccessMode(t, manifest.A11yAccessModeAuditory, "mp3", mediatype.MP3)
	assertAccessMode(t, manifest.A11yAccessModeAuditory, "mpeg", mediatype.MPEG)
}

// If the publications contains a reference to an image or a video resource
// (inspect "resources" and "readingOrder" in RWPM)
func TestInferVisualAccessMode(t *testing.T) {
	assertAccessMode(t, manifest.A11yAccessModeVisual, "jpg", mediatype.JPEG)
	assertAccessMode(t, manifest.A11yAccessModeVisual, "mpeg", mediatype.MPEG)
}

func assertAccessMode(t *testing.T, accessMode manifest.A11yAccessMode, extension string, mt mediatype.MediaType) {
	testManifest := func(m manifest.Manifest) {
		res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.AccessModes, accessMode)
	}

	link := newLink(mt, extension)

	testManifest(manifest.Manifest{
		ReadingOrder: []manifest.Link{link},
	})
	testManifest(manifest.Manifest{
		Resources: []manifest.Link{link},
	})
}

// If the publication is partially or fully accessible (WCAG A or above)
func TestInferTextualAccessModeAndAccessModeSufficientFromProfile(t *testing.T) {
	test := func(profile manifest.A11yProfile) {
		a11y := manifest.NewA11y()
		a11y.ConformsTo = []manifest.A11yProfile{profile}
		m := manifest.Manifest{
			Metadata: manifest.Metadata{
				Accessibility: &a11y,
			},
		}
		res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.AccessModes, manifest.A11yAccessModeTextual)
		assert.Contains(t, res.AccessModesSufficient, []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual})
	}

	test(manifest.EPUBA11y10WCAG20A)
	test(manifest.EPUBA11y10WCAG20AA)
	test(manifest.EPUBA11y10WCAG20AAA)
}

// Or if a reflowable EPUB does not contain any image, audio or video resource
// (inspect "resources" and "readingOrder" in RWPM)
func TestInferTextualAccessModeAndAccessModeSufficientFromLackOfMedia(t *testing.T) {
	testManifest := func(contains bool, m manifest.Manifest) {
		res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
		require.NoError(t, err)
		assert.NotNil(t, res)
		ams := []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual}

		if contains {
			assert.Contains(t, res.AccessModes, manifest.A11yAccessModeTextual)
			assert.Contains(t, res.AccessModesSufficient, ams)
		} else {
			assert.NotContains(t, res.AccessModes, manifest.A11yAccessModeTextual)
			assert.NotContains(t, res.AccessModesSufficient, ams)
		}
	}

	a11y := manifest.NewA11y()
	a11y.ConformsTo = []manifest.A11yProfile{"unknown"}

	testReadingOrder := func(contains bool, mt mediatype.MediaType, extension string) {
		testManifest(contains, manifest.Manifest{
			Metadata: manifest.Metadata{
				ConformsTo:    manifest.Profiles{manifest.ProfileEPUB},
				Accessibility: &a11y,
				Layout:        manifest.LayoutReflowable,
			},
			ReadingOrder:    []manifest.Link{newLink(mt, extension)},
			TableOfContents: []manifest.Link{newLink(mt, extension)},
		})
	}

	testReadingOrder(true, mediatype.HTML, "html")
	testReadingOrder(false, mediatype.JPEG, "jpg")
	testReadingOrder(false, mediatype.MP3, "mp3")
	testReadingOrder(false, mediatype.MPEG, "mpeg")
	testReadingOrder(false, mediatype.PDF, "pdf")

	testResources := func(contains bool, mt mediatype.MediaType, extension string) {
		testManifest(contains, manifest.Manifest{
			Metadata: manifest.Metadata{
				ConformsTo:    manifest.Profiles{manifest.ProfileEPUB},
				Accessibility: &a11y,
				Layout:        manifest.LayoutReflowable,
			},
			ReadingOrder:    []manifest.Link{newLink(mt, extension)},
			Resources:       []manifest.Link{newLink(mt, extension)},
			TableOfContents: []manifest.Link{newLink(mt, extension)},
		})
	}

	testResources(true, mediatype.HTML, "html")
	testResources(false, mediatype.JPEG, "jpg")
	testResources(false, mediatype.MP3, "mp3")
	testResources(false, mediatype.MPEG, "mpeg")
	testResources(false, mediatype.PDF, "pdf")
}

// ... but not for FXL EPUB
func TestDontInferTextualAccessModeAndAccessModeSufficientFromLackOfMediaForFXL(t *testing.T) {
	a11y := manifest.NewA11y()
	a11y.ConformsTo = []manifest.A11yProfile{"unknown"}

	m := manifest.Manifest{
		Metadata: manifest.Metadata{
			ConformsTo:    manifest.Profiles{manifest.ProfileEPUB},
			Accessibility: &a11y,
			Layout:        manifest.LayoutFixed,
		},
		ReadingOrder:    []manifest.Link{newLink(mediatype.HTML, "html")},
		TableOfContents: []manifest.Link{newLink(mediatype.HTML, "html")},
	}

	res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
	require.NoError(t, err)
	assert.NotNil(t, res)
	ams := []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual}
	assert.NotContains(t, res.AccessModes, manifest.A11yAccessModeTextual)
	assert.NotContains(t, res.AccessModesSufficient, ams)
}

func TestInferTextualAccessModeWithIgnoredImages(t *testing.T) {
	testHash := manifest.HashList{
		manifest.HashValue{
			Algorithm: manifest.HashAlgorithmSHA256,
			Value:     "nzGm6cNL7fAadGSoFdtLzg/Z3MFqe3/fiWUZF9CPAKY=",
		},
	}
	l := newLink(mediatype.PNG, "png")
	l.Properties = manifest.Properties{
		"hash": testHash.ToJSONArray(),
	}

	cover := newLink(mediatype.JPEG, "jpg")
	cover.Rels = manifest.Strings{"cover"}

	m := manifest.Manifest{
		Metadata: manifest.Metadata{
			ConformsTo: manifest.Profiles{manifest.ProfileEPUB},
			Layout:     manifest.LayoutReflowable,
		},
		ReadingOrder: []manifest.Link{
			newLink(mediatype.HTML, "html"),
		},
		Resources: []manifest.Link{
			cover,
			l,
		},
	}
	f := fetcher.NewFileFetcher("file.png", "testdata/file.png")

	res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, f, nil), nil)
	require.NoError(t, err)
	assert.NotContains(t, res.AccessModes, manifest.A11yAccessModeTextual)
	assert.Contains(t, res.AccessModes, manifest.A11yAccessModeVisual)
	assert.NotContains(t, res.AccessModesSufficient, []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual})

	res, err = inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, f, nil), testHash)
	require.NoError(t, err)
	assert.Contains(t, res.AccessModes, manifest.A11yAccessModeTextual)
	assert.Contains(t, res.AccessModes, manifest.A11yAccessModeVisual)
	assert.Contains(t, res.AccessModesSufficient, []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual})
}

// If the publication contains only references to audio resources (inspect "resources" and "readingOrder" in RWPM)
func TestInferAuditoryAccessModeSufficient(t *testing.T) {
	testManifest := func(contains bool, m manifest.Manifest) {
		res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
		require.NoError(t, err)
		if res == nil && !contains {
			return
		}
		assert.NotNil(t, res)
		ams := []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeAuditory}

		if contains {
			assert.Contains(t, res.AccessModesSufficient, ams)
		} else {
			assert.NotContains(t, res.AccessModesSufficient, ams)
		}
	}

	a11y := manifest.NewA11y()
	a11y.ConformsTo = []manifest.A11yProfile{"unknown"}

	testReadingOrder := func(contains bool, links ...manifest.Link) {
		testManifest(contains, manifest.Manifest{
			Metadata:     manifest.Metadata{Accessibility: &a11y},
			ReadingOrder: links,
		})
	}

	html := newLink(mediatype.HTML, "html")
	mp3 := newLink(mediatype.MP3, "mp3")

	testReadingOrder(false, html, html)
	testReadingOrder(false, html, mp3)
	testReadingOrder(true, mp3, mp3)

	testResources := func(contains bool, links ...manifest.Link) {
		testManifest(contains, manifest.Manifest{
			Metadata:  manifest.Metadata{Accessibility: &a11y},
			Resources: links,
		})
	}

	testResources(false, html, html)
	testResources(false, html, mp3)
	testResources(true, mp3, mp3)
}

// If the publication contains only references to image or video resources (inspect "resources" and "readingOrder" in RWPM)
func TestInferVisualAccessModeSufficient(t *testing.T) {
	testManifest := func(contains bool, m manifest.Manifest) {
		res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
		require.NoError(t, err)
		if res == nil && !contains {
			return
		}
		assert.NotNil(t, res)
		ams := []manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeVisual}

		if contains {
			assert.Contains(t, res.AccessModesSufficient, ams)
		} else {
			assert.NotContains(t, res.AccessModesSufficient, ams)
		}
	}

	a11y := manifest.NewA11y()
	a11y.ConformsTo = []manifest.A11yProfile{"unknown"}

	testReadingOrder := func(contains bool, links ...manifest.Link) {
		testManifest(contains, manifest.Manifest{
			Metadata:     manifest.Metadata{Accessibility: &a11y},
			ReadingOrder: links,
		})
	}

	html := newLink(mediatype.HTML, "html")
	jpg := newLink(mediatype.JPEG, "jpg")
	mpeg := newLink(mediatype.MPEG, "mpeg")

	testReadingOrder(false, html)
	testReadingOrder(false, html, jpg)
	testReadingOrder(true, jpg)
	testReadingOrder(true, mpeg)
	testReadingOrder(true, jpg, mpeg)

	testResources := func(contains bool, links ...manifest.Link) {
		testManifest(contains, manifest.Manifest{
			Metadata:  manifest.Metadata{Accessibility: &a11y},
			Resources: links,
		})
	}

	testResources(false, html)
	testResources(false, html, jpg)
	testResources(true, jpg)
	testResources(true, mpeg)
	testResources(true, jpg, mpeg)
}

// If the publications contains a table of contents (check for the presence of
// a "toc" collection in RWPM)
func TestInferFeatureTableOfContents(t *testing.T) {
	m := manifest.Manifest{
		TableOfContents: []manifest.Link{newLink(mediatype.HTML, "html")},
	}
	assertFeature(t, m, manifest.A11yFeatureTableOfContents)
}

// If the publications contains a page list (check for the presence of a
// "pageList" collection in RWPM)
func TestInferFeaturePageList(t *testing.T) {
	m := manifest.Manifest{
		Metadata: manifest.Metadata{
			ConformsTo: []manifest.Profile{manifest.ProfileEPUB},
		},
		Subcollections: map[string][]manifest.PublicationCollection{
			"pageList": {
				manifest.PublicationCollection{
					Links: []manifest.Link{newLink(mediatype.HTML, "html")},
				},
			},
		},
		ReadingOrder: []manifest.Link{newLink(mediatype.HTML, "html")},
	}
	assertFeature(t, m, manifest.A11yFeaturePageNavigation)
}

// If the publication contains any resource with MathML (check for the presence
// of the "contains" property where the value is "mathml" in "readingOrder" or
// "resources" in RWPM)
func TestInferFeatureMathML(t *testing.T) {
	link := newLink(mediatype.HTML, "html")
	link.Properties = manifest.Properties{
		"contains": []string{"mathml"},
	}
	m := manifest.Manifest{
		Metadata: manifest.Metadata{
			ConformsTo: []manifest.Profile{manifest.ProfileEPUB},
		},
		ReadingOrder: []manifest.Link{link},
	}
	assertFeature(t, m, manifest.A11yFeatureMathML)
}

// If the publication is fully accessible (WCAG AA or above)
//
// This property should only be inferred for reflowable EPUB files as it
// doesn't apply to other formats (FXL, PDF, audiobooks, CBZ/CBR).
func TestInferFeatureDisplayTransformability(t *testing.T) {
	test := func(contains bool, profile manifest.A11yProfile, layout manifest.Layout) {
		a11y := manifest.NewA11y()
		a11y.ConformsTo = []manifest.A11yProfile{profile}

		m := manifest.Manifest{
			Metadata: manifest.Metadata{
				ConformsTo:    []manifest.Profile{manifest.ProfileEPUB},
				Accessibility: &a11y,
				Layout:        layout,
			},
			ReadingOrder: []manifest.Link{newLink(mediatype.HTML, "html")},
		}

		res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
		require.NoError(t, err)
		assert.NotNil(t, res)
		if contains {
			assert.Contains(t, res.Features, manifest.A11yFeatureDisplayTransformability)
		} else {
			assert.NotContains(t, res.Features, manifest.A11yFeatureDisplayTransformability)
		}
	}

	test(false, manifest.EPUBA11y10WCAG20A, manifest.LayoutReflowable)
	test(true, manifest.EPUBA11y10WCAG20AA, manifest.LayoutReflowable)
	test(true, manifest.EPUBA11y10WCAG20AAA, manifest.LayoutReflowable)
	test(false, manifest.EPUBA11y10WCAG20AAA, manifest.LayoutFixed)
}

// If the publication contains any reference to Media Overlays.
func TestInferFeatureSynchronizedAudioText(t *testing.T) {
	link := newLink(mediatype.XHTML, "xhtml")
	smil := newLink(mediatype.SMIL, "smil")
	m := manifest.Manifest{
		Metadata: manifest.Metadata{
			ConformsTo: []manifest.Profile{manifest.ProfileEPUB},
		},
		ReadingOrder: []manifest.Link{link},
		Resources:    []manifest.Link{smil},
	}
	assertFeature(t, m, manifest.A11yFeatureSynchronizedAudioText)
}

func assertFeature(t *testing.T, m manifest.Manifest, feature manifest.A11yFeature) {
	res, err := inferA11yMetadataInPublicationManifest(context.TODO(), pub.New(m, nil, nil), nil)
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, res.Features, feature)
}
