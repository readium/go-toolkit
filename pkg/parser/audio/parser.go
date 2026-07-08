package audio

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/parser"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Handles parsing of audiobooks from an unstructured archive format containing audio files, such as ZAB (Zipped Audio Book) or a simple ZIP.
// It can also work for a standalone audio file.
type AudioParser struct {
	rich bool // Whether to attempt extraction of metadata (duration, cover etc.) from the audio files

	// skipEmbeddedChapters disables building the table of contents from chapter
	// markers embedded in the audio files. Extracting them can be expensive on
	// remote sources — some files scatter chapter title samples throughout the
	// stream, costing one range request each — so callers that prioritize fast
	// opening can turn it off. A playlist or per-file titles are still used.
	skipEmbeddedChapters bool

	// cacheBlockSize sets the granularity (in bytes) of the per-file read cache
	// used while probing. Zero means the default. Larger blocks make fewer, bigger
	// range requests; smaller blocks transfer less for scattered reads.
	cacheBlockSize int

	// concurrency caps how many audio files are probed in parallel, and how many
	// parallel reads are used within a file (e.g. fetching scattered chapter
	// samples). Zero means the default. Higher values hide more per-file latency
	// on remote sources at the cost of more in-flight requests.
	concurrency int
}

// Option configures an AudioParser.
type Option func(*AudioParser)

// WithoutEmbeddedChapters disables extracting the table of contents from chapter
// markers embedded in the audio files (e.g. an MP4 chapter track or Vorbis
// CHAPTER comments). This avoids the extra reads they require; the TOC then
// comes from a playlist or per-file titles when available.
func WithoutEmbeddedChapters() Option {
	return func(p *AudioParser) { p.skipEmbeddedChapters = true }
}

// WithCacheBlockSize sets the block size (in bytes) of the per-file read cache
// used while probing audio files for rich metadata. The default is 256 KiB. A
// value <= 0 is ignored and keeps the default. Larger blocks coalesce more reads
// into each range request (fewer requests, more bytes); smaller blocks transfer
// less when reads are scattered.
func WithCacheBlockSize(size int) Option {
	return func(p *AudioParser) {
		if size > 0 {
			p.cacheBlockSize = size
		}
	}
}

// WithConcurrency sets how many audio files are probed in parallel while
// extracting rich metadata, and also bounds the parallel reads used within a
// single file (e.g. fetching the scattered chapter-title samples of an MP4
// chapter track). The default is 8. A value <= 0 is ignored and keeps the
// default. Use 1 to probe and read fully sequentially. Note that both levels
// can be in flight at once, so the worst-case number of concurrent requests is
// roughly the square of this value.
func WithConcurrency(n int) Option {
	return func(p *AudioParser) {
		if n > 0 {
			p.concurrency = n
		}
	}
}

func NewParser() AudioParser {
	return AudioParser{}
}

func NewRichParser(opts ...Option) AudioParser {
	p := AudioParser{rich: true}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

// Parse implements PublicationParser
func (p AudioParser) Parse(ctx context.Context, asset asset.PublicationAsset, fetcher fetcher.Fetcher) (*pub.Builder, error) {
	if !p.accepts(ctx, asset, fetcher) {
		return nil, nil
	}

	links, err := fetcher.Links(ctx)
	if err != nil {
		return nil, err
	}
	readingOrder := make(manifest.LinkList, 0, len(links))
	for _, link := range links {
		path := link.URL(nil, nil).Path()

		// Filter out all irrelevant files
		fext := filepath.Ext(strings.ToLower(path))
		if len(fext) > 1 {
			fext = fext[1:] // Remove "." from extension
		}
		_, contains := allowed_extensions_audio[fext]
		if extensions.IsHiddenOrThumbs(path) || !contains {
			continue
		}
		readingOrder = append(readingOrder, link)
	}

	if len(readingOrder) == 0 {
		return nil, errors.New("no audio file found in the publication")
	}

	// Sort in alphabetical order
	sort.Slice(readingOrder, func(i, j int) bool {
		return readingOrder[i].Href.String() < readingOrder[j].Href.String()
	})

	// Try to figure out the publication's title
	title := parser.GuessPublicationTitleFromFileStructure(ctx, fetcher)
	if title == "" {
		title = asset.Name()
	}

	man := manifest.Manifest{
		Context: manifest.Strings{manifest.WebpubManifestContext},
		Metadata: manifest.Metadata{
			Type:           "http://schema.org/Audiobook",
			LocalizedTitle: manifest.NewLocalizedStringFromString(title),
			ConformsTo:     manifest.Profiles{manifest.ProfileAudiobook},
		},
		ReadingOrder: readingOrder,
	}

	serviceFactories := map[pub.ServiceName]pub.ServiceFactory{}

	// When rich parsing is enabled, probe the audio files for durations,
	// bitrates, embedded metadata, a cover and a table of contents.
	if p.rich {
		if coverFactory := p.enrich(ctx, fetcher, &man); coverFactory != nil {
			serviceFactories[pub.CoverService_Name] = coverFactory
		}
	}

	var builder *pub.ServicesBuilder
	if len(serviceFactories) > 0 {
		builder = pub.NewServicesBuilder(serviceFactories)
	}
	return pub.NewBuilder(man, fetcher, builder), nil
}

var allowed_extensions_audio_extra = map[string]struct{}{
	"asx": {}, "bio": {}, "m3u": {}, "m3u8": {}, "pla": {}, "pls": {},
	"smil": {}, "txt": {}, "vlc": {}, "wpl": {}, "xspf": {}, "zpl": {},
	"cue": {}, "log": {},
}
var allowed_extensions_audio = map[string]struct{}{
	"aac": {}, "aiff": {}, "aif": {}, "aifc": {}, "alac": {}, "flac": {},
	"m4a": {}, "m4b": {}, "mp3": {}, "mp4": {}, "m4r": {}, "m4p": {},
	"ogg": {}, "oga": {}, "mogg": {}, "opus": {}, "wav": {}, "wave": {},
	"webm": {},
}

func (p AudioParser) accepts(ctx context.Context, asset asset.PublicationAsset, fetcher fetcher.Fetcher) bool {
	if asset.MediaType(ctx).Equal(&mediatype.ZAB) {
		return true
	}
	links, err := fetcher.Links(ctx)
	if err != nil {
		// TODO log
		return false
	}
	for _, link := range links {
		path := link.URL(nil, nil).Path()

		if extensions.IsHiddenOrThumbs(path) {
			continue
		}
		if link.MediaType.IsBitmap() {
			continue
		}
		fext := filepath.Ext(strings.ToLower(path))
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
