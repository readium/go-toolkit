package fetcher

import (
	"errors"
)

// For opening a fetcher.Resource as a io.ReadSeeker
type ResourceReadSeeker struct {
	r      Resource
	offset int64
	length int64
}

func NewResourceReadSeeker(r Resource) *ResourceReadSeeker {
	return &ResourceReadSeeker{
		r: r,
	}
}

// Seek implements io.ReadSeeker
func (rs *ResourceReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		if offset < 0 {
			return 0, errors.New("new offset smaller than zero")
		}
		rs.offset = offset
		return rs.offset, nil
	case 1:
		if (rs.offset + offset) < 0 {
			return 0, errors.New("new offset smaller than zero")
		}
		rs.offset += offset
		return rs.offset, nil
	case 2:
		if rs.length == 0 {
			length, errx := rs.r.Length()
			if errx != nil {
				return 0, errx
			}
			rs.length = length
		}
		if (rs.length + offset) < 0 {
			return 0, errors.New("new offset smaller than zero")
		}
		rs.offset = rs.length + offset
		return rs.offset, nil
	default:
		panic("invalid whence value")
	}
}

// Seek implements io.ReadSeeker
func (rs *ResourceReadSeeker) Read(p []byte) (n int, err error) {
	bin, errx := rs.r.Read(rs.offset, rs.offset+int64(len(p)))
	if errx != nil {
		err = errx
		return
	}
	n = copy(p, bin)
	rs.offset += int64(n)
	return
}
