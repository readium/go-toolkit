package fetcher

import (
	"errors"
	"io"
	"path"
	"strings"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/xmlquery"
)

// Provides access to entries of an archive.
type ArchiveFetcher struct {
	archive archive.Archive
}

// Links implements Fetcher
func (f *ArchiveFetcher) Links() ([]manifest.Link, error) {
	entries := f.archive.Entries()
	links := make([]manifest.Link, 0, len(entries))
	for _, af := range entries {
		fp := path.Clean(af.Path())
		if !strings.HasPrefix(fp, "/") {
			fp = "/" + fp
		}
		link := manifest.Link{
			Href: fp,
		}
		ext := path.Ext(fp)
		if ext != "" {
			mt := mediatype.OfExtension(ext[1:]) // Remove leading "."
			if mt != nil {
				link.Type = mt.String()
			}
		}
		cl := af.CompressedLength()
		if cl == 0 {
			cl = af.Length()
		}
		link.Properties.Add(manifest.Properties{
			"https://readium.org/webpub-manifest/properties#archive": manifest.Properties{
				"entryLength":       cl,
				"isEntryCompressed": af.CompressedLength() > 0,
			},
		})
		links = append(links, link)
	}
	return links, nil
}

// Get implements Fetcher
func (f *ArchiveFetcher) Get(link manifest.Link) Resource {
	entry, err := f.archive.Entry(strings.TrimPrefix(link.Href, "/"))
	if err != nil {
		return NewFailureResource(link, NotFound(err))
	}
	return &entryResource{
		link:  link,
		entry: entry,
	}
}

// Close implements Fetcher
func (f *ArchiveFetcher) Close() {
	f.archive.Close()
}

func NewArchiveFetcher(a archive.Archive) *ArchiveFetcher {
	return &ArchiveFetcher{
		archive: a,
	}
}

func NewArchiveFetcherFromPath(filepath string) (*ArchiveFetcher, error) {
	return NewArchiveFetcherFromPathWithFactory(filepath, archive.NewArchiveFactory())
}

func NewArchiveFetcherFromPathWithFactory(path string, factory archive.ArchiveFactory) (*ArchiveFetcher, error) {
	a, err := factory.Open(path, "") // TODO password
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		archive: a,
	}, nil
}

// Resource from archive entry
type entryResource struct {
	link  manifest.Link
	entry archive.Entry
}

// File implements Resource
func (r *entryResource) File() string {
	return ""
}

// Close implements Resource
func (r *entryResource) Close() {
	// Nothing needs to be done at the moment
}

func (r *entryResource) Link() manifest.Link {
	cl := r.entry.CompressedLength()
	if cl == 0 {
		cl = r.entry.Length()
	}
	r.link.Properties.Add(manifest.Properties{
		"https://readium.org/webpub-manifest/properties#archive": manifest.Properties{
			"entryLength":       cl,
			"isEntryCompressed": r.entry.CompressedLength() > 0,
		},
	})

	return r.link
}

// Read implements Resource
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

// Stream implements Resource
func (r *entryResource) Stream(w io.Writer, start int64, end int64) (int64, *ResourceError) {
	n, err := r.entry.Stream(w, start, end)
	if err == nil {
		return n, nil
	}

	// Bad range
	if err.Error() == "range not satisfiable" {
		return -1, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	// Other error
	return -1, Other(err)
}

// Length implements Resource
func (r *entryResource) Length() (int64, *ResourceError) {
	return int64(r.entry.Length()), nil
}

// ReadAsString implements Resource
func (r *entryResource) ReadAsString() (string, *ResourceError) { // TODO determine how charset is needed
	return ReadResourceAsString(r)
}

// ReadAsJSON implements Resource
func (r *entryResource) ReadAsJSON() (map[string]interface{}, *ResourceError) {
	return ReadResourceAsJSON(r)
}

// ReadAsXML implements Resource
func (r *entryResource) ReadAsXML(prefixes map[string]string) (*xmlquery.Node, *ResourceError) {
	return ReadResourceAsXML(r, prefixes)
}
