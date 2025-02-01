package parser

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

func withImageParser(t *testing.T, filepath string, f func(*pub.Builder)) {
	a := asset.File(filepath)
	fet, err := a.CreateFetcher(asset.Dependencies{
		ArchiveFactory: archive.NewArchiveFactory(),
	}, "")
	assert.NoError(t, err)
	p, err := ImageParser{}.Parse(a, fet)
	assert.NoError(t, err)
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
		assert.NotNil(t, p)
		pub := p.Build()
		assert.NotNil(t, pub)

		assert.Equal(t, pub.Manifest.Metadata.ConformsTo, manifest.Profiles{manifest.ProfileDivina})
	})
}

func TestImageReadingOrderAlphabetical(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		assert.NotNil(t, p)
		pub := p.Build()
		assert.NotNil(t, pub)
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
		assert.NotNil(t, p)
		pub := p.Build()
		assert.NotNil(t, pub)

		coverItem := pub.Manifest.ReadingOrder.FirstWithRel("cover")
		assert.NotNil(t, coverItem, "readingOrder should have an item with rel=cover")

		u, _ := url.URLFromDecodedPath("Cory Doctorow's Futuristic Tales of the Here and Now/a-fc.jpg")
		assert.Equal(t, manifest.NewHREF(u).String(), coverItem.Href.String())
	})
}

func TestImageTitleBasedOnRoot(t *testing.T) {
	withImageParser(t, "./testdata/image/futuristic_tales.cbz", func(p *pub.Builder) {
		assert.NotNil(t, p)
		pub := p.Build()
		assert.NotNil(t, pub)

		assert.Equal(
			t,
			"Cory Doctorow's Futuristic Tales of the Here and Now",
			pub.Manifest.Metadata.Title(),
			"publication title should be based on archive's root directory",
		)
	})
}
