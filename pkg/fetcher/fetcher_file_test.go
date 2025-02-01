package fetcher

import (
	"bytes"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
)

var testFileFetcher = &FileFetcher{
	paths: map[string]string{
		"file_href": "./testdata/text.txt",
		"dir_href":  "./testdata/directory",
	},
}

func TestFileFetcherLengthNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("unknown", false)})
	_, err := resource.Length()
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherReadNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("unknown", false)})
	_, err := resource.Read(0, 0)
	assert.Equal(t, NotFound(err.Cause), err)
	_, err = resource.Stream(&bytes.Buffer{}, 0, 0)
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherHrefInMap(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("file_href", false)})
	bin, err := resource.Read(0, 0)
	if assert.Nil(t, err) {
		assert.Equal(t, "text", string(bin))
	}
	var b bytes.Buffer
	n, err := resource.Stream(&b, 0, 0)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 4, n)
		assert.Equal(t, "text", b.String())
	}
}

func TestFileFetcherDirectoryFile(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("dir_href/text1.txt", false)})
	bin, err := resource.Read(0, 0)
	if assert.Nil(t, err) {
		assert.Equal(t, "text1", string(bin))
	}
	var b bytes.Buffer
	n, err := resource.Stream(&b, 0, 0)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 5, n)
		assert.Equal(t, "text1", b.String())
	}
}

func TestFileFetcherSubdirectoryFile(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("dir_href/subdirectory/text2.txt", false)})
	bin, err := resource.Read(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, "text2", string(bin))
	var b bytes.Buffer
	n, err := resource.Stream(&b, 0, 0)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 5, n)
		assert.Equal(t, "text2", b.String())
	}
}

func TestFileFetcherDirectoryNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("dir_href/subdirectory", false)})
	_, err := resource.Read(0, 0)
	assert.Equal(t, NotFound(err.Cause), err)
	_, err = resource.Stream(&bytes.Buffer{}, 0, 0)
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherDirectoryTraversalNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("dir_href/../text.txt", false)})
	_, err := resource.Read(0, 0)
	assert.Equal(t, NotFound(err.Cause), err, "cannot traverse up a directory using '..'")
	_, err = resource.Stream(&bytes.Buffer{}, 0, 0)
	assert.Equal(t, NotFound(err.Cause), err, "cannot traverse up a directory using '..'")
}

func TestFileFetcherReadRange(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("file_href", false)})
	bin, err := resource.Read(0, 2)
	if assert.Nil(t, err) {
		assert.Equal(t, "tex", string(bin), "read data should be the first three bytes of the file")
	}

	var b bytes.Buffer
	n, err := resource.Stream(&b, 0, 2)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 3, n)
		assert.Equal(t, "tex", b.String(), "read data should be the first three bytes of the file")
	}
}

func TestFileFetcherTwoRangesSameResource(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("file_href", false)})
	bin, err := resource.Read(0, 1)
	if assert.Nil(t, err) {
		assert.Equal(t, "te", string(bin))
	}
	var b bytes.Buffer
	n, err := resource.Stream(&b, 0, 1)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 2, n)
		assert.Equal(t, "te", b.String())
	}

	bin, err = resource.Read(1, 3)
	if assert.Nil(t, err) {
		assert.Equal(t, "ext", string(bin))
	}
	b.Reset()
	n, err = resource.Stream(&b, 1, 3)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 3, n)
		assert.Equal(t, "ext", b.String())
	}
}

func TestFileFetcherOutOfRangeClamping(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("file_href", false)})
	bin, err := resource.Read(-5, 60)
	if assert.Nil(t, err) {
		assert.Equal(t, "text", string(bin))
	}
	var b bytes.Buffer
	n, err := resource.Stream(&b, -5, 60)
	if assert.Nil(t, err) {
		assert.EqualValues(t, 4, n)
		assert.Equal(t, "text", b.String())
	}
}

func TestFileFetcherDecreasingRange(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("file_href", false)})
	_, err := resource.Read(60, 20)
	if assert.Error(t, err) {
		assert.Equal(t, RangeNotSatisfiable(err.Cause), err, "range isn't satisfiable")
	}
	_, err = resource.Stream(&bytes.Buffer{}, 60, 20)
	if assert.Error(t, err) {
		assert.Equal(t, RangeNotSatisfiable(err.Cause), err, "range isn't satisfiable")
	}
}

func TestFileFetcherComputingLength(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("file_href", false)})
	length, err := resource.Length()
	assert.Nil(t, err)
	assert.EqualValues(t, 4, length)
}

func TestFileFetcherDirectoryLengthNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("dir_href/subdirectory", false)})
	_, err := resource.Length()
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherFileNotFoundLength(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("unknown", false)})
	_, err := resource.Length()
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherLinks(t *testing.T) {
	links, err := testFileFetcher.Links()
	assert.Nil(t, err)

	mustContain := manifest.LinkList{{
		Href:      manifest.MustNewHREFFromString("dir_href/subdirectory/hello.mp3", false),
		MediaType: &mediatype.MP3,
	}, {
		Href:      manifest.MustNewHREFFromString("dir_href/subdirectory/text2.txt", false),
		MediaType: &mediatype.Text,
	}, {
		Href:      manifest.MustNewHREFFromString("dir_href/text1.txt", false),
		MediaType: &mediatype.Text,
	}, {
		Href:      manifest.MustNewHREFFromString("file_href", false),
		MediaType: &mediatype.Text,
	}}

	assert.ElementsMatch(t, mustContain, links)
}
