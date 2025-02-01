package fetcher

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/xmlquery"
)

// Provides access to resources on the local file system.
type FileFetcher struct {
	paths     map[string]string
	resources []Resource // This is weak on mobile
}

// Links implements Fetcher
func (f *FileFetcher) Links() (manifest.LinkList, error) {
	links := make(manifest.LinkList, 0)
	for href, xpath := range f.paths {
		axpath, err := filepath.Abs(xpath)
		if err == nil {
			xpath = axpath
		}

		err = filepath.WalkDir(xpath, func(apath string, d fs.DirEntry, err error) error {
			if d == nil { // xpath is afile
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
				mt := mediatype.OfFileOnly(f)
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
func (f *FileFetcher) Get(link manifest.Link) Resource {
	linkHref := link.Href.String()
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
				f.resources = append(f.resources, resource)
				return resource
			}
		}
	}
	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in FileFetcher paths")))
}

// Close implements Fetcher
func (f *FileFetcher) Close() {
	for _, res := range f.resources {
		res.Close()
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
		r.file.Seek(0, io.SeekStart)
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
	return f, nil
}

// Read implements Resource
func (r *FileResource) Read(start int64, end int64) ([]byte, *ResourceError) {
	if end < start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return nil, err
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
		n, err := f.Read(data)
		if err != nil && err != io.EOF {
			return nil, Other(err)
		}
		return data[:n], nil
	}
}

// Stream implements Resource
func (r *FileResource) Stream(w io.Writer, start int64, end int64) (int64, *ResourceError) {
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
func (r *FileResource) Length() (int64, *ResourceError) {
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

// ReadAsString implements Resource
func (r *FileResource) ReadAsString() (string, *ResourceError) {
	return ReadResourceAsString(r)
}

// ReadAsJSON implements Resource
func (r *FileResource) ReadAsJSON() (map[string]interface{}, *ResourceError) {
	return ReadResourceAsJSON(r)
}

// ReadAsXML implements Resource
func (r *FileResource) ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError) {
	return ReadResourceAsXML(r, prefixes)
}

func NewFileResource(link manifest.Link, abspath string) *FileResource {
	return &FileResource{
		link: link,
		path: abspath,
	}
}
