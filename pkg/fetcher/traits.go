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
