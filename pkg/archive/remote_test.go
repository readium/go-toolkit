package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedRead captures one ReadRange call and how many bytes were actually
// consumed from the reader it returned.
type recordedRead struct {
	offset   int64
	length   int64
	consumed int64
}

// countingRemoteReader is an in-memory RemoteArchiveReader that records every
// range request and the number of bytes read from it, so tests can assert how
// much a remote archive would really have transferred.
type countingRemoteReader struct {
	data  []byte
	reads []*recordedRead
}

func (r *countingRemoteReader) Size() int64 {
	return int64(len(r.data))
}

func (r *countingRemoteReader) ReadRange(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	rec := &recordedRead{offset: offset, length: length}
	r.reads = append(r.reads, rec)
	end := int64(len(r.data))
	if length >= 0 && offset+length < end {
		end = offset + length
	}
	if offset > end {
		offset = end
	}
	return &countingReadCloser{r: bytes.NewReader(r.data[offset:end]), rec: rec}, nil
}

func (r *countingRemoteReader) reset() {
	r.reads = nil
}

func (r *countingRemoteReader) totalConsumed() int64 {
	var total int64
	for _, rec := range r.reads {
		total += rec.consumed
	}
	return total
}

type countingReadCloser struct {
	r   *bytes.Reader
	rec *recordedRead
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.rec.consumed += int64(n)
	return n, err
}

func (c *countingReadCloser) Close() error {
	return nil
}

