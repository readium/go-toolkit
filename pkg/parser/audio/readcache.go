package audio

import (
	"context"
	"sync"

	"github.com/readium/go-toolkit/pkg/fetcher"
)

// defaultCacheBlockSize is the granularity at which readCache fetches data when
// no size is configured. Reads are rounded up to whole blocks so that the many
// small, overlapping reads performed while probing an audio file coalesce into a
// few range requests.
const defaultCacheBlockSize = 256 << 10 // 256 KiB

// readCache wraps a fetcher.Resource and serves reads from fixed-size blocks
// fetched on demand.
//
// Probing an audio file performs many small reads — tag headers, container
// boxes, duration markers, chapter samples — and the tag and duration passes
// both re-read the header region. On remote sources (HTTP, S3) and ZIP archives
// every read is a byte-range request, so without caching opening a multi-file
// audiobook fans out into hundreds of requests. Coalescing reads into block
// fetches (and reusing them across passes) cuts that to a handful per file.
//
// Reads may happen concurrently (chapter-title runs are fetched in parallel),
// so block map access is guarded by a mutex. Concurrent fetches of the same
// missing block may duplicate a request; that is harmless and rare, and beats
// holding the lock across remote I/O.
type readCache struct {
	fetcher.Resource
	size      int64
	blockSize int64
	retain    bool // Route chapter-sample reads through the cache too, see WithRetainedCache

	mu     sync.RWMutex
	blocks map[int64][]byte
}

func newReadCache(res fetcher.Resource, size, blockSize int64, retain bool) *readCache {
	if blockSize <= 0 {
		blockSize = defaultCacheBlockSize
	}
	return &readCache{Resource: res, size: size, blockSize: blockSize, retain: retain, blocks: make(map[int64][]byte)}
}

// Length returns the known size without hitting the underlying resource.
func (c *readCache) Length(ctx context.Context) (int64, *fetcher.ResourceError) {
	if c.size >= 0 {
		return c.size, nil
	}
	return c.Resource.Length(ctx)
}

// Read serves the inclusive byte range [start, end] from cached blocks, fetching
// any missing blocks first.
func (c *readCache) Read(ctx context.Context, start, end int64) ([]byte, *fetcher.ResourceError) {
	// Whole-resource reads, or an unknown size, bypass the block cache.
	if (start == 0 && end == 0) || c.size <= 0 || end < start {
		return c.Resource.Read(ctx, start, end)
	}
	if start >= c.size {
		return []byte{}, nil
	}
	if end >= c.size {
		end = c.size - 1
	}

	firstBlock := start / c.blockSize
	lastBlock := end / c.blockSize
	if err := c.fetch(ctx, firstBlock, lastBlock); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]byte, 0, end-start+1)
	for b := firstBlock; b <= lastBlock; b++ {
		blk := c.blocks[b]
		base := b * c.blockSize
		lo := int64(0)
		if start > base {
			lo = start - base
		}
		hi := min(end-base+1, int64(len(blk)))
		if lo < hi {
			out = append(out, blk[lo:hi]...)
		}
	}
	return out, nil
}

// cachedSlice returns the bytes for the inclusive range [start, end] if they are
// already fully present in the cache, without performing any read. The second
// return value reports whether it was a hit. The returned slice is a copy.
func (c *readCache) cachedSlice(start, end int64) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return (&retainedCache{size: c.size, blockSize: c.blockSize, blocks: c.blocks}).slice(start, end)
}

// snapshot returns the cache's blocks for retention on the publication after
// probing finishes. The block map is shared, not copied: no writes happen once
// probing is done, and retainedCache is read-only.
func (c *readCache) snapshot() *retainedCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.blocks) == 0 || c.size <= 0 {
		return nil
	}
	return &retainedCache{size: c.size, blockSize: c.blockSize, blocks: c.blocks}
}

// fetch ensures every block in [first, last] is cached, retrieving each run of
// contiguous missing blocks in a single underlying read.
func (c *readCache) fetch(ctx context.Context, first, last int64) *fetcher.ResourceError {
	for b := first; b <= last; {
		if c.hasBlock(b) {
			b++
			continue
		}
		runStart := b
		for b <= last {
			if c.hasBlock(b) {
				break
			}
			b++
		}
		runEnd := b - 1

		off := runStart * c.blockSize
		endOff := (runEnd+1)*c.blockSize - 1
		if endOff >= c.size {
			endOff = c.size - 1
		}
		data, err := c.Resource.Read(ctx, off, endOff)
		if err != nil {
			return err
		}
		c.mu.Lock()
		for bi := runStart; bi <= runEnd; bi++ {
			lo := (bi - runStart) * c.blockSize
			if lo >= int64(len(data)) {
				c.blocks[bi] = []byte{}
				continue
			}
			hi := lo + c.blockSize
			if hi > int64(len(data)) {
				hi = int64(len(data))
			}
			c.blocks[bi] = data[lo:hi]
		}
		c.mu.Unlock()
	}
	return nil
}

func (c *readCache) hasBlock(b int64) bool {
	c.mu.RLock()
	_, ok := c.blocks[b]
	c.mu.RUnlock()
	return ok
}
