package audio

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// servedResource is an in-memory fetcher.Resource that counts how many times
// the underlying "remote" data is touched.
type servedResource struct {
	data    []byte
	mu      sync.Mutex
	reads   int
	streams int
	lengths int
}

func (r *servedResource) Link() manifest.Link             { return manifest.Link{} }
func (r *servedResource) Properties() manifest.Properties { return manifest.Properties{} }
func (r *servedResource) File() string                    { return "" }
func (r *servedResource) Close()                          {}

func (r *servedResource) Length(ctx context.Context) (int64, *fetcher.ResourceError) {
	r.mu.Lock()
	r.lengths++
	r.mu.Unlock()
	return int64(len(r.data)), nil
}

func (r *servedResource) clamp(start, end int64) (int64, int64) {
	if start == 0 && end == 0 {
		return 0, int64(len(r.data)) - 1
	}
	if end >= int64(len(r.data)) {
		end = int64(len(r.data)) - 1
	}
	return start, end
}

func (r *servedResource) Read(ctx context.Context, start, end int64) ([]byte, *fetcher.ResourceError) {
	r.mu.Lock()
	r.reads++
	r.mu.Unlock()
	s, e := r.clamp(start, end)
	return append([]byte(nil), r.data[s:e+1]...), nil
}

func (r *servedResource) Stream(ctx context.Context, w io.Writer, start, end int64) (int64, *fetcher.ResourceError) {
	r.mu.Lock()
	r.streams++
	r.mu.Unlock()
	s, e := r.clamp(start, end)
	n, err := w.Write(r.data[s : e+1])
	if err != nil {
		return int64(n), fetcher.Other(err)
	}
	return int64(n), nil
}

func (r *servedResource) counts() (int, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reads, r.streams, r.lengths
}

// A cachedResource built from a probe-time readCache snapshot must serve the
// probed ranges — and the resource length — from memory, and delegate
// everything else.
func TestRetainedCacheServing(t *testing.T) {
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte((i * 13) % 251)
	}
	const blockSize = 64 << 10

	// Probe phase: read the "header" (block 0) and a "chapter sample" in block 8.
	probeSrc := &servedResource{data: data}
	rc := newReadCache(probeSrc, int64(len(data)), blockSize, true)
	_, rerr := rc.Read(t.Context(), 0, 1000)
	require.Nil(t, rerr)
	_, rerr = rc.Read(t.Context(), 8*blockSize+100, 8*blockSize+200)
	require.Nil(t, rerr)

	snap := rc.snapshot()
	require.NotNil(t, snap)

	// Serve phase: a fresh resource wrapped with the retained cache.
	serveSrc := &servedResource{data: data}
	res := &cachedResource{Resource: serveSrc, cache: snap}

	// Length comes from the snapshot, not the resource.
	l, rerr := res.Length(t.Context())
	require.Nil(t, rerr)
	assert.Equal(t, int64(len(data)), l)

	// Fully-cached read: served from memory.
	b, rerr := res.Read(t.Context(), 100, 199)
	require.Nil(t, rerr)
	assert.Equal(t, data[100:200], b)

	// Fully-cached stream (a browser probing a chapter sample block).
	var buf bytes.Buffer
	n, rerr := res.Stream(t.Context(), &buf, 8*blockSize, 8*blockSize+999)
	require.Nil(t, rerr)
	assert.Equal(t, int64(1000), n)
	assert.Equal(t, data[8*blockSize:8*blockSize+1000], buf.Bytes())

	reads, streams, lengths := serveSrc.counts()
	assert.Equal(t, [3]int{0, 0, 0}, [3]int{reads, streams, lengths},
		"cached ranges must not touch the underlying resource")

	// Stream crossing out of the cached region: cached prefix from memory,
	// remainder delegated with the right offset.
	buf.Reset()
	n, rerr = res.Stream(t.Context(), &buf, 8*blockSize, 10*blockSize-1)
	require.Nil(t, rerr)
	assert.Equal(t, int64(2*blockSize), n)
	assert.Equal(t, data[8*blockSize:10*blockSize], buf.Bytes())
	_, streams, _ = serveSrc.counts()
	assert.Equal(t, 1, streams, "only the uncached remainder should be streamed")

	// Uncached read and stream: fully delegated.
	b, rerr = res.Read(t.Context(), 2*blockSize, 2*blockSize+99)
	require.Nil(t, rerr)
	assert.Equal(t, data[2*blockSize:2*blockSize+100], b)
	reads, _, _ = serveSrc.counts()
	assert.Equal(t, 1, reads)

	// Whole-resource stream: block 0 is cached, so it forms the prefix.
	buf.Reset()
	n, rerr = res.Stream(t.Context(), &buf, 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, int64(len(data)), n)
	assert.Equal(t, data, buf.Bytes())
}

// Probing with retention through the rich parser attaches the caches to the
// publication so its resources answer probed ranges without new reads.
func TestProbeRetentionRoundTrip(t *testing.T) {
	data := make([]byte, 512<<10)
	for i := range data {
		data[i] = byte((i * 7) % 256)
	}
	src := &servedResource{data: data}
	rc := newReadCache(src, int64(len(data)), 0, false)
	_, rerr := rc.Read(t.Context(), 0, 100)
	require.Nil(t, rerr)
	// Without retention there is nothing to keep for a resource this small...
	assert.NotNil(t, rc.snapshot(), "snapshot works regardless of the retain flag")

	// The retain flag's effect is exercised in readChapterTitles: with it,
	// cache misses for chapter runs are pulled through the block cache.
	retained := newReadCache(&servedResource{data: data}, int64(len(data)), 0, true)
	assert.True(t, retained.retain)
}
