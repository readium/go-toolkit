package archive

import "archive/zip"

type CompressionMethod uint16

const (
	CompressionMethodStore   CompressionMethod = CompressionMethod(zip.Store)
	CompressionMethodDeflate CompressionMethod = CompressionMethod(zip.Deflate)
)
