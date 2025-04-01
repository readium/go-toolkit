package fetcher

import (
	"context"
	"io"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/xmlquery"
)

type CompressedResource interface {
	CompressedAs(compressionMethod archive.CompressionMethod) bool
	CompressedLength() int64
	StreamCompressed(w io.Writer) (int64, *ResourceError)
	StreamCompressedGzip(w io.Writer) (int64, *ResourceError)
	ReadCompressed() ([]byte, *ResourceError)
	ReadCompressedGzip() ([]byte, *ResourceError)
}

type StringResource interface {
	Resource

	// Reads the full content as a string.
	// Assumes UTF-8 encoding if no Link charset is given
	ReadAsString() (string, *ResourceError)

	// Reads the full content as a JSON object.
	ReadAsJSON() (map[string]interface{}, *ResourceError)

	// Reads the full content as a generic XML document.
	ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError)
}

// Adds context to resource retrieval.
type RemoteResource interface {
	Resource

	// Returns data length from metadata if available, or calculated from reading the bytes otherwise.
	// This value must be treated as a hint, as it might not reflect the actual bytes length. To get the real length, you need to read the whole resource.
	LengthWithContext(ctx context.Context) (int64, *ResourceError)

	// Reads the bytes at the given range.
	// When start and end are null, the whole content is returned. Out-of-range indexes are clamped to the available length automatically.
	ReadWithContext(ctx context.Context, start int64, end int64) ([]byte, *ResourceError)

	// Stream the bytes at the given range to a writer.
	// When start and end are null, the whole content is returned. Out-of-range indexes are clamped to the available length automatically.
	StreamWithContext(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError)
}
