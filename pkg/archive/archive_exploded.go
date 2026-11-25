package archive

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/readium/go-toolkit/pkg/internal/byterange"
	"github.com/readium/go-toolkit/pkg/internal/rootfs"
)

type explodedArchiveEntry struct {
	dir      string
	filepath string // Filepath, already cleaned!
	fi       fs.FileInfo
	pin      fs.FileInfo // the file the entry is pinned to, if any
}

// FileInfo implements FileEntry
func (e explodedArchiveEntry) FileInfo() (fs.FileInfo, error) {
	return rootfs.Stat(e.dir, e.filepath)
}

// Pin implements PinnableEntry
func (e explodedArchiveEntry) Pin(info fs.FileInfo) Entry {
	e.pin = info
	return e
}

// open opens the file of the entry, checking that it is the file the entry
// is pinned to, if any: the check is made on the opened file, which the read
// then uses.
func (e explodedArchiveEntry) open() (*os.File, error) {
	f, err := rootfs.Open(e.dir, e.filepath)
	if err != nil {
		return nil, err
	}
	if e.pin != nil {
		fi, err := f.Stat()
		if err != nil || !os.SameFile(e.pin, fi) {
			_ = f.Close()
			return nil, ErrFileChanged
		}
	}
	return f, nil
}

func (e explodedArchiveEntry) Path() string {
	return filepath.ToSlash(e.filepath)
}

func (e explodedArchiveEntry) Length() uint64 {
	return uint64(e.fi.Size())
}

func (e explodedArchiveEntry) CompressedLength() uint64 {
	return 0
}

func (e explodedArchiveEntry) CRC32Checksum() *uint32 {
	return nil
}

func (e explodedArchiveEntry) CompressedAs(compressionMethod CompressionMethod) bool {
	// Exploded-archive entries are plain files, i.e. stored uncompressed.
	return compressionMethod == CompressionMethodStore
}

func (e explodedArchiveEntry) Read(start int64, end int64) ([]byte, error) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return nil, errors.New("range not satisfiable")
	}
	f, err := e.open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if start == 0 && end == 0 {
		data := make([]byte, e.fi.Size())
		_, err := io.ReadFull(f, data)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	// Ranges are allocated, so they are clamped to the file first: callers
	// may ask for more than there is.
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if start >= fi.Size() {
		return []byte{}, nil
	}
	end = min(end, fi.Size()-1)
	if start > 0 {
		_, err := f.Seek(start, 0)
		if err != nil {
			return nil, err
		}
	}
	data := make([]byte, end-start+1)
	n, err := io.ReadFull(f, data)
	if n > 0 && err == io.ErrUnexpectedEOF {
		// Not EOF error if some data was read
		err = nil
	}
	return data[:n], err
}

func (e explodedArchiveEntry) Stream(w io.Writer, start int64, end int64) (int64, error) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return -1, errors.New("range not satisfiable")
	}
	f, err := e.open()
	if err != nil {
		return -1, err
	}
	defer f.Close()
	if start == 0 && end == 0 {
		return io.Copy(w, f)
	}
	if start > 0 {
		_, err := f.Seek(start, 0)
		if err != nil {
			return -1, err
		}
	}
	n, err := io.CopyN(w, f, byterange.Length(start, end))
	if err != nil && err != io.EOF {
		return n, err
	}
	return n, nil
}

func (e explodedArchiveEntry) StreamCompressed(w io.Writer) (int64, error) {
	return -1, errors.New("entry is not compressed")
}

func (e explodedArchiveEntry) StreamCompressedGzip(w io.Writer) (int64, error) {
	return -1, errors.New("entry is not compressed")
}

func (e explodedArchiveEntry) ReadCompressed() ([]byte, error) {
	return nil, errors.New("entry is not compressed")
}

func (e explodedArchiveEntry) ReadCompressedGzip() ([]byte, error) {
	return nil, errors.New("entry is not compressed")
}

// An archive exploded on the file system as a directory.
//
// Its entries are the regular files of the directory. Symbolic links are
// followed as long as they lead to a regular file of the directory: they
// could otherwise expose any file of the system.
type explodedArchive struct {
	directory string // Directory, already cleaned!
}

// Close implements Archive
func (a explodedArchive) Close() {
	// Nothing needs to be done
}

// Entries implements Archive. See [explodedArchive.ListEntries], which
// reports a walk failing midway.
func (a explodedArchive) Entries() []Entry {
	entries, _ := a.ListEntries()
	return entries
}

// ListEntries implements [EntriesLister]: an unreadable directory, or an
// entry which disappears while it is listed, is an error, rather than the
// listing silently left incomplete.
func (a explodedArchive) ListEntries() ([]Entry, error) {
	entries := make([]Entry, 0)
	err := filepath.WalkDir(a.directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// The path relative to the directory, which paths under "." don't
		// start with followed by a separator.
		rel, err := filepath.Rel(a.directory, path)
		if err != nil {
			return err
		}
		var inf fs.FileInfo
		switch {
		case d.Type().IsRegular():
			if inf, err = d.Info(); err != nil {
				return err
			}
		case d.Type()&fs.ModeSymlink != 0:
			if inf, err = rootfs.Stat(a.directory, rel); err != nil || !inf.Mode().IsRegular() {
				return nil // leads outside of the directory, or not to a file
			}
		default:
			return nil // named pipe, device...
		}
		entries = append(entries, explodedArchiveEntry{
			dir:      a.directory,
			filepath: rel,
			fi:       inf,
		})
		return nil
	})
	return entries, err
}

// Entry implements Archive
func (a explodedArchive) Entry(path string) (Entry, error) {
	if !fs.ValidPath(path) {
		return nil, fs.ErrNotExist
	}
	cpath := filepath.Clean(filepath.FromSlash(path))
	fi, err := rootfs.Stat(a.directory, cpath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, rootfs.ErrNotRegular
	}
	if !fi.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: path, Err: rootfs.ErrNotRegular}
	}
	return explodedArchiveEntry{
		dir:      a.directory,
		filepath: cpath,
		fi:       fi,
	}, nil
}

func NewExplodedArchive(directory string) Archive {
	return &explodedArchive{
		directory: filepath.Clean(directory),
	}
}

type explodedArchiveFactory struct{}

// Open implements ArchiveFactory
func (e explodedArchiveFactory) Open(filepath string, password string) (Archive, error) {
	st, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New("[filepath] must be a directory to be opened as an exploded archive")
	}
	return NewExplodedArchive(filepath), nil // TODO
}
