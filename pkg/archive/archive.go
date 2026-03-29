package archive

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/readium/go-toolkit/pkg/util/url"
)

type ArchiveFactory interface {
	Open(ctx context.Context, location url.URL, password string) (Archive, error)                                            // Opens an archive from a location.
	OpenBytes(ctx context.Context, data []byte, password string) (Archive, error)                                            // Opens an archive from a [data] slice.
	OpenReader(ctx context.Context, reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) // Opens an archive from a reader.
}

type SchemeSpecificArchiveFactory interface {
	CanOpen(url.Scheme) bool // Whether this factory can open the given scheme.
}

type DefaultArchiveFactory struct {
	gozipFactory    gozipArchiveFactory
	explodedFactory explodedArchiveFactory
}

// Open implements ArchiveFactory
func (e DefaultArchiveFactory) Open(ctx context.Context, location url.URL, password string) (Archive, error) {
	u := url.BaseFile.Resolve(location).(url.AbsoluteURL)
	if u.Scheme() != url.SchemeFile {
		return nil, errors.New("unsupported scheme " + u.Scheme().String())
	}

	st, err := os.Stat(u.Path())
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return e.explodedFactory.Open(u.Path(), password)
	} else {
		return e.gozipFactory.Open(u.Path(), password)
	}
}

// OpenBytes implements ArchiveFactory
func (e DefaultArchiveFactory) OpenBytes(ctx context.Context, data []byte, password string) (Archive, error) {
	if data == nil {
		return nil, errors.New("archive is nil")
	}
	return e.gozipFactory.OpenBytes(data, password)
}

// OpenBytes implements ArchiveFactory
func (e DefaultArchiveFactory) OpenReader(ctx context.Context, reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	if reader == nil {
		return nil, errors.New("archive is nil")
	}
	return e.gozipFactory.OpenReader(reader, size, password, minimizeReads)
}

// CanOpenScheme implements SchemeSpecificArchiveFactory
func (e DefaultArchiveFactory) CanOpenScheme(scheme url.Scheme) bool {
	return scheme == url.SchemeFile
}

func NewArchiveFactory() DefaultArchiveFactory {
	return DefaultArchiveFactory{}
}

// Holds an archive entry's metadata.
type Entry interface {
	Path() string                                              // Absolute path to the entry in the archive.
	Length() uint64                                            // Uncompressed data length.
	CompressedLength() uint64                                  // Compressed data length.
	CompressedAs(compressionMethod CompressionMethod) bool     // Whether the entry is compressed using the given method.
	Read(start int64, end int64) ([]byte, error)               // Reads the whole content of this entry, or a portion when [start] or [end] are specified.
	Stream(w io.Writer, start int64, end int64) (int64, error) // Streams the whole content of this entry to a writer, or a portion when [start] or [end] are specified.

	StreamCompressed(w io.Writer) (int64, error)     // Streams the compressed content of this entry to a writer.
	StreamCompressedGzip(w io.Writer) (int64, error) // Streams the compressed content of this entry to a writer in a GZIP container.
	ReadCompressed() ([]byte, error)                 // Reads the compressed content of this entry.
	ReadCompressedGzip() ([]byte, error)             // Reads the compressed content of this entry inside a GZIP container.

	CRC32Checksum() *uint32 // Returns the CRC32 checksum of the uncompressed data.
}

// Represents an immutable archive.
type Archive interface {
	Entries() []Entry                 // List of all the archived file entries.
	Entry(path string) (Entry, error) // Gets the entry at the given `path`.
	Close()
}
