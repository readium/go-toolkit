package fetcher

import "io"

type CompressedResource interface {
	CompressedAs(compressionMethod uint16) bool
	StreamCompressed(w io.Writer) (int64, *ResourceError)
}
