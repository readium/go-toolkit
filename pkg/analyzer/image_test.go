package analyzer

import (
	"os"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectImage(t *testing.T) {
	fs := os.DirFS("testdata/")
	catLink := manifest.Link{
		Href:      manifest.MustNewHREFFromString("catsink.jpg", false),
		MediaType: &mediatype.JPEG,
	}

	link, err := InspectImage(fs, catLink, []manifest.HashAlgorithm{})
	require.NoError(t, err)
	require.NotNil(t, link)
	assert.Equal(t, uint(615), link.Width)
	assert.Equal(t, uint(458), link.Height)
	assert.Equal(t, uint(36710), link.Size)
	assert.False(t, link.Properties.Get("animated").(bool))
	assert.Empty(t, link.Properties.Hash())

	link, err = InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("animated.webp", false),
		MediaType: &mediatype.WEBP,
	}, []manifest.HashAlgorithm{})
	require.NoError(t, err)
	require.NotNil(t, link)
	assert.Equal(t, uint(1000), link.Width)
	assert.Equal(t, uint(1000), link.Height)
	assert.Equal(t, uint(5764), link.Size)
	assert.True(t, link.Properties.Get("animated").(bool))

	link, err = InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("animated.png", false),
		MediaType: &mediatype.PNG,
	}, []manifest.HashAlgorithm{})
	require.NoError(t, err)
	require.NotNil(t, link)
	assert.Equal(t, uint(1000), link.Width)
	assert.Equal(t, uint(1000), link.Height)
	assert.Equal(t, uint(2932), link.Size)
	assert.True(t, link.Properties.Get("animated").(bool))

	_, err = InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("corrupt.png", false),
		MediaType: &mediatype.PNG,
	}, []manifest.HashAlgorithm{})
	require.Error(t, err)

	_, err = InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("frame1.jxl", false),
		MediaType: &mediatype.JXL,
	}, []manifest.HashAlgorithm{})
	require.ErrorContains(t, err, "JXL file format is currently unsupported")

	link, err = InspectImage(fs, catLink, []manifest.HashAlgorithm{
		manifest.HashAlgorithmBlake2b, // This is expected to not to anything
		manifest.HashAlgorithmSHA256,
	})
	require.NoError(t, err)
	require.NotNil(t, link)
	if assert.Len(t, link.Properties.Hash(), 1) {
		assert.True(t, link.Properties.Hash()[0].Equal(manifest.HashValue{
			Algorithm: manifest.HashAlgorithmSHA256,
			Value:     "nzGm6cNL7fAadGSoFdtLzg/Z3MFqe3/fiWUZF9CPAKY=",
		}))
	}

	link, err = InspectImage(fs, catLink, []manifest.HashAlgorithm{
		manifest.HashAlgorithmPhashDCT,
	})
	require.NoError(t, err)
	require.NotNil(t, link)
	if assert.Len(t, link.Properties.Hash(), 1) {
		assert.True(t, link.Properties.Hash()[0].Equal(manifest.HashValue{
			Algorithm: manifest.HashAlgorithmPhashDCT,
			Value:     "TL5pWb0AIL8=",
		}))
	}
}

func TestMatchImage(t *testing.T) {
	fs := os.DirFS("testdata/")

	ok, err := MatchImage(manifest.Link{
		Href:      manifest.MustNewHREFFromString("audio.mp3", false),
		MediaType: &mediatype.MP3,
	}, manifest.HashList{})
	require.ErrorContains(t, err, "link is not to an image that can be matched")
	require.False(t, ok)

	link, err := InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("catsink.jpg", false),
		MediaType: &mediatype.JPEG,
	}, []manifest.HashAlgorithm{
		manifest.HashAlgorithmSHA256,
		manifest.HashAlgorithmPhashDCT,
	})
	require.NoError(t, err)
	require.NotNil(t, link)
	ok, err = MatchImage(*link, manifest.HashList{
		manifest.HashValue{
			Algorithm: manifest.HashAlgorithmSHA256,
			Value:     "nzGm6cNL7fAadGSoFdtLzg/Z3MFqe3/fiWUZF9CPAKY=",
		},
	})
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = MatchImage(*link, manifest.HashList{
		manifest.HashValue{
			Algorithm: manifest.HashAlgorithmSHA256,
			Value:     "xxxxxxxxfAadGSoFdtLzg/Z3MFqe3/fiWUZF9CPAKY=",
		},
	})
	require.NoError(t, err)
	require.False(t, ok)

	link1, err := InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("frame1.png", false),
		MediaType: &mediatype.PNG,
	}, []manifest.HashAlgorithm{manifest.HashAlgorithmPhashDCT})
	require.NoError(t, err)
	require.NotNil(t, link1)
	link2, err := InspectImage(fs, manifest.Link{
		Href:      manifest.MustNewHREFFromString("frame2.png", false),
		MediaType: &mediatype.PNG,
	}, []manifest.HashAlgorithm{manifest.HashAlgorithmPhashDCT})
	require.NoError(t, err)
	require.NotNil(t, link2)
	if assert.Len(t, link1.Properties.Hash(), 1) && assert.Len(t, link2.Properties.Hash(), 1) {
		hashes1 := link1.Properties.Hash()
		hashes2 := link2.Properties.Hash()

		// Too similar, they match
		ok, err = MatchImage(*link1, hashes2)
		require.NoError(t, err)
		assert.True(t, ok)

		// Pretty different, no match
		ok, err = MatchImage(*link, hashes1)
		require.NoError(t, err)
		assert.False(t, ok)
	}
}
