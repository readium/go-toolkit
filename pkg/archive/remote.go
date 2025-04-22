package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type RemoteArchiveConfig struct {
	Timeout             time.Duration // Timeout for remote requests to read from the archive
	CacheAllThreshold   int64         // Threshold for caching the entire ZIP
	CacheSizeThreshold  int64         // Threshold for caching of a single entry in the ZIP
	CacheCountThreshold int64         // Threshold for the number of entries in the ZIP to cache
}

func (c RemoteArchiveConfig) Empty() bool {
	return c.Timeout == 0 && c.CacheSizeThreshold == 0 && c.CacheCountThreshold == 0 && c.CacheAllThreshold == 0
}

func NewDefaultRemoteArchiveConfig() RemoteArchiveConfig {
	return RemoteArchiveConfig{
		Timeout:             time.Second * 60, // 1 minute
		CacheSizeThreshold:  1024 * 1024,      // 1MB
		CacheCountThreshold: 32,               // 32 items
		CacheAllThreshold:   1024 * 1024,      // 1MB
	}
}

type RemoteArchiveReader interface {
	Size() int64                                                                // Size of the remote archive object
	ReadRange(ctx context.Context, offset, length int64) (io.ReadCloser, error) // Negative length means "read to the end"
}

type readRange struct {
	HeaderOffset int64    // Offset of the local file header
	Offset       int64    // Offset of the file body
	Size         int64    // Size of the file body in the archive
	Header       [30]byte // Local file header
	Data         []byte   // File body
}

// Read ZIP archives from the a remote location efficiently
type remoteZIPAdapter struct {
	rdr      RemoteArchiveReader // Remote archive reader
	zipReady bool                // Is the ZIP file opened by Go's zip reader?
	timeout  time.Duration       // // Timeout for remote requests to read from the archive

	cacheAllThreshold   int64        // Threshold for caching the entire ZIP
	cacheSizeThreshold  int64        // Threshold for caching of a single entry in the ZIP
	cacheCountThreshold int64        // Threshold for the number of entries in the ZIP to cache
	cachedRanges        []readRange  // Cached byte ranges of the ZIP file
	cacheMutex          sync.RWMutex // Mutex for the cached ranges
	completeBytes       []byte       // Entire ZIP file in memory

	// No mutex here, because it's only set once during the ZIP opening procedure
	zipTail     []byte
	zipTailSize int64
}

func (r *remoteZIPAdapter) cacheAll() bool {
	return r.rdr.Size() <= r.cacheAllThreshold
}

