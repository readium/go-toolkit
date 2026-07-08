package audio

import (
	"context"
	"io"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// retainedCache is a read-only snapshot of the blocks a readCache fetched while
// probing an audio file. When [WithRetainedCache] is enabled it stays attached
// to the publication so that serving the file can answer the same byte ranges
// from memory. Those ranges are exactly what a browser's demuxer asks for
// before starting playback: the container headers and the chapter-title
// samples scattered through the file.
type retainedCache struct {
	size      int64
	blockSize int64
	blocks    map[int64][]byte
}

// slice returns a copy of the inclusive range [start, end] if it is fully
// covered by cached blocks.
func (c *retainedCache) slice(start, end int64) ([]byte, bool) {
	if c.size <= 0 || start < 0 || end < start {
		return nil, false
	}
	if end >= c.size {
		end = c.size - 1
	}

	firstBlock := start / c.blockSize
	lastBlock := end / c.blockSize
	for b := firstBlock; b <= lastBlock; b++ {
		blk, ok := c.blocks[b]
		if !ok {
			return nil, false
		}
		// The needed portion of this block must actually be present (the final
		// cached block may be short if it sits at the end of the resource).
		base := b * c.blockSize
		needEnd := end
		if blockEnd := base + c.blockSize - 1; needEnd > blockEnd {
			needEnd = blockEnd
		}
		if int64(len(blk)) <= needEnd-base {
			return nil, false
		}
	}

	out := make([]byte, 0, end-start+1)
	for b := firstBlock; b <= lastBlock; b++ {
		blk := c.blocks[b]
		base := b * c.blockSize
		lo := int64(0)
		if start > base {
			lo = start - base
		}
		hi := min(end-base+1, int64(len(blk)))
		out = append(out, blk[lo:hi]...)
	}
	return out, true
}

// prefix returns a copy of the longest cached run of bytes beginning exactly at
// start, capped at end (inclusive). It returns nil when the byte at start is
// not cached.
func (c *retainedCache) prefix(start, end int64) []byte {
	if c.size <= 0 || start < 0 || end < start || start >= c.size {
		return nil
	}
	if end >= c.size {
		end = c.size - 1
	}

	var out []byte
	for pos := start; pos <= end; {
		b := pos / c.blockSize
		blk, ok := c.blocks[b]
		if !ok {
			break
		}
		base := b * c.blockSize
		lo := pos - base
		hi := min(end-base+1, int64(len(blk)))
		if lo >= hi {
			break
		}
		out = append(out, blk[lo:hi]...)
		pos = base + hi
		if hi < int64(len(blk)) || pos > end {
			break
		}
	}
	return out
}

// cacheFetcher wraps the publication's fetcher, attaching each reading-order
// resource's retained probe cache to the resources it returns.
type cacheFetcher struct {
	fetcher.Fetcher
	caches map[string]*retainedCache // Keyed by the link's HREF string
}

// Get implements fetcher.Fetcher
func (f *cacheFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	res := f.Fetcher.Get(ctx, link)
	if c, ok := f.caches[link.Href.String()]; ok {
		return &cachedResource{Resource: res, cache: c}
	}
	return res
}

// cachedResource serves a resource's length and any byte ranges covered by the
// retained probe cache from memory, delegating everything else to the wrapped
// resource. It intentionally does not forward the CompressedResource trait:
// audio files are already compressed and virtually never deflated inside
// archives, and hiding the trait only disables a passthrough optimization,
// not correctness.
type cachedResource struct {
	fetcher.Resource
	cache *retainedCache
}

// Length implements fetcher.Resource, using the size learned while probing so
// no remote metadata request is needed.
func (r *cachedResource) Length(ctx context.Context) (int64, *fetcher.ResourceError) {
	if r.cache.size >= 0 {
		return r.cache.size, nil
	}
	return r.Resource.Length(ctx)
}

// Read implements fetcher.Resource, serving fully-cached ranges from memory.
func (r *cachedResource) Read(ctx context.Context, start, end int64) ([]byte, *fetcher.ResourceError) {
	s, e, whole := r.normalize(start, end)
	if !whole {
		if b, ok := r.cache.slice(s, e); ok {
			return b, nil
		}
	}
	return r.Resource.Read(ctx, start, end)
}

// Stream implements fetcher.Resource. The cached prefix of the range is
// written immediately from memory — a browser probing chapter metadata
// typically aborts within it, costing no remote request at all — and only the
// remainder, if the client keeps reading, is streamed from the wrapped
// resource.
func (r *cachedResource) Stream(ctx context.Context, w io.Writer, start, end int64) (int64, *fetcher.ResourceError) {
	s, e, whole := r.normalize(start, end)
	if whole {
		return r.Resource.Stream(ctx, w, start, end)
	}

	pre := r.cache.prefix(s, e)
	if len(pre) == 0 {
		return r.Resource.Stream(ctx, w, start, end)
	}
	n, err := w.Write(pre)
	written := int64(n)
	if err != nil {
		return written, fetcher.Other(err)
	}
	if s+written > e {
		return written, nil
	}
	m, rerr := r.Resource.Stream(ctx, w, s+written, e)
	if m > 0 {
		written += m
	}
	return written, rerr
}

// HasEfficientStream implements fetcher.EfficientStreamer by delegating to the
// wrapped resource: the cached prefix only ever makes Stream cheaper.
func (r *cachedResource) HasEfficientStream() bool {
	if es, ok := r.Resource.(fetcher.EfficientStreamer); ok {
		return es.HasEfficientStream()
	}
	return false
}

// normalize resolves the (0, 0) whole-resource convention against the known
// size, reporting whole == true when the size is unknown and the range can't
// be resolved.
func (r *cachedResource) normalize(start, end int64) (int64, int64, bool) {
	if start == 0 && end == 0 {
		if r.cache.size <= 0 {
			return start, end, true
		}
		return 0, r.cache.size - 1, false
	}
	return start, end, false
}
