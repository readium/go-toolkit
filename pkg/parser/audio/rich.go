package audio

import (
	"context"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/pub"
	"golang.org/x/sync/errgroup"
)

// defaultProbeConcurrency bounds how many reading-order files are probed in
// parallel by default. Probing is I/O-bound (range requests to a remote source
// or archive), so a small amount of concurrency hides per-file latency without
// flooding the backend with requests.
const defaultProbeConcurrency = 8

// enrich performs the rich-parsing pass over an audiobook: it probes every
// reading-order resource for its duration and bitrate, extracts publication
// metadata (and a cover) from the first audio file, and builds a table of
// contents. It mutates m in place and returns a cover service factory (or nil
// when no cover was found), plus — when the parser retains probe caches — the
// per-HREF caches to attach to the publication.
func (p AudioParser) enrich(ctx context.Context, fetch fetcher.Fetcher, m *manifest.Manifest) (pub.ServiceFactory, map[string]*retainedCache) {
	var firstTags *audioTags
	var totalDuration float64
	var embeddedChapters manifest.LinkList

	readingOrder := m.ReadingOrder

	// Acquire all resource handles serially: Get is cheap, but not guaranteed to
	// be safe for concurrent use (e.g. FileFetcher tracks handles in a shared
	// slice). The expensive part — reading each file — is then parallelized.
	resources := make([]fetcher.Resource, len(readingOrder))
	for i := range readingOrder {
		resources[i] = fetch.Get(ctx, readingOrder[i])
	}

	// Probe files concurrently, bounded by the configured concurrency. Reading
	// distinct resources in parallel is safe for the file and archive fetchers,
	// and hides per-file round-trip latency on remote sources.
	concurrency := p.concurrency
	if concurrency <= 0 {
		concurrency = defaultProbeConcurrency
	}
	probed := make([]probedItem, len(readingOrder))
	var g errgroup.Group
	g.SetLimit(concurrency)
	extractChapters := !p.skipEmbeddedChapters
	blockSize := int64(p.cacheBlockSize)
	for i := range readingOrder {
		i, link, res := i, readingOrder[i], resources[i]
		g.Go(func() error {
			probed[i] = probeReadingOrderItem(ctx, res, link, extractChapters, blockSize, concurrency, p.retainCache)
			return nil
		})
	}
	_ = g.Wait()

	// Combine the results in reading order so the output is deterministic.
	var caches map[string]*retainedCache
	for i := range readingOrder {
		link := readingOrder[i]
		p := probed[i]

		if p.cache != nil {
			if caches == nil {
				caches = make(map[string]*retainedCache, len(readingOrder))
			}
			caches[link.Href.String()] = p.cache
		}

		if p.probe.Duration > 0 {
			link.Duration = p.probe.Duration
			totalDuration += p.probe.Duration
		}
		if p.probe.Bitrate > 0 {
			link.Bitrate = p.probe.Bitrate
		}
		if p.tags != nil && p.tags.Title != "" {
			link.Title = p.tags.Title
		}
		if len(p.probe.Chapters) > 0 {
			embeddedChapters = append(embeddedChapters, chaptersToLinks(p.probe.Chapters, link)...)
		}

		readingOrder[i] = link
		if i == 0 {
			firstTags = p.tags
		}
	}
	m.ReadingOrder = readingOrder

	if totalDuration > 0 {
		d := totalDuration
		m.Metadata.Duration = &d
	}

	applyTagsToMetadata(&m.Metadata, firstTags)

	// Table of contents priority: a playlist file wins over embedded chapters,
	// which in turn win over a flat per-file listing.
	if toc := playlistTOC(ctx, fetch, readingOrder); len(toc) > 0 {
		m.TableOfContents = toc
	} else if len(embeddedChapters) > 0 {
		m.TableOfContents = embeddedChapters
	} else if toc := perFileTOC(readingOrder); len(toc) > 0 {
		m.TableOfContents = toc
	}

	if firstTags != nil {
		return coverServiceFactory(firstTags.Picture), caches
	}
	return nil, caches
}

// probedItem is the per-file result of the parallel probing pass.
type probedItem struct {
	tags  *audioTags
	probe probeResult
	cache *retainedCache // Blocks fetched while probing, when retention is enabled
}

