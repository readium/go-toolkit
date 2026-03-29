package archive

import "math"

const (
	gzipID1     = 0x1f
	gzipID2     = 0x8b
	gzipDeflate = 8
)

const GzipHeaderLength = 10
const GzipTrailerLength = 8
const GzipWrapperLength = GzipHeaderLength + GzipTrailerLength
const GzipMaxLength = math.MaxUint32

const ZRandCutoff = 1024 * 1024 // 1MB
