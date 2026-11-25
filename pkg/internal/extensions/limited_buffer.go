package extensions

import (
	"bytes"
	"errors"
)

// MaxDocumentSize bounds the metadata documents read into memory by the toolkit.
const MaxDocumentSize = 32 << 20

// ErrResourceTooLarge is returned when a buffer's byte limit is exceeded.
var ErrResourceTooLarge = errors.New("resource too large to be read into memory")

// LimitedBuffer accumulates at most Limit bytes from a stream. Exceeded
// remains true even if a producer discards the write error.
type LimitedBuffer struct {
	Limit    int64
	data     bytes.Buffer
	Exceeded bool
}

func (b *LimitedBuffer) Write(p []byte) (int, error) {
	if b.Exceeded || b.Limit < int64(b.data.Len()) || int64(len(p)) > b.Limit-int64(b.data.Len()) {
		b.Exceeded = true
		return 0, ErrResourceTooLarge
	}
	return b.data.Write(p)
}

func (b *LimitedBuffer) Bytes() []byte { return b.data.Bytes() }
