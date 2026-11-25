// Package ziputil holds ZIP writing helpers shared by the packagers.
package ziputil

import (
	"archive/zip"
	"compress/flate"
	"context"
	"hash/crc32"
	"io"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
)

// zip64Threshold is the size at which ZIP64 extensions become necessary; such
// entries can't go through the raw-copy path.
const zip64Threshold = 0xFFFFFFFF

// NewWriter returns a ZIP writer deflating entries at the best compression
// level, instead of archive/zip's default level 5: packages are written once
// and read many times, so the extra compression time is worth it.
//
// Raw-copied entries (see [copyRawDeflated]) keep their original compression.
func NewWriter(w io.Writer) *zip.Writer {
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	return zw
}

// WriteMimetype writes an OCF-style mimetype entry: stored uncompressed, and
// without any extra field in its local header (which rules out setting a
// modification time). It must be the first entry of the archive.
func WriteMimetype(zw *zip.Writer, mediaType string) error {
	content := []byte(mediaType)
	fh := &zip.FileHeader{
		Name:               "mimetype",
		Method:             zip.Store,
		CRC32:              crc32.ChecksumIEEE(content),
		CompressedSize64:   uint64(len(content)),
		UncompressedSize64: uint64(len(content)),
	}
	fh.ReaderVersion = 20
	w, err := zw.CreateRaw(fh)
	if err != nil {
		return err
	}
	_, err = w.Write(content)
	return err
}

// CopyEntry copies a resource into the archive under the given name.
//
// When the resource is a deflated archive entry, its compressed bytes are
// copied without being inflated and deflated again. Otherwise the resource is
// streamed into a new entry using the given compression method.
func CopyEntry(ctx context.Context, zw *zip.Writer, name string, res fetcher.Resource, method uint16) error {
	copied, err := copyRawDeflated(ctx, zw, name, res)
	if err != nil || copied {
		return err
	}
	return StreamEntry(ctx, zw, name, res, method)
}

// copyRawDeflated copies the compressed bytes of a deflated archive entry into
// a new entry, without recompressing them. It reports false, without writing
// anything, when the resource is not a deflated entry or can't be copied raw
// (unknown CRC or length, or ZIP64 sizes).
func copyRawDeflated(ctx context.Context, zw *zip.Writer, name string, res fetcher.Resource) (bool, error) {
	cres, ok := res.(fetcher.CompressedResource)
	if !ok || !cres.CompressedAs(archive.CompressionMethodDeflate) {
		return false, nil
	}
	crc := cres.CRC32Checksum(ctx)
	if crc == nil {
		return false, nil
	}
	compressedLength := cres.CompressedLength(ctx)
	length, lerr := res.Length(ctx)
	if compressedLength < 0 || lerr != nil || length < 0 ||
		uint64(compressedLength) >= zip64Threshold || uint64(length) >= zip64Threshold {
		return false, nil
	}

	fh := &zip.FileHeader{
		Name:               name,
		Method:             zip.Deflate,
		CRC32:              *crc,
		CompressedSize64:   uint64(compressedLength),
		UncompressedSize64: uint64(length),
	}
	fh.ReaderVersion = 20
	// CreateRaw skips CreateHeader's UTF-8 detection.
	if !IsASCII(name) {
		fh.Flags |= 0x800
	}
	ew, err := zw.CreateRaw(fh)
	if err != nil {
		return false, err
	}
	if _, rerr := cres.StreamCompressed(ctx, ew); rerr != nil {
		return true, rerr
	}
	return true, nil
}

// StreamEntry streams the whole resource into a new entry using the given
// compression method.
func StreamEntry(ctx context.Context, zw *zip.Writer, name string, res fetcher.Resource, method uint16) error {
	ew, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
	if err != nil {
		return err
	}
	if _, rerr := res.Stream(ctx, ew, 0, 0); rerr != nil {
		return rerr
	}
	return nil
}

// WriteBytes adds an entry holding data, using the given compression method.
func WriteBytes(zw *zip.Writer, name string, data []byte, method uint16) error {
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// CreateEntry creates a new entry using the given compression method and
// returns a writer for its content.
func CreateEntry(zw *zip.Writer, name string, method uint16) (io.Writer, error) {
	return zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
}

// Method returns the ZIP compression method to use for content of the given
// kind: stored when already compressed, deflated otherwise.
func Method(precompressed bool) uint16 {
	if precompressed {
		return zip.Store
	}
	return zip.Deflate
}

// IsASCII reports whether s only holds ASCII characters, i.e. whether a ZIP
// entry with that name can do without the UTF-8 name flag (0x800).
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