// ReadAt implements io.ReaderAt
func (r *remoteZIPAdapter) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("read negative offset")
	}

	if len(p) == 0 {
		return 0, errors.New("read into empty byte slice")
	}

	// Limited amount of time to perform the read
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	if r.cacheAll() { // Read from a complete in-memory copy of the publication
		if len(r.completeBytes) == 0 {
			rdr, err := r.rdr.ReadRange(ctx, 0, r.rdr.Size())
			if err != nil {
				return 0, err
			}
			defer rdr.Close()
			r.completeBytes = make([]byte, r.rdr.Size())
			n, err := io.ReadFull(rdr, r.completeBytes) // Read the entire object into memory
			if err != nil {
				return n, err
			}
		}
		// Perform ReadAt on the in-memory copy of the publication
		return bytes.NewReader(r.completeBytes).ReadAt(p, off)
	}

	// Special accomodation to speed up zip reader scanning the end of the file for the central directory
	if !r.zipReady {
		tailOffset := r.rdr.Size() - r.zipTailSize
		newOff := off - tailOffset

		if newOff < 0 {
			// The central directory is really long, we can't use the cached version
			// Instead, we increase its size to include the requested offset
			r.zipTail = nil
			r.zipTailSize -= newOff
			tailOffset = r.rdr.Size() - r.zipTailSize
			newOff = off - tailOffset
		}
		if len(r.zipTail) > 0 {
			n := copy(p, r.zipTail[newOff:newOff+int64(len(p))])
			return n, nil
		}
		newZipTail := make([]byte, r.zipTailSize)

		rdr, err := r.rdr.ReadRange(ctx, tailOffset, r.rdr.Size())
		if err != nil {
			return 0, err
		}
		defer rdr.Close()
		_, err = io.ReadFull(rdr, newZipTail) // Read tail of file into memory
		if err != nil {
			newZipTail = nil
			return 0, err
		}
		n := copy(p, newZipTail[newOff:newOff+int64(len(p))])
		r.zipTail = newZipTail
		return n, nil
	}

	size := int64(len(p))
	var n int

	if size == 30 && r.cacheCountThreshold > 0 && r.cacheSizeThreshold > 0 {
		// 30 bytes is the size of a ZIP's local file header
		// There could theoretically be a real file with compressed or uncompressed length of 30 bytes,
		// but this is not that likely in an EPUB. So this is a good enough heuristic to use.

		// First, check if we've already read this header as a shortcut
		r.cacheMutex.RLock()
		for _, rng := range r.cachedRanges {
			if rng.HeaderOffset == off {
				r.cacheMutex.RUnlock()
				return copy(p, rng.Header[:]), nil
			}
		}
		r.cacheMutex.RUnlock()

		// We start reading at the offset of the local file header, with the assumption that the actual
		// file content follows right after. This way, we only need to start a read from the remote *one* time.
		rdr, err := r.rdr.ReadRange(ctx, off, -1)
		if err != nil {
			return 0, err
		}

		var fileHeaderBuf [30]byte
		n, err = rdr.Read(fileHeaderBuf[:])
		if err != nil {
			rdr.Close()
			return 0, errors.Wrap(err, "failed reading local file header bytes")
		}
		if fileHeaderBuf[0] == 'P' && fileHeaderBuf[1] == 'K' && fileHeaderBuf[2] == 0x03 && fileHeaderBuf[3] == 0x04 {
			// PK\x05\x06 is the signature of a ZIP's local file header. This confirms our suspsicion that it's
			// what it seems. The possibility of it being something else is very very low at this point.

			// Get compression method
			compressionMethod := binary.LittleEndian.Uint16(fileHeaderBuf[8:])
			var bodySize uint32

			b := fileHeaderBuf[18:]

			compressedSize := binary.LittleEndian.Uint32(b)
			uncompressedSize := binary.LittleEndian.Uint32(b[4:])

			if compressedSize == 0 && uncompressedSize == 0 {
				// No file size given. It's not great, but it's technically still valid.
				// Happens especially if the author of the ZIP is streaming the contents into it,
				// e.g. with Go, where if you write a streaming ZIP, the size is not known in advance.

				// We can still at least cache the file header
				r.cacheMutex.Lock()
				if len(r.cachedRanges) >= int(r.cacheCountThreshold) {
					// Remove the oldest range
					r.cachedRanges = r.cachedRanges[1:]
				}

				r.cachedRanges = append(r.cachedRanges, readRange{
					HeaderOffset: off,
					Header:       fileHeaderBuf,
				})
				r.cacheMutex.Unlock()
			} else if compressedSize == 0xFFFFFFFF && uncompressedSize == 0xFFFFFFFF {
				// ZIP64 is not supported by this routine
			} else {
				if compressionMethod == zip.Store {
					// File is uncompressed
					bodySize = uncompressedSize
				} else {
					// File is compressed
					bodySize = compressedSize
				}

				// Now the important part - we precache the actual file!

				// ...but only if it's not too big
				if int64(bodySize) <= r.cacheSizeThreshold {
					// Remaining local file headers are needed to get the total size of useless stuff
					filenameLength := binary.LittleEndian.Uint16(b[8:])
					extraFieldLength := binary.LittleEndian.Uint16(b[10:])
					useless := int64(extraFieldLength) + int64(filenameLength)
					bodyOffset := off + 30 + useless

					r.cacheMutex.RLock()
					var hasSameRange bool
					for _, rng := range r.cachedRanges {
						if rng.Offset == bodyOffset && rng.Size == int64(bodySize) {
							hasSameRange = true
							break
						}
					}
					r.cacheMutex.RUnlock()
					if !hasSameRange {
						// Allocate a slice to hold the filename, extra field and file body
						rest := make([]byte, int64(bodySize)+useless)
						_, err := io.ReadAtLeast(rdr, rest, len(rest))
						if err != nil {
							rdr.Close()
							return 0, errors.Wrap(err, "failed reading rest of zip file bytes for precaching")
						}

						// Write to cache
						r.cacheMutex.Lock()
						if len(r.cachedRanges) >= int(r.cacheCountThreshold) {
							// Remove the oldest range
							r.cachedRanges = r.cachedRanges[1:]
						}

						r.cachedRanges = append(r.cachedRanges, readRange{
							HeaderOffset: off,
							Offset:       bodyOffset,
							Size:         int64(bodySize),
							Header:       fileHeaderBuf,
							Data:         rest[useless:], // Trim off the filename and extra field, just store the body
						})
						r.cacheMutex.Unlock()
					}
				}
			}
		}
		copy(p, fileHeaderBuf[:]) // Copy the 30 read bytes
		io.Copy(io.Discard, rdr)  // Discard the rest of the read
		rdr.Close()               // Then close it
	} else {
		// Check all the cache ranges to see if what we're looking for is somewhere inside a cached range
		// This is especially useful when doing a range read / stream of e.g. 4096-byte chunks
		r.cacheMutex.RLock()
		for _, rng := range r.cachedRanges {
			if off >= rng.Offset && off < rng.Offset+rng.Size && off+size <= rng.Offset+rng.Size {
				// Found a range that contains the requested range
				// Extract the relevant part of the range
				n = copy(p, rng.Data[off-rng.Offset:off-rng.Offset+size])
				r.cacheMutex.RUnlock()
				return n, nil
			}
		}
		r.cacheMutex.RUnlock()

		// Cache miss, need to read a brand new range
		rdr, err := r.rdr.ReadRange(ctx, off, size)
		if err != nil {
			return 0, err
		}
		defer rdr.Close()

		n, err = io.ReadFull(rdr, p) // Read range into containing slice
		if err != nil {
			return n, err
		}

		if size > r.cacheSizeThreshold {
			return n, nil // Too big to cache, just return
		}
	}

	// Write to cache
	r.cacheMutex.Lock()
	var hasSameRange bool
	for _, rng := range r.cachedRanges {
		if rng.Offset == off && rng.Size == size {
			hasSameRange = true
			break
		}
	}
	if !hasSameRange {
		if len(r.cachedRanges) >= int(r.cacheCountThreshold) {
			// Remove the oldest range
			r.cachedRanges = r.cachedRanges[1:]
		}

		r.cachedRanges = append(r.cachedRanges, readRange{
			HeaderOffset: -1,
			Offset:       off,
			Size:         size,
			Data:         p,
		})
	}
	r.cacheMutex.Unlock()

	return n, nil
}

func (r *remoteZIPAdapter) makeReady() {
	r.zipReady = true
	r.zipTail = nil
}

func (r *remoteZIPAdapter) Close() error {
	clear(r.cachedRanges)
	return nil
}

func newRemoteZIPAdapter(rdr RemoteArchiveReader, config RemoteArchiveConfig) *remoteZIPAdapter {
	if config.Empty() {
		config = NewDefaultRemoteArchiveConfig()
	}
	r := &remoteZIPAdapter{
		rdr:                 rdr,
		timeout:             config.Timeout,
		cacheSizeThreshold:  config.CacheSizeThreshold,
		cacheCountThreshold: config.CacheCountThreshold,
		cacheAllThreshold:   config.CacheAllThreshold,
		zipTailSize:         65 * 1024, // 65KB
	}
	if !r.cacheAll() {
		r.cachedRanges = make([]readRange, 0, r.cacheCountThreshold)
	}
	return r
}
