package fetcher

import (
	"errors"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
)

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
	}
	if start == 0 && end == 0 {
		return r._bytes, nil
	}
	return r._bytes[start:end], nil
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
func (r *BytesResource) ReadAsXML() (*xmlquery.Node, *ResourceError) {
	return ReadResourceAsXML(r)
}

func NewBytesResource(link manifest.Link, loader func() []byte) *BytesResource {
	return &BytesResource{link: link, loader: loader}
}
