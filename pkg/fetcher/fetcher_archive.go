package fetcher

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/kennygrant/sanitize"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"golang.org/x/text/encoding"
)

// Provides access to entries of an archive.
type ArchiveFetcher struct {
	file    *os.File
	archive *zip.Reader
}

func (f *ArchiveFetcher) Links() []pub.Link {
	links := make([]pub.Link, 0, len(f.archive.File))
	for _, af := range f.archive.File {
		if af.FileInfo().IsDir() {
			continue
		}
		fp := sanitize.Path(af.Name) // No funny business!
		if !strings.HasPrefix(fp, "/") {
			fp = "/" + fp
		}
		link := pub.Link{
			Href: fp,
			Type: mediatype.MediaTypeOfExtension(path.Ext(fp)).String(),
		}
		if af.CompressedSize64 > 0 {
			link.Properties = &pub.Properties{
				Size: af.CompressedSize64, // TODO put this in a better place
			}
		}
		links = append(links, link)
	}
	return links
}

func (f *ArchiveFetcher) Get(link pub.Link) Resource {
	return &EntryResource{
		link:    link,
		archive: f.archive,
	}
}

func (f *ArchiveFetcher) Close() {
	f.file.Close()
}

func NewArchiveFetcher(file *os.File) (*ArchiveFetcher, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	r, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		file:    file,
		archive: r,
	}, nil
}

// TODO
func ArchiveFetcherFromPath(path string, factory interface{}) (*ArchiveFetcher, error) {
	// ArchiveFetcher(archiveFactory.open(File(path), password = null))
	return nil, nil // TODO
}

// Resource from archive entry
type EntryResource struct {
	entry   fs.File
	read    bool
	link    pub.Link
	archive *zip.Reader
}

func (r *EntryResource) Entry() (fs.File, *ResourceException) {
	if r.read {
		// Re-open if already
		r.entry.Close()
		r.entry = nil
	}
	f, err := r.archive.Open(strings.TrimPrefix(r.link.Href, "/"))
	if err != nil {
		return nil, OsErrorToException(err)
	}

	r.entry = f
	return r.entry, nil
}

func (r *EntryResource) File() fs.File {
	en, ex := r.Entry()
	if ex != nil {
		return nil
	}
	return en
}

func (r *EntryResource) Close() {
	if r.entry != nil {
		r.entry.Close()
	}
}

func (r *EntryResource) Link() pub.Link {
	mlen, err := r.Length()
	if mlen > 0 && err == nil {
		if r.link.Properties == nil {
			r.link.Properties = &pub.Properties{}
		}
		r.link.Properties.Size = uint64(mlen)
	}

	return r.link
}

func (r *EntryResource) Read(start int64, end int64) ([]byte, *ResourceException) {
	if end <= start {
		err := RangeNotSatisfiable(errors.New("end of range smaller than start"))
		return nil, &err
	}
	f, ex := r.Entry()
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

func (r *EntryResource) Length() (int64, *ResourceException) {
	f, ex := r.Entry()
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

func (r *EntryResource) ReadAsString(charset encoding.Encoding) (string, *ResourceException) { // TODO determine how charset is needed
	return ReadResourceAsString(r)
}

func (r *EntryResource) ReadAsJSON() (map[string]interface{}, *ResourceException) {
	return ReadResourceAsJSON(r)
}

/*func (r EntryResource) ReadAsXML() (xml.Token, *ResourceException) {
	return ReadResourceAsXML(r)
}*/
