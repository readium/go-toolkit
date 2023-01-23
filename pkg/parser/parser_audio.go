package parser

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Handles parsing of audiobooks from an unstructured archive format containing audio files, such as ZAB (Zipped Audio Book) or a simple ZIP.
// It can also work for a standalone audio file.
type AudioParser struct{}

// Parse implements PublicationParser
func (p AudioParser) Parse(asset asset.PublicationAsset, fetcher fetcher.Fetcher) (*pub.Builder, error) {
	if !p.accepts(asset, fetcher) {
		return nil, nil
	}

	links, err := fetcher.Links()
	if err != nil {
		return nil, err
	}
	readingOrder := make(manifest.LinkList, 0, len(links))
	for _, link := range links {
		// Filter out all irrelevant files
		fext := filepath.Ext(strings.ToLower(link.Href))
		if len(fext) > 1 {
			fext = fext[1:] // Remove "." from extension
		}
		_, contains := allowed_extensions_audio[fext]
		if extensions.IsHiddenOrThumbs(link.Href) || !contains {
			continue
		}
		readingOrder = append(readingOrder, link)
	}

	if len(readingOrder) == 0 {
		return nil, errors.New("no audio file found in the publication")
	}

	// Sort in alphabetical order
	sort.Slice(readingOrder, func(i, j int) bool {
		return readingOrder[i].Href < readingOrder[j].Href
	})

	// Try to figure out the publication's title
	title := guessPublicationTitleFromFileStructure(fetcher)
	if title == "" {
		title = asset.Name()
	}

	manifest := manifest.Manifest{
		Context: manifest.Strings{manifest.WebpubManifestContext},
		Metadata: manifest.Metadata{
			LocalizedTitle: manifest.NewLocalizedStringFromString(title),
			ConformsTo:     manifest.Profiles{manifest.ProfileAudiobook},
		},
		ReadingOrder: readingOrder,
	}

	return pub.NewBuilder(manifest, fetcher, nil), nil // TODO services!
}

var allowed_extensions_audio_extra = map[string]struct{}{
	"asx": {}, "bio": {}, "m3u": {}, "m3u8": {}, "pla": {}, "pls": {},
	"smil": {}, "txt": {}, "vlc": {}, "wpl": {}, "xspf": {}, "zpl": {},
}
var allowed_extensions_audio = map[string]struct{}{
	"aac": {}, "aiff": {}, "alac": {}, "flac": {}, "m4a": {}, "m4b": {}, "mp3": {},
	"ogg": {}, "oga": {}, "mogg": {}, "opus": {}, "wav": {}, "webm": {},
}

func (p AudioParser) accepts(asset asset.PublicationAsset, fetcher fetcher.Fetcher) bool {
	if asset.MediaType().Equal(&mediatype.ZAB) {
		return true
	}
	links, err := fetcher.Links()
	if err != nil {
		// TODO log
		return false
	}
	for _, link := range links {
		if extensions.IsHiddenOrThumbs(link.Href) {
			continue
		}
		if link.MediaType().IsBitmap() {
			continue
		}
		fext := filepath.Ext(strings.ToLower(link.Href))
		if len(fext) > 1 {
			fext = fext[1:] // Remove "." from extension
		}
		_, contains1 := allowed_extensions_audio[fext]
		_, contains2 := allowed_extensions_audio_extra[fext]
		if !contains1 && !contains2 {
			return false
		}
	}
	return true
}
