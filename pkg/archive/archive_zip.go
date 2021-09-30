package archive

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"path"
)

type gozipArchiveEntry struct {
	file *zip.File
}

func (e gozipArchiveEntry) Path() string {
	return path.Clean(e.file.Name)
}

func (e gozipArchiveEntry) Length() uint64 {
	return e.file.UncompressedSize64
}

func (e gozipArchiveEntry) CompressedLength() uint64 {
	if e.file.Method == zip.Store {
		return 0
	}
	return e.file.CompressedSize64
}

func (e gozipArchiveEntry) Read(start int64, end int64) ([]byte, error) {
	if end < start {
		return nil, errors.New("range not satisfiable")
	}
	f, err := e.file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if start == 0 && end == 0 {
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	if start > 0 {
		_, err := io.CopyN(io.Discard, f, start)
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

// An archive from a zip file using go's stdlib
type gozipArchive struct {
	zip           *zip.ReadCloser
	cachedEntries map[string]Entry
}

func (a *gozipArchive) Close() {
	a.zip.Close()
}

func (a *gozipArchive) Entries() []Entry {
	if a.cachedEntries == nil { // Initialize cache
		a.cachedEntries = make(map[string]Entry)
	}
	entries := make([]Entry, 0, len(a.zip.File))
	for _, f := range a.zip.File {
		if f.FileInfo().IsDir() {
			continue
		}

		aentry, ok := a.cachedEntries[f.Name]
		if !ok {
			aentry = gozipArchiveEntry{
				file: f,
			}
			a.cachedEntries[f.Name] = aentry
		}
		entries = append(entries, aentry)
	}
	return entries
}

func (a *gozipArchive) Entry(p string) (Entry, error) {
	if !fs.ValidPath(p) {
		return nil, fs.ErrNotExist
	}
	cpath := path.Clean(p)
	if a.cachedEntries == nil {
		// Initialize cache
		a.cachedEntries = make(map[string]Entry)
	} else {
		// Check for entry in cache
		aentry, ok := a.cachedEntries[cpath]
		if ok { // Found entry in cache
			return aentry, nil
		}
	}
	for _, f := range a.zip.File {
		fp := path.Clean(f.Name)
		if fp == cpath {
			aentry := gozipArchiveEntry{
				file: f,
			}
			a.cachedEntries[fp] = aentry // Put entry in cache
			return aentry, nil
		}
	}
	return nil, fs.ErrNotExist
}

func NewGoZIPArchive(zip *zip.ReadCloser) Archive {
	return &gozipArchive{
		zip: zip,
	}
}

type gozipArchiveFactory struct{}

func (e gozipArchiveFactory) Open(filepath string, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords. Maybe look into something like https://github.com/mholt/archiver
	rc, err := zip.OpenReader(filepath)
	if err != nil {
		return nil, err
	}
	return NewGoZIPArchive(rc), nil
}
