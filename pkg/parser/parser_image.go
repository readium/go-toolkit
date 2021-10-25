package parser

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/utils"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Parses an image–based Publication from an unstructured archive format containing bitmap files, such as CBZ or a simple ZIP.
// It can also work for a standalone bitmap file.
type ImageParser struct{}

func (p ImageParser) Parse(asset asset.PublicationAsset, fetcher fetcher.Fetcher) (*pub.Builder, error) {
	if !p.accepts(asset, fetcher) {
		return nil, nil
	}

	links, err := fetcher.Links()
	if err != nil {
		return nil, err
	}
	readingOrder := make([]manifest.Link, 0)
	for _, link := range links {
		// Filter out all irrelevant files
		if utils.IsHiddenOrThumbs(link.Href) || !link.MediaType().IsBitmap() {
			continue
		}
		links = append(links, link)
	}

	if len(readingOrder) == 0 {
		return nil, errors.New("no bitmap found in the publication")
	}

	// Sort in alphabetical order
	sort.Slice(readingOrder, func(i, j int) bool {
		return readingOrder[i].Href < readingOrder[j].Href
	})

	// Try to figure out the publication's title
	title := guessFetcherTitle(fetcher)
	if title == "" {
		title = asset.Name()
	}

	// First valid resource is the cover.
	readingOrder[0].Rels = []string{"cover"}

	manifest := manifest.Manifest{
		Metadata:     manifest.Metadata{LocalizedTitle: manifest.NewLocalizedStringFromString(title)},
		ReadingOrder: readingOrder,
	}

	return pub.NewBuilder(manifest, fetcher), nil // TODO services!
}

var allowed_extensions_image = map[string]struct{}{"acbf": {}, "xml": {}, "txt": {}}

func (p ImageParser) accepts(asset asset.PublicationAsset, fetcher fetcher.Fetcher) bool {
	if asset.MediaType().Equal(&mediatype.CBZ) {
		return true
	}
	links, err := fetcher.Links()
	if err != nil {
		// TODO log
		return false
	}
	for _, link := range links {
		if utils.IsHiddenOrThumbs(link.Href) {
			continue
		}
		if link.MediaType().IsBitmap() {
			continue
		}
		fext := filepath.Ext(strings.ToLower(link.Href))
		if len(fext) > 1 {
			fext = fext[1:] // Remove "." from extension
		}
		_, contains := allowed_extensions_image[fext]
		if !contains {
			return false
		}
	}
	return true
}
