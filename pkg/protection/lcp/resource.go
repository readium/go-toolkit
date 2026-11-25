package lcp

import (
	"context"
	"errors"
	"io"
	"math"
	"path"
	"strings"
	"sync/atomic"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/byterange"
	"github.com/readium/go-toolkit/pkg/manifest"
	epubparser "github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// opaqueResource is the base of the resources whose bytes differ from the
// proxied resource's.
//
// [fetcher.ProxyResource] forwards the [fetcher.CompressedResource] methods
// to the proxied archive entry, which would let consumers copy its raw
// deflated bytes, e.g. ciphertext where plaintext is expected. They are
// all overridden to report that no compressed form is available.
type opaqueResource struct {
	fetcher.ProxyResource
}

var errNoCompressedForm = errors.New("lcp: transformed resource has no compressed form")

// File implements Resource. The proxied file holds other bytes than the
// resource, so it is not exposed: a consumer serving it directly, e.g. with
// sendfile, would bypass the transformation.
func (r opaqueResource) File() string { return "" }

// CompressedAs implements CompressedResource
func (r opaqueResource) CompressedAs(archive.CompressionMethod) bool { return false }

// CompressedLength implements CompressedResource
func (r opaqueResource) CompressedLength(context.Context) int64 { return -1 }

// CRC32Checksum implements CompressedResource
func (r opaqueResource) CRC32Checksum(context.Context) *uint32 { return nil }

// StreamCompressed implements CompressedResource
func (r opaqueResource) StreamCompressed(context.Context, io.Writer) (int64, *fetcher.ResourceError) {
	return -1, fetcher.Other(errNoCompressedForm)
}

// StreamCompressedGzip implements CompressedResource
func (r opaqueResource) StreamCompressedGzip(context.Context, io.Writer) (int64, *fetcher.ResourceError) {
	return -1, fetcher.Other(errNoCompressedForm)
}

// ReadCompressed implements CompressedResource
func (r opaqueResource) ReadCompressed(context.Context) ([]byte, *fetcher.ResourceError) {
	return nil, fetcher.Other(errNoCompressedForm)
}

// ReadCompressedGzip implements CompressedResource
func (r opaqueResource) ReadCompressedGzip(context.Context) ([]byte, *fetcher.ResourceError) {
	return nil, fetcher.Other(errNoCompressedForm)
}

// resolveRange converts the inclusive [start, end] range of the Resource API
// into a half-open [from, to) range clamped to a resource of the given
// length. start == end == 0 means the whole resource.
func resolveRange(start, end, length int64) (from, to int64, rerr *fetcher.ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if start == 0 && end == 0 {
		return 0, length, nil
	}
	if !valid {
		return 0, 0, fetcher.RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	from = min(start, length)
	to = max(min(exclusiveEnd(end), length), from)
	return from, to, nil
}

// exclusiveEnd converts the inclusive end of a range into an exclusive one,
// without overflowing.
func exclusiveEnd(end int64) int64 {
	if end == math.MaxInt64 {
		return end
	}
	return end + 1
}

// errRangeRead stops a stream once the requested range has been written.
var errRangeRead = errors.New("lcp: range read")

// rangeForwarder writes to w the bytes of the half-open range [from, to) of
// what is written to it, and stops the writing with errRangeRead once it
// has written them.
type rangeForwarder struct {
	w        io.Writer
	from, to int64
	pos      int64
	written  int64
}

func (f *rangeForwarder) Write(p []byte) (int, error) {
	start := f.pos
	f.pos += int64(len(p))
	lo := min(max(f.from-start, 0), int64(len(p)))
	hi := min(max(f.to-start, 0), int64(len(p)))
	if lo < hi {
		n, err := f.w.Write(p[lo:hi])
		f.written += int64(n)
		if err != nil {
			return int(lo) + n, err
		}
	}
	if f.pos >= f.to {
		return len(p), errRangeRead
	}
	return len(p), nil
}

// streamRange writes to w the inclusive range [start, end] of the content
// written by stream, which is stopped as soon as the range is complete. The
// content is never held in memory: a range past its end is truncated, or
// empty. It returns the number of bytes written to w.
func streamRange(w io.Writer, start, end int64, stream func(w io.Writer) (int64, *fetcher.ResourceError)) (int64, *fetcher.ResourceError) {
	if end < start {
		return 0, fetcher.RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	f := &rangeForwarder{w: w, from: max(start, 0), to: exclusiveEnd(end)}
	if _, rerr := stream(f); rerr != nil && !errors.Is(rerr.Cause, errRangeRead) {
		return f.written, rerr
	}
	return f.written, nil
}

// streamThrough streams the whole proxied resource through process, which
// reads it from r and writes its output to w.
func streamThrough(ctx context.Context, res fetcher.Resource, process func(r io.Reader) (int64, error)) (int64, *fetcher.ResourceError) {
	return streamRangeThrough(ctx, res, 0, 0, process)
}

// streamRangeThrough streams the inclusive range [start, end] of the proxied
// resource through process, as [streamThrough] does.
func streamRangeThrough(ctx context.Context, res fetcher.Resource, start, end int64, process func(r io.Reader) (int64, error)) (int64, *fetcher.ResourceError) {
	pr, pw := io.Pipe()
	go func() {
		if _, rerr := res.Stream(ctx, pw, start, end); rerr != nil {
			_ = pw.CloseWithError(rerr)
		} else {
			_ = pw.Close()
		}
	}()
	n, err := process(pr)
	// Unblocks the producer when process stopped early.
	_ = pr.Close()
	if err != nil {
		return n, toResourceError(err)
	}
	return n, nil
}

func toResourceError(err error) *fetcher.ResourceError {
	var rerr *fetcher.ResourceError
	if errors.As(err, &rerr) {
		return rerr
	}
	return fetcher.Other(err)
}

// resourceKey returns the identity of the resource a link points to inside a
// package: its decoded path, cleaned and without leading slash, query or
// fragment. This is how fetchers locate resources, so two links share a key
// exactly when they reach the same container entry, e.g. "chapter.html",
// "./chapter.html#intro" and "chapter.html?v=2". Encryption decisions are
// keyed by it: keying them by any other spelling of the path would let a
// resource selected for encryption be found under a spelling which isn't.
func resourceKey(href manifest.HREF) string {
	u := href.Resolve(nil, nil)
	if u == nil {
		return href.String()
	}
	return containerPath(u.Path())
}

// uriKey returns the resource key of a URI relative to the root of the
// container, e.g. from an EPUB encryption.xml, and whether it is the one of a
// resource of the package, as the EPUB parser keys them (see
// [epubparser.ResourceID]). A URI pointing outside of the package has a
// key of its own, which no resource of the package has.
func uriKey(uri string) (key string, local bool) {
	u, err := url.URLFromString(uri)
	if err != nil {
		return uri, true
	}
	return epubparser.ResourceID(u)
}

// containerPath cleans a decoded path into the form of a container entry
// name, which is also its resource key.
func containerPath(p string) string {
	return strings.TrimPrefix(path.Clean(p), "/")
}

// lengthCache holds the length of a resource once known. A resource may be
// read concurrently: its first readers may all compute the length, and
// errors aren't kept, so that a failed read can be retried.
type lengthCache struct {
	v atomic.Int64 // the length plus one, 0 while unknown
}

func (c *lengthCache) get() (int64, bool) {
	v := c.v.Load()
	return v - 1, v > 0
}

// set keeps the length, and returns it.
func (c *lengthCache) set(length int64) int64 {
	c.v.Store(length + 1)
	return length
}
