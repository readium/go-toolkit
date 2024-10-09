package archive

import "math"

const (
	gzipID1     = 0x1f
	gzipID2     = 0x8b
	gzipDeflate = 8
)

const GzipWrapperLength = 18
const GzipMaxLength = math.MaxUint32
