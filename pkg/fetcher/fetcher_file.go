package fetcher

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"weak"

	"github.com/readium/go-toolkit/pkg/internal/byterange"
	"github.com/readium/go-toolkit/pkg/internal/rootfs"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

// Provides access to resources on the local file system.
type FileFetcher struct {
	paths map[string]string

	mu        sync.Mutex // guards resources, which concurrent Gets append to
	resources []weak.Pointer[FileResource]
}

// Links implements Fetcher
func (f *FileFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	links := make(manifest.LinkList, 0)
	for href, xpath := range f.paths {
		axpath, err := filepath.Abs(xpath)
		if err == nil {
			xpath = axpath
		}

		err = filepath.WalkDir(xpath, func(apath string, d fs.DirEntry, err error) error {
			if d == nil { // xpath is a file
				fi, err := os.Stat(xpath)
				if err != nil {
					return err
				}
				d = fs.FileInfoToDirEntry(fi)
			}

			if d.IsDir() || err != nil {
				return err
			}

			rel := strings.TrimPrefix(strings.TrimPrefix(apath, xpath), string(filepath.Separator))
			open := func() (*os.File, error) { return os.Open(apath) }
			if rel != "" {
				// Only regular files of the directory are listed. Symbolic
				// links could lead to any file of the system, and named
				// pipes would block reads.
				switch {
				case d.Type().IsRegular():
				case d.Type()&fs.ModeSymlink != 0 && rootfs.IsRegular(xpath, rel):
				default:
					return nil
				}
				open = func() (*os.File, error) { return rootfs.Open(xpath, rel) }
			}
			href, err := fileHREF(filepath.ToSlash(filepath.Join(href, rel)))
			if err != nil {
				return err
			}
			link := manifest.Link{
				Href: href,
			}

			f, err := open()
			if err == nil {
				defer f.Close()
				mt := mediatype.OfFileOnly(ctx, f)
				if mt != nil {
					link.MediaType = mt
				}
			} else {
				ext := filepath.Ext(apath)
				if ext != "" {
					mt := mediatype.OfExtension(ext[1:])
					if mt != nil {
						link.MediaType = mt
					}
				}
			}
			links = append(links, link)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return links, nil
}

// Exhaustive implements [Exhaustive]: the links are the files of the
// directories, walked recursively.
func (f *FileFetcher) Exhaustive() bool { return true }

// Get implements Fetcher
func (f *FileFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	// use decoded path for local file lookup to support files with spaces and special characters
	linkHref, ok := localPath(link)
	if !ok {
		return NewFailureResource(link, NotFound(errNotLocal))
	}
	for itemHref, itemFile := range f.paths {
		if after, ok := strings.CutPrefix(linkHref, itemHref); ok {
			resourceFile := filepath.Join(itemFile, after)
			// Make sure that the requested resource is [path] or one of its descendant.
			rapath, err := filepath.Abs(filepath.ToSlash(resourceFile))
			if err != nil {
				continue // TODO somehow get this error out?
			}
			iapath, err := filepath.Abs(filepath.ToSlash(itemFile))
			if err != nil {
				continue // TODO somehow get this error out?
			}
			// The path must be [itemFile] itself, or sit below it beyond a separator:
			// a plain prefix check would let "dir-other" pass as a descendant of "dir".
			sep := string(filepath.Separator)
			var resource *FileResource
			if rapath == iapath {
				resource = NewFileResource(link, resourceFile)
			} else if strings.HasPrefix(rapath, strings.TrimSuffix(iapath, sep)+sep) {
				name, err := filepath.Rel(iapath, rapath)
				if err != nil {
					continue
				}
				// Opened within the directory, so that symbolic links can't
				// lead outside of it.
				resource = &FileResource{link: link, path: resourceFile, root: iapath, name: name}
			} else {
				continue
			}
			f.mu.Lock()
			f.resources = append(f.resources, weak.Make(resource))
			f.mu.Unlock()
			return resource
		}
	}
	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in FileFetcher paths")))
}

// Close implements Fetcher
func (f *FileFetcher) Close() {
	f.mu.Lock()
	resources := f.resources
	f.resources = nil
	f.mu.Unlock()
	// Safety mechanism to cleanup any os.File handles still open
	for _, res := range resources {
		if r := res.Value(); r != nil {
			r.Close()
		}
	}
}

func NewFileFetcher(href string, fpath string) *FileFetcher {
	return &FileFetcher{
		paths: map[string]string{href: fpath},
	}
}

type FileResource struct {
	link manifest.Link
	path string
	// root is the directory of the fetcher the file is opened within, and
	// name its path relative to it, when the file is below one.
	root string
	name string

	mu   sync.Mutex // guards lazily opening and closing file, and pin
	file *os.File
	pin  fs.FileInfo // the file the resource is pinned to, if any
}

// Link implements Resource
func (r *FileResource) Link() manifest.Link {
	return r.link
}

// Properties implements Resource
func (r *FileResource) Properties() manifest.Properties {
	return manifest.Properties{}
}

// Close implements Resource
func (r *FileResource) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		r.file.Close()
	}
}

// File implements Resource. It is empty when the file is a symbolic link
// leading outside of the directory of the fetcher, or not a regular file. It
// is empty for a resource pinned to a file as well: the file its path names
// may be another one, which consumers opening it would serve instead.
func (r *FileResource) File() string {
	r.mu.Lock()
	pinned := r.pin != nil
	r.mu.Unlock()
	if pinned || (r.root != "" && !rootfs.IsRegular(r.root, r.name)) {
		return ""
	}
	return r.path
}

// FileInfo implements FileBacked
func (r *FileResource) FileInfo() (fs.FileInfo, error) {
	if r.root != "" {
		return rootfs.Stat(r.root, r.name)
	}
	return os.Stat(r.path)
}

// PinFile implements FilePinner
func (r *FileResource) PinFile(info fs.FileInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pin = info
	// A file opened before may be another one.
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
	return nil
}

// openFile opens the file of the resource, which must be a regular file. A
// file below a directory of the fetcher is opened within it: symbolic links
// can't lead outside of it. When the resource is pinned to a file, given as
// pin, the opened file must be that one.
func (r *FileResource) openFile(pin fs.FileInfo) (*os.File, *ResourceError) {
	var f *os.File
	var err error
	if r.root != "" {
		f, err = rootfs.Open(r.root, r.name)
	} else {
		f, err = openRegular(r.path)
	}
	if err != nil {
		if errors.Is(err, rootfs.ErrNotRegular) {
			return nil, NotFound(err)
		}
		return nil, OsErrorToException(err)
	}
	if pin != nil {
		fi, err := f.Stat()
		if err != nil || !os.SameFile(pin, fi) {
			_ = f.Close()
			return nil, Forbidden(ErrFileChanged)
		}
	}
	return f, nil
}

// openRegular opens a regular file, refusing others before opening them:
// opening a named pipe blocks until something writes to it.
func openRegular(path string) (*os.File, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: path, Err: rootfs.ErrNotRegular}
	}
	return os.Open(path)
}

