package extensions

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	for n, s := range map[int64]string{
		0:             "0 B",
		1023:          "1023 B",
		1024:          "1 KiB",
		1536:          "1.5 KiB",
		32 << 20:      "32 MiB",
		256 << 20:     "256 MiB",
		1 << 30:       "1 GiB",
		math.MaxInt64: "8 EiB",
	} {
		assert.Equal(t, s, FormatSize(n), n)
	}
}
