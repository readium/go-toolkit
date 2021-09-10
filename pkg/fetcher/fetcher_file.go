package fetcher

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kennygrant/sanitize"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"golang.org/x/text/encoding"
)

// Provides access to resources on the local file system.
type FileFetcher struct {
	paths     map[string]string
	resources []Resource // This is weak on mobile
}

func (f *FileFetcher) Links() []pub.Link {
	links := make([]pub.Link, 0)
	for href, xpath := range f.paths {
		axpath, err := filepath.Abs(sanitize.Path(xpath))
		if err == nil {
			xpath = axpath
		}

		filepath.WalkDir(xpath, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() || err != nil {
				return err
			}

			apath, err := filepath.Abs(sanitize.Path(path))
			if err == nil {
				return err
			}

			link := pub.Link{
				Href: filepath.Join(href, strings.TrimPrefix(apath, xpath)), // TODO double-check comparison to https://github.com/readium/r2-shared-kotlin/blob/develop/r2-shared/src/main/java/org/readium/r2/shared/fetcher/FileFetcher.kt#L48
			}

			f, err := os.Open(apath)
			if err == nil {
				link.Type = mediatype.MediaTypeOfFileOnly(f).String()
			} else {
				ext := filepath.Ext(f.Name())
				if ext != "" {
					link.Type = mediatype.MediaTypeOfExtension(ext[1:]).String()
				}
			}
			links = append(links, link)
			return nil
		})
	}
	return links
}

func (f *FileFetcher) Get(link pub.Link) Resource {
	linkHref := link.Href
	if !strings.HasPrefix(linkHref, "/") {
		linkHref = "/" + linkHref
	}
	for itemHref, itemFile := range f.paths {
		if !strings.HasPrefix(itemHref, "/") {
			itemHref = "/" + itemHref
		}
		if strings.HasPrefix(linkHref, itemHref) {
			resourceFile := filepath.Join(itemFile, strings.TrimPrefix(linkHref, itemHref))
			// Make sure that the requested resource is [path] or one of its descendant.
			rapath, err := filepath.Abs(sanitize.Path(resourceFile))
			if err != nil {
				continue // TODO somehow get this error out?
			}
			iapath, err := filepath.Abs(sanitize.Path(itemFile))
			if err != nil {
				continue // TODO somehow get this error out?
			}
			if strings.HasPrefix(rapath, iapath) {
				resource := NewFileResource(link, resourceFile)
				f.resources = append(f.resources, nil)
				return resource
			}
		}
	}
	return NewFailureResource(link, NotFound(nil))
}

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
	link pub.Link
	path string
	file *os.File
	read bool
}

func (r *FileResource) Link() pub.Link {
	return r.link
}

func (r *FileResource) Close() {
	if r.file != nil {
		r.file.Close()
	}
}

func (r *FileResource) File() fs.File {
	return r.file
}

func (r *FileResource) open() (*os.File, *ResourceException) {
	if r.file != nil {
		r.file.Seek(0, io.SeekStart)
		return r.file, nil
	}
	f, err := os.Open(r.path)
	if err != nil {
		return nil, OsErrorToException(err)
	}
	r.file = f
	return f, nil
}

func (r *FileResource) Read(start int64, end int64) ([]byte, *ResourceException) {
	if end <= start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return nil, &err
	}
	f, ex := r.open()
	if ex != nil {
		return nil, ex
	}
	r.read = true
	if start == 0 && end == 0 {
		data, err := io.ReadAll(f)
		if err != nil {
			ex := Other(err)
			return nil, &ex
		}
		return data, nil
	}
	if start > 0 {
		_, err := io.CopyN(io.Discard, f, start)
		if err != nil {
			ex := Other(err)
			return nil, &ex
		}
	}
	data := make([]byte, end-start+1)
	_, err := f.Read(data)
	if err != nil {
		ex := Other(err)
		return nil, &ex
	}
	return data, nil
}

func (r *FileResource) Length() (int64, *ResourceException) {
	f, ex := r.open()
	if ex != nil {
		return 0, ex
	}
	fi, err := f.Stat()
	if err != nil {
		ex := Other(err)
		return 0, &ex
	}
	return fi.Size(), nil
}

func (r *FileResource) ReadAsString(charset encoding.Encoding) (string, *ResourceException) { // TODO determine how charset is needed
	return ReadResourceAsString(r)
}

func (r *FileResource) ReadAsJSON() (map[string]interface{}, *ResourceException) {
	return ReadResourceAsJSON(r)
}

/*func (r FileResource) ReadAsXML() (xml.Token, *ResourceException) {
	return ReadResourceAsXML(r)
}*/

func NewFileResource(link pub.Link, abspath string) *FileResource {
	return &FileResource{
		link: link,
		path: abspath,
	}
}
