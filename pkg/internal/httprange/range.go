// Package httprange validates byte ranges returned by HTTP origins.
package httprange

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Length checks the inclusive range, allowing the server to clamp
// its end to EOF. A negative end means through EOF. A negative knownSize
// means the caller has no independently established size.
func Length(resp *http.Response, start, end, knownSize int64) (int64, error) {
	if len(resp.Header.Values("Content-Range")) != 1 {
		return 0, errors.New("expected one Content-Range header")
	}
	span, ok := strings.CutPrefix(resp.Header.Get("Content-Range"), "bytes ")
	bounds, total, slash := strings.Cut(span, "/")
	first, last, dash := strings.Cut(bounds, "-")
	parse := func(s string) (int64, error) {
		n, err := strconv.ParseUint(s, 10, 63)
		return int64(n), err
	}
	from, errFrom := parse(first)
	to, errTo := parse(last)
	if !ok || !slash || !dash || errFrom != nil || errTo != nil || from != start || to < from || to-from == math.MaxInt64 {
		return 0, errors.New("invalid HTTP Content-Range")
	}
	if total != "*" {
		size, err := parse(total)
		if err != nil || to >= size || (knownSize >= 0 && size != knownSize) {
			return 0, errors.New("invalid HTTP Content-Range size")
		}
		if end < 0 || end >= size {
			end = size - 1
		}
	} else if end < 0 {
		return 0, errors.New("unknown length of open-ended HTTP range")
	}
	n := to - from + 1
	if to != end || (resp.ContentLength >= 0 && resp.ContentLength != n) {
		return 0, errors.New("HTTP response does not match requested range")
	}
	return n, nil
}
