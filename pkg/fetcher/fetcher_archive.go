package fetcher

import (
	"errors"
	"path"
	"strings"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Provides access to entries of an archive.
type ArchiveFetcher struct {
	archive archive.Archive
}

func (f *ArchiveFetcher) Links() ([]pub.Link, error) {
	entries := f.archive.Entries()
	links := make([]pub.Link, 0, len(entries))
	for _, af := range entries {
		fp := path.Clean(af.Path())
		if !strings.HasPrefix(fp, "/") {
			fp = "/" + fp
		}
		link := pub.Link{
			Href: fp,
		}
		ext := path.Ext(fp)
		if ext != "" {
			mt := mediatype.MediaTypeOfExtension(ext[1:]) // Remove leading "."
			if mt != nil {
				link.Type = mt.String()
			}
		}
		cl := af.CompressedLength()
		if cl == 0 {
			cl = af.Length()
		}
		link.Properties.Add(pub.Properties{
			"archive": pub.Properties{
				"entryLength":       cl,
				"isEntryCompressed": af.CompressedLength() > 0,
			},
		})
		links = append(links, link)
	}
	return links, nil
}

func (f *ArchiveFetcher) Get(link pub.Link) Resource {
	entry, err := f.archive.Entry(strings.TrimPrefix(link.Href, "/"))
	if err != nil {
		return NewFailureResource(link, NotFound(err))
	}
	return &entryResource{
		link:  link,
		entry: entry,
	}
}

func (f *ArchiveFetcher) Close() {
	f.archive.Close()
}

func NewArchiveFetcher(a archive.Archive) *ArchiveFetcher {
	return &ArchiveFetcher{
		archive: a,
	}
}

func NewArchiveFetcherFromPath(filepath string) (*ArchiveFetcher, error) {
	archive, err := archive.NewArchiveFactory().Open(filepath, "")
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		archive: archive,
	}, nil
}

func NewArchiveFetcherFromPathWithFactory(path string, factory archive.ArchiveFactory) (*ArchiveFetcher, error) {
	a, err := factory.Open(path, "") // TODO password
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		archive: *a,
	}, nil
}

// Resource from archive entry
type entryResource struct {
	link  pub.Link
	entry archive.Entry
}

func (r *entryResource) File() string {
	return ""
}

func (r *entryResource) Close() {
	// Nothing needs to be done at the moment
}

func (r *entryResource) Link() pub.Link {
	cl := r.entry.CompressedLength()
	if cl == 0 {
		cl = r.entry.Length()
	}
	r.link.Properties.Add(pub.Properties{
		"archive": pub.Properties{
			"entryLength":       cl,
			"isEntryCompressed": r.entry.CompressedLength() > 0,
		},
	})

	return r.link
}

func (r *entryResource) Read(start int64, end int64) ([]byte, *ResourceError) {
	data, err := r.entry.Read(start, end)
	if err == nil {
		return data, nil
	}

	// Bad range
	if err.Error() == "range not satisfiable" {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	// Other error
	return nil, Other(err)
}

func (r *entryResource) Length() (int64, *ResourceError) {
	return int64(r.entry.Length()), nil
}

func (r *entryResource) ReadAsString() (string, *ResourceError) { // TODO determine how charset is needed
	return ReadResourceAsString(r)
}

func (r *entryResource) ReadAsJSON() (map[string]interface{}, *ResourceError) {
	return ReadResourceAsJSON(r)
}

/*func (r entryResource) ReadAsXML() (xml.Token, *ResourceError) {
	return ReadResourceAsXML(r)
}*/
