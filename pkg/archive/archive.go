package archive

import "os"

type ArchiveFactory interface {
	Open(filepath string, password string) (Archive, error) // Opens an archive from a local [file].
}

type DefaultArchiveFactory struct {
	gozipFactory    gozipArchiveFactory
	explodedFactory explodedArchiveFactory
}

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

func NewArchiveFactory() DefaultArchiveFactory {
	return DefaultArchiveFactory{}
}

// Holds an archive entry's metadata.
type Entry interface {
	Path() string                                // Absolute path to the entry in the archive.
	Length() uint64                              // Uncompressed data length.
	CompressedLength() uint64                    // Compressed data length.
	Read(start int64, end int64) ([]byte, error) // Reads the whole content of this entry, or a portion when [start] or [end] are specified.
	// Close()
}

// Represents an immutable archive.
type Archive interface {
	Entries() []Entry                 // List of all the archived file entries.
	Entry(path string) (Entry, error) // Gets the entry at the given `path`.
	Close()
}
