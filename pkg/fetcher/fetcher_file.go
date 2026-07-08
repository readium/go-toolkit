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

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

// Provides access to resources on the local file system.
type FileFetcher struct {
	paths     map[string]string
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
			href, err := manifest.NewHREFFromString(filepath.ToSlash(filepath.Join(href, rel)), false)
			if err != nil {
				return err
			}
			link := manifest.Link{
				Href: href,
			}

			f, err := os.Open(apath)
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

// Get implements Fetcher
func (f *FileFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	// use decoded path for local file lookup to support files with spaces and special characters
	var linkHref string
	if hrefURL := link.Href.Resolve(nil, nil); hrefURL != nil {
		linkHref = hrefURL.Path()
	} else {
		linkHref = link.Href.String()
	}
	for itemHref, itemFile := range f.paths {
		if strings.HasPrefix(linkHref, itemHref) {
			resourceFile := filepath.Join(itemFile, strings.TrimPrefix(linkHref, itemHref))
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
			if rapath == iapath || strings.HasPrefix(rapath, strings.TrimSuffix(iapath, sep)+sep) {
				resource := NewFileResource(link, resourceFile)
				f.resources = append(f.resources, weak.Make(resource))
				return resource
			}
		}
	}
	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in FileFetcher paths")))
}

// Close implements Fetcher
func (f *FileFetcher) Close() {
	// Safety mechanism to cleanup any os.File handles still open
	for _, res := range f.resources {
		if r := res.Value(); r != nil {
			r.Close()
		}
	}
	f.resources = nil
}

func NewFileFetcher(href string, fpath string) *FileFetcher {
	return &FileFetcher{
		paths: map[string]string{href: fpath},
	}
}

type FileResource struct {
	link manifest.Link
	path string

	mu   sync.Mutex // guards file and sequential (offset-based) access to it
	file *os.File
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

// File implements Resource
func (r *FileResource) File() string {
	return r.path
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
	f, err := os.Open(r.path)
	if err != nil {
		return nil, OsErrorToException(err)
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, Other(err)
	}
	if stat.IsDir() {
		return nil, NotFound(errors.New("is a directory"))
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
	if end < start {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	if start < 0 {
		start = 0
	}
	f, ex := r.open()
	if ex != nil {
		return nil, ex
	}
	if start == 0 && end == 0 {
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, Other(err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, Other(err)
		}
		return data, nil
	}
	data := make([]byte, end-start+1)
	n, err := f.ReadAt(data, start)
	if err != nil && err != io.EOF {
		return nil, Other(err)
	}
	return data[:n], nil
}

// Stream implements Resource
func (r *FileResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	defer runtime.KeepAlive(r)
	if end < start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return -1, err
	}
	if start < 0 {
		start = 0
	}
	f, ex := r.open()
	if ex != nil {
		return -1, ex
	}
	if start == 0 && end == 0 {
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return -1, Other(err)
		}
		n, err := io.Copy(w, f)
		if err != nil {
			return -1, Other(err)
		}
		return n, nil
	}
	n, err := io.Copy(w, io.NewSectionReader(f, start, end-start+1))
	if err != nil {
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
