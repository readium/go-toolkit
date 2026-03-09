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

			href, err := manifest.NewHREFFromString(filepath.ToSlash(filepath.Join(href, strings.TrimPrefix(apath, xpath))), false)
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
			if strings.HasPrefix(rapath, iapath) {
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
	file *os.File
	read bool
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
	if r.file != nil {
		r.file.Close()
	}
}

// File implements Resource
func (r *FileResource) File() string {
	return r.path
}

func (r *FileResource) open() (*os.File, *ResourceError) {
	if r.file != nil {
		if _, err := r.file.Seek(0, io.SeekStart); err != nil {
			return nil, Other(err)
		}
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

// Read implements Resource
func (r *FileResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	defer runtime.KeepAlive(r)
	if end < start {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	f, ex := r.open()
	if ex != nil {
		return nil, ex
	}
	r.read = true
	if start == 0 && end == 0 {
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, Other(err)
		}
		return data, nil
	}
	data := make([]byte, end-start+1)
	if start > 0 {
		n, err := f.ReadAt(data, start)
		if err != nil && err != io.EOF {
			return nil, Other(err)
		}
		return data[:n], nil
	} else {
		n, err := io.ReadFull(f, data)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, Other(err)
		}
		return data[:n], nil
	}
}

// Stream implements Resource
func (r *FileResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	defer runtime.KeepAlive(r)
	if end < start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return -1, err
	}
	f, ex := r.open()
	if ex != nil {
		return -1, ex
	}
	r.read = true
	if start == 0 && end == 0 {
		n, err := io.Copy(w, f)
		if err != nil {
			return -1, Other(err)
		}
		return n, nil
	}
	if start > 0 {
		_, err := f.Seek(start, 0)
		if err != nil {
			return -1, Other(err)
		}
	}
	n, err := io.CopyN(w, f, end-start+1)
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