// buildRemoteTestZIP creates an in-memory ZIP with a small stored entry, a
// large stored entry (media-like), and a small deflated entry, returning the
// archive bytes and the content of the large entry.
func buildRemoteTestZIP(t *testing.T, bigSize int) (zipBytes []byte, big []byte) {
	t.Helper()
	big = make([]byte, bigSize)
	for i := range big {
		big[i] = byte((i * 7) % 251)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	small, err := zw.CreateHeader(&zip.FileHeader{Name: "small.bin", Method: zip.Store})
	require.NoError(t, err)
	smallData := bytes.Repeat([]byte("readium!"), 1280) // 10 KiB
	_, err = small.Write(smallData)
	require.NoError(t, err)

	bigW, err := zw.CreateHeader(&zip.FileHeader{Name: "big.bin", Method: zip.Store})
	require.NoError(t, err)
	_, err = bigW.Write(big)
	require.NoError(t, err)

	defl, err := zw.CreateHeader(&zip.FileHeader{Name: "small.txt", Method: zip.Deflate})
	require.NoError(t, err)
	_, err = defl.Write([]byte("hello deflated world"))
	require.NoError(t, err)

	require.NoError(t, zw.Close())
	return buf.Bytes(), big
}

// openCountingArchive opens the ZIP the same way the remote (HTTP/S3/GCS)
// archive factories do: through a remoteZIPAdapter with minimizeReads set.
func openCountingArchive(t *testing.T, zipBytes []byte) (*countingRemoteReader, Archive) {
	t.Helper()
	reader := &countingRemoteReader{data: zipBytes}
	rdr := newRemoteZIPAdapter(reader, RemoteArchiveConfig{
		Timeout:             time.Minute,
		CacheSizeThreshold:  1024 * 1024,
		CacheCountThreshold: 32,
		CacheAllThreshold:   1024, // Below the archive size, so nothing is fully cached
	})
	zr, err := zip.NewReader(rdr, int64(len(zipBytes)))
	require.NoError(t, err)
	rdr.makeReady()
	return reader, &gozipArchive{zip: zr, minimizeReads: true, closer: rdr.Close}
}

// A ranged Read of a stored entry must transfer roughly the requested range:
// no reading-and-discarding of the range's prefix, and no draining of the rest
// of the archive after the header probe.
func TestRemoteStoredEntryRangedReadBounded(t *testing.T) {
	zipBytes, big := buildRemoteTestZIP(t, 5<<20)
	reader, a := openCountingArchive(t, zipBytes)

	entry, err := a.Entry("big.bin")
	require.NoError(t, err)
	reader.reset()

	// 100 KiB read starting 4 MiB into the entry
	start := int64(4 << 20)
	length := int64(100 << 10)
	data, err := entry.Read(start, start+length-1)
	require.NoError(t, err)
	assert.Equal(t, big[start:start+length], data)

	// Header probe (30 bytes + bounded drain) plus the requested range;
	// before the fixes this consumed the 4 MiB prefix and the archive tail.
	consumed := reader.totalConsumed()
	assert.LessOrEqual(t, consumed, length+maxDrainBytes+1024,
		"ranged read should not transfer (much) more than the range itself, got %d bytes", consumed)
}

// plainWriter is a writer that deliberately does NOT implement io.ReaderFrom,
// mirroring how an http.ResponseWriter behind middleware may behave, so the
// copy loop's own read granularity is what is measured.
type plainWriter struct {
	buf bytes.Buffer
}

func (w *plainWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// Streaming a range of a stored entry must fetch it in large chunks (one
// remote request per copy buffer) rather than one request per 32 KiB, and a
// second stream of the same entry must not re-fetch the entry header.
func TestRemoteStoredEntryStreamChunked(t *testing.T) {
	zipBytes, big := buildRemoteTestZIP(t, 5<<20)
	reader, a := openCountingArchive(t, zipBytes)

	entry, err := a.Entry("big.bin")
	require.NoError(t, err)
	reader.reset()

	// 3 MiB range: expect one 2 MiB chunk plus one 1 MiB chunk
	start := int64(1 << 20)
	length := int64(3 << 20)
	var buf plainWriter
	n, err := entry.Stream(&buf, start, start+length-1)
	require.NoError(t, err)
	assert.Equal(t, length, n)
	assert.Equal(t, big[start:start+length], buf.buf.Bytes())

	var bodyReads int
	for _, rec := range reader.reads {
		if rec.length >= 1<<20 {
			bodyReads++
		}
	}
	assert.Equal(t, 2, bodyReads, "3 MiB should stream as two large chunks, got reads: %+v", reader.reads)
	// Header probe + two chunks; a flood of small reads (e.g. one per 32 KiB)
	// would mean the copy buffer is being bypassed.
	assert.LessOrEqual(t, len(reader.reads), 3, "unexpected extra reads: %+v", reader.reads)
	assert.LessOrEqual(t, reader.totalConsumed(), length+maxDrainBytes+1024)

	// Second stream: the entry header is now cached (and chunks small enough
	// for the range cache may be too), so at most the body chunks are fetched
	// again — crucially, no extra header request.
	reader.reset()
	buf.buf.Reset()
	_, err = entry.Stream(&buf, start, start+length-1)
	require.NoError(t, err)
	assert.Equal(t, big[start:start+length], buf.buf.Bytes())
	assert.LessOrEqual(t, len(reader.reads), 2, "header should be cached after the first stream, got reads: %+v", reader.reads)
}

// Fully reading a small stored entry precaches it via a single open-ended
// request, which must not drain the remainder of the archive.
func TestRemoteSmallEntryReadDoesNotDrainArchive(t *testing.T) {
	zipBytes, _ := buildRemoteTestZIP(t, 5<<20)
	reader, a := openCountingArchive(t, zipBytes)

	entry, err := a.Entry("small.bin")
	require.NoError(t, err)
	reader.reset()

	data, err := entry.Read(0, 0)
	require.NoError(t, err)
	assert.Equal(t, bytes.Repeat([]byte("readium!"), 1280), data)

	// Entry body is 10 KiB; the big entry follows it in the archive. Before
	// the drain fix this transferred the entire archive tail (> 5 MiB).
	consumed := reader.totalConsumed()
	assert.LessOrEqual(t, consumed, int64(10<<10)+maxDrainBytes+1024,
		"reading a small entry should not drain the archive tail, got %d bytes", consumed)
}

// Deflated entries are still read correctly through the adapter after the
// stored-entry changes.
func TestRemoteDeflatedEntryRead(t *testing.T) {
	zipBytes, _ := buildRemoteTestZIP(t, 2<<20)
	_, a := openCountingArchive(t, zipBytes)

	entry, err := a.Entry("small.txt")
	require.NoError(t, err)
	data, err := entry.Read(0, 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello deflated world"), data)

	// Ranged read of a deflated entry
	data, err = entry.Read(6, 13)
	require.NoError(t, err)
	assert.Equal(t, []byte("deflated"), data)
}
