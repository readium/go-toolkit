package fetcher

import (
	"context"
	"io"

	"github.com/readium/go-toolkit/pkg/archive"
)

type CompressedResource interface {
	CompressedAs(compressionMethod archive.CompressionMethod) bool
	CompressedLength(ctx context.Context) int64
	StreamCompressed(ctx context.Context, w io.Writer) (int64, *ResourceError)
	StreamCompressedGzip(ctx context.Context, w io.Writer) (int64, *ResourceError)
	ReadCompressed(ctx context.Context) ([]byte, *ResourceError)
	ReadCompressedGzip(ctx context.Context) ([]byte, *ResourceError)
	CRC32Checksum(ctx context.Context) *uint32
}

// EfficientStreamer is implemented by resources that can report whether their
// Stream method retrieves just the requested byte range from the underlying
// source with bounded per-request overhead — no reading and discarding of the
// range's prefix, and no degradation into one remote request per tiny chunk.
//
// When HasEfficientStream reports true, Stream is at least as efficient as
// Read even when the resource is backed by a remote source (HTTP, S3, GCS),
// while using bounded memory and delivering the first byte as soon as it is
// available. When it reports false — e.g. for a deflate-compressed entry in an
// archive, where a ranged Stream must decompress from the start of the entry —
// callers serving remote content should prefer Read for ranged access.
type EfficientStreamer interface {
	// HasEfficientStream reports whether Stream retrieves only the requested
	// range from the underlying source. See [EfficientStreamer].
	HasEfficientStream() bool
}
