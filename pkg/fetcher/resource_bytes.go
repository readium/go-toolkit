package fetcher

import (
	"bytes"
	"errors"
	"io"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/xmlquery"
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
func (r *BytesResource) Length() (int64, *ResourceError) {
	bin, err := r.Read(0, 0)
	if err != nil {
		return 0, err
	}
	return int64(len(bin)), nil
}

// Read implements Resource
func (r *BytesResource) Read(start int64, end int64) ([]byte, *ResourceError) {
	if end < start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return nil, err
	}
	if r._bytes == nil {
		r._bytes = r.loader()
		if len(r._bytes) == 0 {
			return nil, Other(errors.New("BytesResource has empty bytes"))
		}
	}
	if start == 0 && end == 0 {
		return r._bytes, nil
	}

	// Bounds check
	length := int64(len(r._bytes))
	if start > (length - 1) {
		start = length - 1
	}
	if end > length {
		end = length
	}

	return r._bytes[start : end+1], nil
}

// Stream implements Resource
func (r *BytesResource) Stream(w io.Writer, start int64, end int64) (int64, *ResourceError) {
	if end < start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return -1, err
	}
	if r._bytes == nil {
		r._bytes = r.loader()
	}
	var buff *bytes.Buffer
	if start == 0 && end == 0 {
		buff = bytes.NewBuffer(r._bytes)
	} else {
		buff = bytes.NewBuffer(r._bytes[start : end+1])
	}
	n, err := io.Copy(w, buff)
	if err != nil {
		return n, Other(err)
	}
	return n, nil
}

// ReadAsString implements Resource
func (r *BytesResource) ReadAsString() (string, *ResourceError) {
	return ReadResourceAsString(r)
}

// ReadAsJSON implements Resource
func (r *BytesResource) ReadAsJSON() (map[string]interface{}, *ResourceError) {
	return ReadResourceAsJSON(r)
}

// ReadAsXML implements Resource
func (r *BytesResource) ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError) {
	return ReadResourceAsXML(r, prefixes)
}

// NewBytesResource creates a new BytesResources from a lazy loader callback.
func NewBytesResource(link manifest.Link, loader func() []byte) *BytesResource {
	return &BytesResource{link: link, loader: loader}
}