// probeReadingOrderItem reads the tags and probes the duration/bitrate/chapters
// of a single reading-order resource, always releasing the resource handle.
// concurrency bounds the parallel reads used within the file (e.g. fetching
// scattered chapter samples).
//
// All reads go through a per-file block cache so that the tag and duration
// passes — which perform many small, overlapping reads of the header region —
// coalesce into a handful of range requests rather than one request each. This
// matters most for remote sources (HTTP, S3) and ZIP archives.
func probeReadingOrderItem(ctx context.Context, res fetcher.Resource, link manifest.Link, extractChapters bool, blockSize int64, concurrency int, retain bool) probedItem {
	defer res.Close()
	size, _ := res.Length(ctx)
	cached := newReadCache(res, size, blockSize, retain)
	tags := readAudioTags(cached)
	item := probedItem{tags: tags, probe: probeAudioFile(ctx, cached, link, tags, extractChapters, concurrency)}
	if retain {
		item.cache = cached.snapshot()
	}
	return item
}

// playlistTOC looks for the first parseable playlist file in the publication and
// converts it into a table of contents whose entries map to the reading order.
func playlistTOC(ctx context.Context, fetch fetcher.Fetcher, readingOrder manifest.LinkList) manifest.LinkList {
	links, err := fetch.Links(ctx)
	if err != nil {
		return nil
	}

	playlists := make(manifest.LinkList, 0)
	for _, l := range links {
		if _, ok := allowed_extensions_audio_extra[linkExtension(l)]; ok {
			playlists = append(playlists, l)
		}
	}
	sort.Slice(playlists, func(i, j int) bool {
		return playlists[i].Href.String() < playlists[j].Href.String()
	})

	for _, l := range playlists {
		content := readResource(ctx, fetch, l)
		if content == nil {
			continue
		}
		entries := parsePlaylist(linkExtension(l), content)
		if len(entries) == 0 {
			continue
		}
		if toc := playlistEntriesToTOC(entries, readingOrder); len(toc) > 0 {
			return toc
		}
	}
	return nil
}

// readResource reads the full content of a link's resource, always releasing the
// resource handle (even on a read error or panic).
func readResource(ctx context.Context, fetch fetcher.Fetcher, link manifest.Link) []byte {
	res := fetch.Get(ctx, link)
	defer res.Close()
	content, err := res.Read(ctx, 0, 0)
	if err != nil {
		return nil
	}
	return content
}

// playlistEntriesToTOC maps playlist entries onto reading-order resources by
// matching file names, producing one TOC link per resolvable entry.
func playlistEntriesToTOC(entries []playlistEntry, readingOrder manifest.LinkList) manifest.LinkList {
	byName := make(map[string]manifest.Link, len(readingOrder))
	for _, l := range readingOrder {
		byName[strings.ToLower(l.URL(nil, nil).Filename())] = l
	}

	toc := make(manifest.LinkList, 0, len(entries))
	for _, e := range entries {
		name := playlistEntryName(e.Path)
		l, ok := byName[name]
		if !ok {
			continue
		}
		title := e.Title
		if title == "" {
			title = l.Title
		}
		toc = append(toc, chapterLink(l, title, e.Start))
	}
	return toc
}

// playlistEntryName extracts the lower-cased file name from a playlist entry
// path, ignoring any query string and decoding percent-escapes.
func playlistEntryName(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	base := path.Base(p)
	if decoded, err := url.PathUnescape(base); err == nil {
		base = decoded
	}
	return strings.ToLower(base)
}

// perFileTOC builds a flat table of contents with one entry per reading-order
// resource. It is used as a last resort when no playlist or embedded chapters
// are available, and only when there is more than one titled resource.
func perFileTOC(readingOrder manifest.LinkList) manifest.LinkList {
	if len(readingOrder) < 2 {
		return nil
	}
	titled := false
	for _, l := range readingOrder {
		if l.Title != "" {
			titled = true
			break
		}
	}
	if !titled {
		return nil
	}

	toc := make(manifest.LinkList, 0, len(readingOrder))
	for _, l := range readingOrder {
		title := l.Title
		if title == "" {
			title = l.URL(nil, nil).Filename()
		}
		toc = append(toc, chapterLink(l, title, 0))
	}
	return toc
}
