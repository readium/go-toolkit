package archive

import (
	"context"
	"errors"
	"io"
	"io/fs"
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

	path := u.ToFilepath()
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return e.explodedFactory.Open(path, password)
	} else {
		return e.gozipFactory.Open(path, password)
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

// FileEntry is implemented by the entries of archives exploded on the file
// system, which are files.
type FileEntry interface {
	// FileInfo describes the file of the entry. It identifies the file,
	// e.g. to tell with os.SameFile whether two entries are the same file
	// through symbolic or hard links.
	FileInfo() (fs.FileInfo, error)
}

// ErrFileChanged is returned by the reads of an entry pinned to a file (see
// [PinnableEntry]) when its path names another file since.
var ErrFileChanged = errors.New("the file changed since it was checked")

// PinnableEntry is implemented by entries read from a file, which can be
// pinned to it.
type PinnableEntry interface {
	FileEntry
	// Pin returns the entry, reading only from the file described by info:
	// each read checks the file it opens, and fails with [ErrFileChanged]
	// when the path of the entry names another file, e.g. because it was
	// renamed or replaced by a link since info was taken.
	Pin(info fs.FileInfo) Entry
}

// StableEntry is implemented by entries which can report whether they are
// read from a single version of their archive, which can't be replaced while
// it is open: e.g. a local archive is read from the file opened, while a
// remote one is read with requests which may each get another version. The
// file of a local archive is assumed not to be rewritten in place while it
// is open: replacing it by renaming another file over it doesn't change the
// version read.
type StableEntry interface {
	Stable() bool
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

// EntriesLister is implemented by archives whose listing may fail, such as
// exploded ones, whose directories may be unreadable, or change while they
// are walked. ListEntries returns the error, where Entries returns the
// entries listed until then.
type EntriesLister interface {
	ListEntries() ([]Entry, error)
}

// Archive represents an immutable archive.
type Archive interface {
	Entries() []Entry                 // List of all the archived file entries.
	Entry(path string) (Entry, error) // Gets the entry at the given `path`.
	Close()
}
