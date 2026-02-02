package parser

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withImageParser(t *testing.T, filepath string, f func(*pub.Builder)) {
	u, _ := url.FromFilepath(filepath)
	a := asset.File(u)
	fet, err := a.CreateFetcher(t.Context(), asset.Dependencies{
		ArchiveFactory: archive.NewArchiveFactory(),
	}, "")
	require.NoError(t, err)
	p, err := ImageParser{}.Parse(t.Context(), a, fet)
	require.NoError(t, err)
	f(p)
}

func TestImageCBZAccepted(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		assert.NotNil(t, p)
	})
}

func TestImageJPGAccepted(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.jpg", func(p *pub.Builder) {
		assert.NotNil(t, p)
	})
}

func TestImageConformsTo(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		require.NotNil(t, p)
		pub := p.Build()
		require.NotNil(t, pub)

		assert.Equal(t, pub.Manifest.Metadata.ConformsTo, manifest.Profiles{manifest.ProfileDivina})
	})
}

func TestImageReadingOrderAlphabetical(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		require.NotNil(t, p)
		pub := p.Build()
		require.NotNil(t, pub)
		base, _ := url.URLFromDecodedPath("Cory Doctorow's Futuristic Tales of the Here and Now/")

		hrefs := make([]string, 0, len(pub.Manifest.ReadingOrder))
		for _, roi := range pub.Manifest.ReadingOrder {
			hrefs = append(hrefs, base.Relativize(roi.URL(nil, nil)).String())
		}
		assert.Exactly(t, []string{
			"a-fc.jpg", "x-002.jpg", "x-003.jpg", "x-004.jpg",
		}, hrefs, "readingOrder should be sorted alphabetically")
	})
}

func TestImageCoverFirstItem(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		require.NotNil(t, p)
		pub := p.Build()
		require.NotNil(t, pub)

		coverItem := pub.Manifest.ReadingOrder.FirstWithRel("cover")
		require.NotNil(t, coverItem, "readingOrder should have an item with rel=cover")

		u, _ := url.URLFromDecodedPath("Cory Doctorow's Futuristic Tales of the Here and Now/a-fc.jpg")
		assert.Equal(t, manifest.NewHREF(u).String(), coverItem.Href.String())
	})
}

func TestImageTitleBasedOnRoot(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		require.NotNil(t, p)
		pub := p.Build()
		require.NotNil(t, pub)

		assert.Equal(
			t,
			"Cory Doctorow's Futuristic Tales of the Here and Now",
			pub.Manifest.Metadata.Title(),
			"publication title should be based on archive's root directory",
		)
	})
}
