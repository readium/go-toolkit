package archive

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type explodedArchiveEntry struct {
	dir      string
	filepath string // Filepath, already cleaned!
	fi       fs.FileInfo
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

func (e explodedArchiveEntry) CompressedAs(compressionMethod CompressionMethod) bool {
	return false
}

func (e explodedArchiveEntry) Read(start int64, end int64) ([]byte, error) {
	if end < start {
		return nil, errors.New("range not satisfiable")
	}
	f, err := os.Open(filepath.Join(e.dir, e.filepath))
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
	if start > 0 {
		_, err := f.Seek(start, 0)
		if err != nil {
			return nil, err
		}
	}
	data := make([]byte, end-start+1)
	n, err := f.Read(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (e explodedArchiveEntry) Stream(w io.Writer, start int64, end int64) (int64, error) {
	if end < start {
		return -1, errors.New("range not satisfiable")
	}
	f, err := os.Open(filepath.Join(e.dir, e.filepath))
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
	n, err := io.CopyN(w, f, end-start+1)
	if err != nil && err != io.EOF {
		return n, err
	}
	return n, nil
}

func (e explodedArchiveEntry) StreamCompressed(w io.Writer) (int64, error) {
	return -1, errors.New("entry is not compressed")
}

// An archive exploded on the file system as a directory.
type explodedArchive struct {
	directory string // Directory, already cleaned!
}

func (a explodedArchive) Close() {
	// Nothing needs to be done
}

func (a explodedArchive) Entries() []Entry {
	entries := make([]Entry, 0)
	filepath.WalkDir(a.directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		inf, err := d.Info()
		if err != nil {
			return err
		}
		entries = append(entries, explodedArchiveEntry{
			dir:      a.directory,
			filepath: filepath.Clean(path[len(a.directory)+1:]), // Remove the directory from the path
			fi:       inf,
		})
		return nil
	})
	return entries
}

func (a explodedArchive) Entry(path string) (Entry, error) {
	if !fs.ValidPath(path) {
		return nil, fs.ErrNotExist
	}
	cpath := filepath.Clean(path)
	entirePath := filepath.Join(a.directory, cpath)
	fi, err := os.Stat(entirePath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() { // directory.isParentOf(file) ?
		return nil, errors.New("supposed file is a directory")
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
