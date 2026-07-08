package audio

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingResource records how many underlying Read calls are made, to verify
// that readCache coalesces requests.
type countingResource struct {
	fetcher.Resource
	data  []byte
	reads int
}

func (r *countingResource) Length(_ context.Context) (int64, *fetcher.ResourceError) {
	return int64(len(r.data)), nil
}

func (r *countingResource) Read(_ context.Context, start, end int64) ([]byte, *fetcher.ResourceError) {
	r.reads++
	if end >= int64(len(r.data)) {
		end = int64(len(r.data)) - 1
	}
	if start > end {
		return []byte{}, nil
	}
	return r.data[start : end+1], nil
}

func TestReadCacheCorrectnessAndCoalescing(t *testing.T) {
	data := make([]byte, 700<<10) // 700 KiB -> blocks 0,1,2 (256 KiB each)
	for i := range data {
		data[i] = byte(i)
	}
	base, _ := bytesResource(data)
	src := &countingResource{Resource: base, data: data}
	c := newReadCache(src, int64(len(data)), 0, false) // 0 -> default 256 KiB blocks
	require.EqualValues(t, defaultCacheBlockSize, c.blockSize)
	ctx := t.Context()

	read := func(start, end int64) []byte {
		b, err := c.Read(ctx, start, end)
		require.Nil(t, err)
		return b
	}

	// First read of block 0.
	assert.Equal(t, data[0:10], read(0, 9))
	assert.Equal(t, 1, src.reads)

	// Subsequent reads within block 0 hit the cache (no new request).
	assert.Equal(t, data[100:201], read(100, 200))
	assert.Equal(t, 1, src.reads)

	// A read into block 1 triggers exactly one more request.
	assert.Equal(t, data[300<<10:(300<<10)+50], read(300<<10, (300<<10)+49))
	assert.Equal(t, 2, src.reads)

	// A span across blocks 0-2: only block 2 is missing -> one more request.
	assert.Equal(t, data[0:600<<10], read(0, (600<<10)-1))
	assert.Equal(t, 3, src.reads)

	// Reading past EOF returns the clamped tail, not an error.
	got := read(int64(len(data))-5, int64(len(data))+100)
	assert.Equal(t, data[len(data)-5:], got)
}

func TestReadCacheCustomBlockSize(t *testing.T) {
	data := make([]byte, 400<<10)
	for i := range data {
		data[i] = byte(i)
	}
	base, _ := bytesResource(data)
	src := &countingResource{Resource: base, data: data}
	c := newReadCache(src, int64(len(data)), 128<<10, false) // 128 KiB blocks
	require.EqualValues(t, 128<<10, c.blockSize)
	ctx := t.Context()

	b, err := c.Read(ctx, 0, 9)
	require.Nil(t, err)
	assert.Equal(t, data[0:10], b)
	assert.Equal(t, 1, src.reads) // block 0

	// Still within block 0 (< 128 KiB): cached.
	_, err = c.Read(ctx, 100, 200)
	require.Nil(t, err)
	assert.Equal(t, 1, src.reads)

	// At 200 KiB: block 1 with 128 KiB blocks (would be block 0 at 256 KiB).
	b, err = c.Read(ctx, 200<<10, (200<<10)+9)
	require.Nil(t, err)
	assert.Equal(t, data[200<<10:(200<<10)+10], b)
	assert.Equal(t, 2, src.reads)
}
