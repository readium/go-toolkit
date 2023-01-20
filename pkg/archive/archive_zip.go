package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path"
	"sync"
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
		data := make([]byte, e.file.UncompressedSize64)
		_, err := io.ReadFull(f, data)
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

func (e gozipArchiveEntry) Stream(w io.Writer, start int64, end int64) (int64, error) {
	if end < start {
		return -1, errors.New("range not satisfiable")
	}
	f, err := e.file.Open()
	if err != nil {
		return -1, err
	}
	defer f.Close()
	if start == 0 && end == 0 {
		return io.Copy(w, f)
	}
	if start > 0 {
		n, err := io.CopyN(io.Discard, f, start)
		if err != nil {
			return n, err
		}
	}
	n, err := io.CopyN(w, f, end-start+1)
	if err != nil && err != io.EOF {
		return n, err
	}
	return n, nil
}

// An archive from a zip file using go's stdlib
type gozipArchive struct {
	zip           *zip.Reader
	closer        func() error
	cachedEntries sync.Map
}

func (a *gozipArchive) Close() {
	a.closer()
}

func (a *gozipArchive) Entries() []Entry {
	entries := make([]Entry, 0, len(a.zip.File))
	for _, f := range a.zip.File {
		if f.FileInfo().IsDir() {
			continue
		}

		aentry, ok := a.cachedEntries.Load(f.Name)
		if !ok {
			aentry = gozipArchiveEntry{
				file: f,
			}
			a.cachedEntries.Store(f.Name, aentry)
		}
		entries = append(entries, aentry.(Entry))
	}
	return entries
}

func (a *gozipArchive) Entry(p string) (Entry, error) {
	if !fs.ValidPath(p) {
		return nil, fs.ErrNotExist
	}
	cpath := path.Clean(p)

	// Check for entry in cache
	aentry, ok := a.cachedEntries.Load(cpath)
	if ok { // Found entry in cache
		return aentry.(Entry), nil
	}

	for _, f := range a.zip.File {
		fp := path.Clean(f.Name)
		if fp == cpath {
			aentry := gozipArchiveEntry{
				file: f,
			}
			a.cachedEntries.Store(fp, aentry) // Put entry in cache
			return aentry, nil
		}
	}
	return nil, fs.ErrNotExist
}

func NewGoZIPArchive(zip *zip.Reader, closer func() error) Archive {
	return &gozipArchive{
		zip:    zip,
		closer: closer,
	}
}

type gozipArchiveFactory struct{}

func (e gozipArchiveFactory) Open(filepath string, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	rc, err := zip.OpenReader(filepath)
	if err != nil {
		return nil, err
	}
	return NewGoZIPArchive(&rc.Reader, rc.Close), nil
}

func (e gozipArchiveFactory) OpenBytes(data []byte, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return NewGoZIPArchive(r, func() error { return nil }), nil
}

type ReaderAtCloser interface {
	io.Closer
	io.ReaderAt
}

func (e gozipArchiveFactory) OpenReader(reader ReaderAtCloser, size int64, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	r, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	return NewGoZIPArchive(r, reader.Close), nil
}
