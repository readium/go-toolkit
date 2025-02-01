package fetcher

import (
	"io"

	"github.com/readium/go-toolkit/pkg/archive"
)

type CompressedResource interface {
	CompressedAs(compressionMethod archive.CompressionMethod) bool
	CompressedLength() int64
	StreamCompressed(w io.Writer) (int64, *ResourceError)
	StreamCompressedGzip(w io.Writer) (int64, *ResourceError)
	ReadCompressed() ([]byte, *ResourceError)
	ReadCompressedGzip() ([]byte, *ResourceError)
}