// open returns the lazily-opened file handle. The returned *os.File is only
// safe for position-independent access (ReadAt, Stat); sequential access that
// moves the file offset must hold r.mu.
func (r *FileResource) open() (*os.File, *ResourceError) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return r.file, nil
	}
	f, rerr := r.openFile(r.pin)
	if rerr != nil {
		return nil, rerr
	}
	r.file = f
	runtime.AddCleanup(r, func(f *os.File) {
		f.Close()
	}, f)
	return f, nil
}

// Read implements Resource. Ranged reads (end > 0) are safe for concurrent use.
func (r *FileResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	defer runtime.KeepAlive(r)
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	f, ex := r.open()
	if ex != nil {
		return nil, ex
	}
	if start == 0 && end == 0 {
		fi, err := f.Stat()
		if err != nil {
			return nil, Other(err)
		}
		data := make([]byte, fi.Size())
		n, err := f.ReadAt(data, 0)
		if err != nil && err != io.EOF {
			return nil, Other(err)
		}
		return data[:n], nil
	}
	// Ranges are allocated, so they are clamped to the file first: callers
	// may ask for more than there is.
	fi, err := f.Stat()
	if err != nil {
		return nil, Other(err)
	}
	if start >= fi.Size() {
		return []byte{}, nil
	}
	end = min(end, fi.Size()-1)
	data := make([]byte, end-start+1)
	n, err := f.ReadAt(data, start)
	if err != nil && err != io.EOF {
		return nil, Other(err)
	}
	return data[:n], nil
}

// Stream implements Resource
func (r *FileResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return -1, err
	}
	// Streaming uses a private handle per call: the offset-based access below
	// never contends with concurrent streams of the same resource, and the
	// source stays a bare *os.File (wrapped only by io.CopyN's
	// *io.LimitedReader, which the runtime unwraps), preserving the kernel
	// sendfile fast path when w is a network connection. An io.SectionReader
	// here would force a userspace copy loop.
	r.mu.Lock()
	pin := r.pin
	r.mu.Unlock()
	f, rerr := r.openFile(pin)
	if rerr != nil {
		return -1, rerr
	}
	defer f.Close()
	if start == 0 && end == 0 {
		n, err := io.Copy(w, f)
		if err != nil {
			return -1, Other(err)
		}
		return n, nil
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return -1, Other(err)
	}
	n, err := io.CopyN(w, f, byterange.Length(start, end))
	if err != nil && err != io.EOF {
		return n, Other(err)
	}
	return n, nil
}

// Length implements Resource
func (r *FileResource) Length(ctx context.Context) (int64, *ResourceError) {
	defer runtime.KeepAlive(r)
	f, ex := r.open()
	if ex != nil {
		return 0, ex
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, Other(err)
	}
	return fi.Size(), nil
}

func NewFileResource(link manifest.Link, abspath string) *FileResource {
	return &FileResource{
		link: link,
		path: abspath,
	}
}
