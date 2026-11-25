// Package byterange implements the inclusive range convention of publication resources.
package byterange

import "math"

// Normalize clamps a negative start to zero and reports whether the end is
// valid. A normalized range of (0, 0) denotes the whole resource.
func Normalize(start, end int64) (int64, bool) {
	start = max(start, 0)
	return start, end >= start
}

// Length returns the number of bytes in a valid, normalized range. The
// largest representable resource has MaxInt64 bytes.
func Length(start, end int64) int64 {
	span := end - start
	if span == math.MaxInt64 {
		return math.MaxInt64
	}
	return span + 1
}
