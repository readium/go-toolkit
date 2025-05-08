package fetcher

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"sync/atomic"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type resourceInfo struct {
	Resource
	length int64
}

// IsDir implements [fs.FileInfo]
func (r resourceInfo) IsDir() bool {
	return false
}

// ModTime implements [fs.FileInfo]
func (r resourceInfo) ModTime() time.Time {
	return time.Time{} // Zero time
}

// Mode implements [fs.FileInfo]
func (r resourceInfo) Mode() fs.FileMode {
	return 0444 // Read-only
}

// Name implements [fs.FileInfo]
func (r resourceInfo) Name() string {
	return path.Base(r.Resource.Link().Href.String())
}

// Size implements [fs.FileInfo]
func (r resourceInfo) Size() int64 {
	return r.length
}

// Sys implements [fs.FileInfo]
func (r resourceInfo) Sys() any {
	return r.Resource
}

type fsResource struct {
	r      Resource
	offset atomic.Int64
	ctx    context.Context
}

// Close implements [fs.File]
func (f *fsResource) Close() error {
	f.r.Close()
	return nil
}

// ReadAt implements [io.ReaderAt]
func (f *fsResource) ReadAt(b []byte, off int64) (int, error) {
	bin, rerr := f.r.Read(f.ctx, off, off+int64(len(b))-1)
	if rerr != nil {
		if rerr.Cause == io.EOF {
			copy(b, bin)
			return len(bin), io.EOF
		}
		return len(bin), rerr
	}
	return copy(b, bin), nil
}

// Seek implements [io.Seeker]
func (f *fsResource) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset.Store(offset)
		return offset, nil
	case io.SeekCurrent:
		return f.offset.Add(offset), nil
	case io.SeekEnd:
		length, err := f.r.Length(f.ctx)
		if err != nil {
			return length, err
		}
		newOffset := length + offset
		f.offset.Store(newOffset)
		return newOffset, nil
	default:
		return -1, errors.New("invalid whence")
	}
}

// Read implements [fs.File]
func (f *fsResource) Read(b []byte) (int, error) {
	blen := int64(len(b))
	currentOffset := f.offset.Add(blen) - blen
	bin, rerr := f.r.Read(f.ctx, currentOffset, currentOffset+blen-1)
	if rerr != nil {
		if rerr.Cause == io.EOF {
			copy(b, bin)
			return len(bin), io.EOF
		}
		return len(bin), rerr
	}
	// Out-of-range indexes are clamped to the available length automatically when calling `Read`
	// That means we need to find the EOF ourselves by comparing the length requested and returned
	if len(bin) < len(b) {
		if len(bin) > 0 {
			copy(b, bin)
		}
		return len(bin), io.EOF
	}
	return copy(b, bin), nil
}

// Stat implements [fs.File]
func (f *fsResource) Stat() (fs.FileInfo, error) {
	length, err := f.r.Length(f.ctx)
	if err != nil {
		return nil, err
	}

	return resourceInfo{
		Resource: f.r,
		length:   length,
	}, nil
}

// TODO: directory listing support
type fsFetcher struct {
	Fetcher
	ctx context.Context
}

func (f fsFetcher) get(name string) (Resource, error) {
	u, err := url.URLFromString(name)
	if err != nil {
		return nil, err
	}

	return f.Get(f.ctx, manifest.Link{Href: manifest.NewHREF(u)}), nil
}

// Stat implements [fs.StatFS]
func (f fsFetcher) Stat(name string) (fs.FileInfo, error) {
	r, err := f.get(name)
	if err != nil {
		return nil, err
	}

	length, rerr := r.Length(f.ctx)
	if rerr != nil {
		return nil, rerr
	}

	return resourceInfo{
		Resource: r,
		length:   length,
	}, nil
}

// Open implements [fs.FS]
func (f fsFetcher) Open(name string) (fs.File, error) {
	r, err := f.get(name)
	if err != nil {
		return nil, err
	}

	return &fsResource{r: r, ctx: f.ctx}, nil
}

// Turn a [Fetcher] into a [fs.FS] virtual filesystem
func ToFS(ctx context.Context, f Fetcher) fsFetcher {
	return fsFetcher{f, ctx}
}

// Turn a [Resource] into a [fs.File] virtual file
func ToFSFile(ctx context.Context, r Resource) fs.File {
	return &fsResource{r: r, ctx: ctx}
}
