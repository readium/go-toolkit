package fetcher

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/readium/go-toolkit/pkg/internal/byterange"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// BytesResource is a Resource serving a lazy-loaded bytes buffer.
type BytesResource struct {
	link   manifest.Link
	loader func() []byte
	_bytes []byte
}

// File implements Resource
func (r *BytesResource) File() string {
	return ""
}

// Close implements Resource
func (r *BytesResource) Close() {}

// Link implements Resource
func (r *BytesResource) Link() manifest.Link {
	return r.link
}

// Properties implements Resource
func (r *BytesResource) Properties() manifest.Properties {
	return manifest.Properties{}
}

// Length implements Resource
func (r *BytesResource) Length(ctx context.Context) (int64, *ResourceError) {
	bin, err := r.Read(ctx, 0, 0)
	if err != nil {
		return 0, err
	}
	return int64(len(bin)), nil
}

// Read implements Resource
func (r *BytesResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return nil, err
	}
	if r._bytes == nil {
		r._bytes = r.loader()
		if len(r._bytes) == 0 {
			return nil, Other(errors.New("BytesResource has empty bytes"))
		}
	}
	return sliceRange(r._bytes, start, end)
}

// Stream implements Resource
func (r *BytesResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	if r._bytes == nil {
		r._bytes = r.loader()
	}
	data, rerr := sliceRange(r._bytes, start, end)
	if rerr != nil {
		return -1, rerr
	}
	n, err := io.Copy(w, bytes.NewReader(data))
	if err != nil {
		return n, Other(err)
	}
	return n, nil
}

// sliceRange returns the inclusive range [start, end] of data, or the whole of
// it for 0 to 0, clamped to data. A negative start is the start of data, and
// an end before the start is an error.
func sliceRange(data []byte, start, end int64) ([]byte, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if start == 0 && end == 0 {
		return data, nil
	}
	if !valid {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	length := int64(len(data))
	if start >= length {
		return data[length:], nil
	}
	return data[start : min(end, length-1)+1], nil
}

// NewBytesResource creates a new BytesResources from a lazy loader callback.
func NewBytesResource(link manifest.Link, loader func() []byte) *BytesResource {
	return &BytesResource{link: link, loader: loader}
}
