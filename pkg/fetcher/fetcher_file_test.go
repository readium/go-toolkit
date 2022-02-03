package fetcher

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

var testFileFetcher = &FileFetcher{
	paths: map[string]string{
		"/file_href": "./testdata/text.txt",
		"/dir_href":  "./testdata/directory",
	},
}

func TestFileFetcherLengthNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/unknown"})
	_, err := resource.Length()
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherReadNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/unknown"})
	_, err := resource.Read(0, 0)
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherHrefInMap(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/file_href"})
	bin, err := resource.Read(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, "text", string(bin))
}

func TestFileFetcherDirectoryFile(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/dir_href/text1.txt"})
	bin, err := resource.Read(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, "text1", string(bin))
}

func TestFileFetcherSubdirectoryFile(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/dir_href/subdirectory/text2.txt"})
	bin, err := resource.Read(0, 0)
	assert.Nil(t, err)
	assert.Equal(t, "text2", string(bin))
}

func TestFileFetcherDirectoryNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/dir_href/subdirectory"})
	_, err := resource.Read(0, 0)
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherDirectoryTraversalNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/dir_href/../text.txt"})
	_, err := resource.Read(0, 0)
	assert.Equal(t, NotFound(err.Cause), err, "cannot traverse up a directory using '..'")
}

func TestFileFetcherReadRange(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/file_href"})
	bin, err := resource.Read(0, 2)
	assert.Nil(t, err)
	assert.Equal(t, "tex", string(bin), "read data should be the first three bytes of the file")
}

func TestFileFetcherTwoRangesSameResource(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/file_href"})
	bin, err := resource.Read(0, 1)
	assert.Nil(t, err)
	assert.Equal(t, "te", string(bin))

	bin, err = resource.Read(1, 3)
	assert.Nil(t, err)
	assert.Equal(t, "ext", string(bin))
}

func TestFileFetcherOutOfRangeClamping(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/file_href"})
	bin, err := resource.Read(-5, 60)
	assert.Nil(t, err)
	assert.Equal(t, "text", string(bin))
}

func TestFileFetcherDecreasingRange(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/file_href"})
	_, err := resource.Read(60, 20)
	assert.Equal(t, RangeNotSatisfiable(err.Cause), err, "range isn't satisfiable")
}

func TestFileFetcherComputingLength(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/file_href"})
	length, err := resource.Length()
	assert.Nil(t, err)
	assert.EqualValues(t, 4, length)
}

func TestFileFetcherDirectoryLengthNotFound(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/dir_href/subdirectory"})
	_, err := resource.Length()
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherFileNotFoundLength(t *testing.T) {
	resource := testFileFetcher.Get(manifest.Link{Href: "/unknown"})
	_, err := resource.Length()
	assert.Equal(t, NotFound(err.Cause), err)
}

func TestFileFetcherLinks(t *testing.T) {
	links, err := testFileFetcher.Links()
	assert.Nil(t, err)

	mustContain := []manifest.Link{{
		Href: "/dir_href/subdirectory/hello.mp3",
		Type: "audio/mpeg",
	}, {
		Href: "/dir_href/subdirectory/text2.txt",
		Type: "text/plain",
	}, {
		Href: "/dir_href/text1.txt",
		Type: "text/plain",
	}, {
		Href: "/file_href",
		Type: "text/plain",
	}}

	assert.ElementsMatch(t, mustContain, links)
}
