package archive

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"io"
	"io/fs"
	"math"
	"path"
	"sync"

	"github.com/pkg/errors"
)

type gozipArchiveEntry struct {
	file          *zip.File
	minimizeReads bool
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

func (e gozipArchiveEntry) CompressedAs(compressionMethod CompressionMethod) bool {
	if compressionMethod != CompressionMethodDeflate {
		return false
	}
	return e.file.Method == zip.Deflate
}

// This is a special mode to minimize the number of reads from the underlying reader.
// It's especially useful when trying to stream the ZIP from a remote file, e.g.
// cloud storage. It's only enabled when trying to read the entire file and compression
// is enabled. Care needs to be taken to cover every edge case.
func (e gozipArchiveEntry) couldMinimizeReads() bool {
	return e.minimizeReads && e.CompressedLength() > 0
}

func (e gozipArchiveEntry) Read(start int64, end int64) ([]byte, error) {
	if end < start {
		return nil, errors.New("range not satisfiable")
	}

	minimizeReads := e.couldMinimizeReads()

	var f io.Reader
	var err error
	if minimizeReads {
		f, err = e.file.OpenRaw()
		if err != nil {
			return nil, err
		}
	} else {
		rc, err := e.file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		f = rc
	}

	if minimizeReads {
		compressedData := make([]byte, e.file.CompressedSize64)
		_, err := io.ReadFull(f, compressedData)
		if err != nil {
			return nil, err
		}
		frdr := flate.NewReader(bytes.NewReader(compressedData))
		defer frdr.Close()
		f = frdr
	}

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
	data := make([]byte, min(end-start+1, int64(e.file.UncompressedSize64)))
	_, err = io.ReadFull(f, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (e gozipArchiveEntry) Stream(w io.Writer, start int64, end int64) (int64, error) {
	if end < start {
		return -1, errors.New("range not satisfiable")
	}

	minimizeReads := e.couldMinimizeReads() && start == 0 && end == 0

	var f io.Reader
	var err error
	if minimizeReads {
		f, err = e.file.OpenRaw()
		if err != nil {
			return -1, err
		}
	} else {
		rc, err := e.file.Open()
		if err != nil {
			return -1, err
		}
		defer rc.Close()
		f = rc
	}

	if minimizeReads {
		compressedData := make([]byte, e.file.CompressedSize64)
		_, err := io.ReadFull(f, compressedData)
		if err != nil {
			return -1, err
		}
		frdr := flate.NewReader(bytes.NewReader(compressedData))
		defer frdr.Close()
		f = frdr
	}

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

func (e gozipArchiveEntry) StreamCompressed(w io.Writer) (int64, error) {
	if e.file.Method != zip.Deflate {
		return -1, errors.New("not a compressed resource")
	}
	f, err := e.file.OpenRaw()
	if err != nil {
		return -1, err
	}

	return io.Copy(w, f)
}

func (e gozipArchiveEntry) StreamCompressedGzip(w io.Writer) (int64, error) {
	if e.file.Method != zip.Deflate {
		return -1, errors.New("not a compressed resource")
	}
	if e.file.UncompressedSize64 > math.MaxUint32 {
		return -1, errors.New("uncompressed size > 2^32 too large for GZIP")
	}
	f, err := e.file.OpenRaw()
	if err != nil {
		return -1, err
	}

	// Header
	buf := [10]byte{0: gzipID1, 1: gzipID2, 2: gzipDeflate, 9: 255}
	// No extra, no name, no comment, no mod time, no compress level hint, unknown OS

	n, err := w.Write(buf[:10])
	if err != nil {
		return -1, errors.Wrap(err, "failed to write GZIP header")
	}

	nn, err := io.Copy(w, f)
	if err != nil {
		return int64(n), errors.Wrap(err, "failed copying deflated bytes")
	}

	// Trailer
	binary.LittleEndian.PutUint32(buf[:4], e.file.CRC32)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(e.file.UncompressedSize64))
	nnn, err := w.Write(buf[:8])
	if err != nil {
		return int64(n) + nn, errors.Wrap(err, "failed writing GZIP trailer")
	}
	return int64(n) + nn + int64(nnn), nil
}

func (e gozipArchiveEntry) ReadCompressed() ([]byte, error) {
	if e.file.Method != zip.Deflate {
		return nil, errors.New("not a compressed resource")
	}
	f, err := e.file.OpenRaw()
	if err != nil {
		return nil, err
	}

	compressedData := make([]byte, e.file.CompressedSize64)
	_, err = io.ReadFull(f, compressedData)
	if err != nil {
		return nil, err
	}

	return compressedData, nil
}

func (e gozipArchiveEntry) ReadCompressedGzip() ([]byte, error) {
	if e.file.Method != zip.Deflate {
		return nil, errors.New("not a compressed resource")
	}
	if e.file.UncompressedSize64 > math.MaxUint32 {
		return nil, errors.New("uncompressed size > 2^32 too large for GZIP")
	}
	f, err := e.file.OpenRaw()
	if err != nil {
		return nil, err
	}

	compressedData := make([]byte, e.file.CompressedSize64+GzipWrapperLength) // Size of file + header + trailer

	// Deflated data
	_, err = io.ReadAtLeast(f, compressedData[10:], int(e.file.CompressedSize64))
	if err != nil {
		return nil, err
	}

	// Header
	compressedData[0] = gzipID1
	compressedData[1] = gzipID2
	compressedData[2] = gzipDeflate
	compressedData[9] = 255
	// No extra, no name, no comment, no mod time, no compress level hint, unknown OS

	// Trailer
	binary.LittleEndian.PutUint32(compressedData[10+e.file.CompressedSize64:], e.file.CRC32)
	binary.LittleEndian.PutUint32(compressedData[10+e.file.CompressedSize64+4:], uint32(e.file.UncompressedSize64))

	return compressedData, nil
}

// An archive from a zip file using go's stdlib
type gozipArchive struct {
	zip           *zip.Reader
	closer        func() error
	cachedEntries sync.Map
	minimizeReads bool
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
				file:          f,
				minimizeReads: a.minimizeReads,
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
				file:          f,
				minimizeReads: a.minimizeReads,
			}
			a.cachedEntries.Store(fp, aentry) // Put entry in cache
			return aentry, nil
		}
	}
	return nil, fs.ErrNotExist
}

func NewGoZIPArchive(zip *zip.Reader, closer func() error, minimizeReads bool) Archive {
	return &gozipArchive{
		zip:           zip,
		closer:        closer,
		minimizeReads: minimizeReads,
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
	return NewGoZIPArchive(&rc.Reader, rc.Close, false), nil
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
	return NewGoZIPArchive(r, func() error { return nil }, false), nil
}

type ReaderAtCloser interface {
	io.Closer
	io.ReaderAt
}

func (e gozipArchiveFactory) OpenReader(reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	r, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	return NewGoZIPArchive(r, reader.Close, minimizeReads), nil
}
