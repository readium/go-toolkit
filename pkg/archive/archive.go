package archive

import (
	"errors"
	"io"
	"os"
)

type ArchiveFactory interface {
	Open(filepath string, password string) (Archive, error)                                             // Opens an archive from a local [file].
	OpenBytes(data []byte, password string) (Archive, error)                                            // Opens an archive from a [data] slice.
	OpenReader(reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) // Opens an archive from a reader.
}

type DefaultArchiveFactory struct {
	gozipFactory    gozipArchiveFactory
	explodedFactory explodedArchiveFactory
}

// Open implements ArchiveFactory
func (e DefaultArchiveFactory) Open(filepath string, password string) (Archive, error) {
	st, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return e.explodedFactory.Open(filepath, password)
	} else {
		return e.gozipFactory.Open(filepath, password)
	}
}

// OpenBytes implements ArchiveFactory
func (e DefaultArchiveFactory) OpenBytes(data []byte, password string) (Archive, error) {
	if data == nil {
		return nil, errors.New("archive is nil")
	}
	return e.gozipFactory.OpenBytes(data, password)
}

// OpenBytes implements ArchiveFactory
func (e DefaultArchiveFactory) OpenReader(reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	if reader == nil {
		return nil, errors.New("archive is nil")
	}
	return e.gozipFactory.OpenReader(reader, size, password, minimizeReads)
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
	StreamCompressed(w io.Writer) (int64, error)               // Streams the compressed content of this entry to a writer.
}

// Represents an immutable archive.
type Archive interface {
	Entries() []Entry                 // List of all the archived file entries.
	Entry(path string) (Entry, error) // Gets the entry at the given `path`.
	Close()
}
