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
	"github.com/readium/zran"
)

type gozipArchiveEntry struct {
	file          *zip.File
	minimizeReads bool

	gi zran.Index
	gm sync.Mutex
}

func (e *gozipArchiveEntry) Path() string {
	return path.Clean(e.file.Name)
}

func (e *gozipArchiveEntry) Length() uint64 {
	return e.file.UncompressedSize64
}

func (e *gozipArchiveEntry) CompressedLength() uint64 {
	if e.file.Method == zip.Store {
		return 0
	}
	return e.file.CompressedSize64
}

func (e *gozipArchiveEntry) CRC32Checksum() *uint32 {
	c := e.file.CRC32
	return &c
}

func (e *gozipArchiveEntry) CompressedAs(compressionMethod CompressionMethod) bool {
	if compressionMethod != CompressionMethodDeflate {
		return false
	}
	return e.file.Method == zip.Deflate
}

// This is a special mode to minimize the number of reads from the underlying reader.
// It's especially useful when trying to stream the ZIP from a remote file, e.g.
// cloud storage. It's only enabled when trying to read the entire file and compression
// is enabled. Care needs to be taken to cover every edge case.
func (e *gozipArchiveEntry) couldMinimizeReads() bool {
	return e.minimizeReads && e.CompressedLength() > 0
}

func (e *gozipArchiveEntry) Read(start int64, end int64) ([]byte, error) {
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
		// If uncompressed size is smaller than 1MB, it's not worth
		// using deflate random access, because the state itself takes memory.
		// We also skip using zrand logic if the entire file is being requested,
		// becaus that means the client probably won't need a partial range
		if e.file.UncompressedSize64 < ZRandCutoff || (start == 0 && (end == 0 || end == int64(e.file.UncompressedSize64-1))) {
			compressedData := make([]byte, e.file.CompressedSize64)
			_, err := io.ReadFull(f, compressedData)
			if err != nil {
				return nil, err
			}
			frdr := flate.NewReader(bytes.NewReader(compressedData))
			defer frdr.Close()
			f = frdr
		} else {
			e.gm.Lock()
			var lastCompressedOffset int64
			for _, v := range e.gi {
				if v.CompressedOffset > lastCompressedOffset && v.UncompressedOffset <= start {
					lastCompressedOffset = v.CompressedOffset
				}
			}
			e.gm.Unlock()

			compressedData := make([]byte, e.file.CompressedSize64)
			f.(io.Seeker).Seek(lastCompressedOffset, io.SeekStart)
			_, err := io.ReadFull(f, compressedData[lastCompressedOffset:])
			if err != nil {
				return nil, err
			}

			// This special reader lets us restore the decompressor state at known offsets
			// which is useful when a client has already requested previous parts of the file,
			// such as when a web browser requests subsequent byte ranges for media playback.
			fzr, err := zran.NewDReader(bytes.NewReader(compressedData)) // Default interval = 1MB, same as current ZRandCutoff
			if err != nil {
				return nil, err
			}
			// Note: if an implementor uses the same publication instance for all clients,
			// this code will lock all clients. This could be problematic and should be
			// mitigated in a future version. For us to get this far is pretty rare though,
			// and mainly applies to multimedia that is natively streamed by web browsers and
			// was also inconveniently compressed by the original author of the ZIP.
			e.gm.Lock()
			defer e.gm.Unlock()
			defer func() {
				e.gi = fzr.Index
			}()
			defer fzr.Close()
			if len(e.gi) > 0 {
				fzr.Index = e.gi
			}

			f = fzr
		}
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
		if skr, ok := f.(io.Seeker); ok {
			_, err = skr.Seek(start, io.SeekStart)
		} else {
			_, err = io.CopyN(io.Discard, f, start)
		}
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

func (e *gozipArchiveEntry) Stream(w io.Writer, start int64, end int64) (int64, error) {
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
	if n > 0 && err == io.EOF {
		// Not EOF error if some data was read
		err = nil
	}
	return n, err
}

func (e *gozipArchiveEntry) StreamCompressed(w io.Writer) (int64, error) {
	if e.file.Method != zip.Deflate {
		return -1, errors.New("not a compressed resource")
	}
	f, err := e.file.OpenRaw()
	if err != nil {
		return -1, err
	}

	return io.Copy(w, f)
}

func (e *gozipArchiveEntry) StreamCompressedGzip(w io.Writer) (int64, error) {
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

func (e *gozipArchiveEntry) ReadCompressed() ([]byte, error) {
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

func (e *gozipArchiveEntry) ReadCompressedGzip() ([]byte, error) {
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
	minimizeReads bool

	// nameIndex maps each entry's cleaned path to its *zip.File, giving O(1)
	// lookups instead of a linear scan. It's built once, lazily, on the first
	// Entry call that misses the wrapper cache.
	indexOnce sync.Once
	nameIndex map[string]*zip.File

	// cachedEntries holds lazily-created *gozipArchiveEntry wrappers, keyed by
	// cleaned path. Wrappers are shared so per-entry read state (e.g. the
	// deflate random-access index) persists across reads of the same entry.
	cachedEntries sync.Map
}

// buildIndex populates nameIndex from the archive's central directory. First
// occurrence of a cleaned path wins, matching the previous linear-scan lookup.
func (a *gozipArchive) buildIndex() {
	nameIndex := make(map[string]*zip.File, len(a.zip.File))
	for _, f := range a.zip.File {
		cp := path.Clean(f.Name)
		if _, ok := nameIndex[cp]; !ok {
			nameIndex[cp] = f
		}
	}
	a.nameIndex = nameIndex
}

// wrap returns the shared entry wrapper for f, creating it on first access.
// LoadOrStore guarantees a single wrapper per entry even under concurrent Get.
func (a *gozipArchive) wrap(cleanPath string, f *zip.File) *gozipArchiveEntry {
	if e, ok := a.cachedEntries.Load(cleanPath); ok {
		return e.(*gozipArchiveEntry)
	}
	entry := &gozipArchiveEntry{
		file:          f,
		minimizeReads: a.minimizeReads,
	}
	actual, _ := a.cachedEntries.LoadOrStore(cleanPath, entry)
	return actual.(*gozipArchiveEntry)
}

// Close implements Archive
func (a *gozipArchive) Close() {
	a.closer()
}

// Entries implements Archive
func (a *gozipArchive) Entries() []Entry {
	entries := make([]Entry, 0, len(a.zip.File))
	for _, f := range a.zip.File {
		if f.FileInfo().IsDir() {
			continue
		}
		entries = append(entries, a.wrap(path.Clean(f.Name), f))
	}
	return entries
}

// Entry implements Archive
func (a *gozipArchive) Entry(p string) (Entry, error) {
	if !fs.ValidPath(p) {
		return nil, fs.ErrNotExist
	}
	cpath := path.Clean(p)

	// Check for an already-wrapped entry first.
	if aentry, ok := a.cachedEntries.Load(cpath); ok {
		return aentry.(Entry), nil
	}

	a.indexOnce.Do(a.buildIndex)
	f, ok := a.nameIndex[cpath]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return a.wrap(cpath, f), nil
}

func NewGoZIPArchive(zip *zip.Reader, closer func() error, minimizeReads bool) Archive {
	return &gozipArchive{
		zip:           zip,
		closer:        closer,
		minimizeReads: minimizeReads,
	}
}

type gozipArchiveFactory struct{}

// Open implements ArchiveFactory
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

// OpenBytes implements ArchiveFactory
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

// OpenReader implements ArchiveFactory
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
