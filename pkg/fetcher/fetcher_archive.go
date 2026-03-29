package fetcher

import (
	"context"
	"errors"
	"io"
	"path"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Provides access to entries of an archive.
type ArchiveFetcher struct {
	archive archive.Archive
}

// Links implements Fetcher
func (f *ArchiveFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	entries := f.archive.Entries()
	links := make(manifest.LinkList, 0, len(entries))
	for _, af := range entries {
		fp := path.Clean(af.Path())
		href, err := manifest.NewHREFFromString(fp, false)
		if err != nil {
			return nil, err
		}
		link := manifest.Link{
			Href: href,
		}
		ext := path.Ext(fp)
		if ext != "" {
			mt := mediatype.OfExtension(ext[1:]) // Remove leading "."
			if mt != nil {
				link.MediaType = mt
			}
		}
		links = append(links, link)
	}
	return links, nil
}

// Get implements Fetcher
func (f *ArchiveFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	// use decoded path for archive entry lookup to support files with spaces and special characters
	var entryPath string
	if hrefURL := link.Href.Resolve(nil, nil); hrefURL != nil {
		entryPath = hrefURL.Path()
	} else {
		entryPath = link.Href.String()
	}
	entry, err := f.archive.Entry(entryPath)
	if err != nil {
		return NewFailureResource(link, NotFound(err))
	}

	// Compute archive properties
	cl := entry.CompressedLength()
	if cl == 0 {
		cl = entry.Length()
	}

	er := &entryResource{
		link:  link,
		entry: entry,
		properties: manifest.Properties{
			"https://readium.org/webpub-manifest/properties#archive": map[string]interface{}{
				"entryLength":       cl,
				"isEntryCompressed": entry.CompressedLength() > 0,
			},
		},
	}

	return er
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

func NewArchiveFetcherFromPath(ctx context.Context, path string) (*ArchiveFetcher, error) {
	return NewArchiveFetcherFromPathWithFactory(ctx, path, archive.NewArchiveFactory())
}

func NewArchiveFetcherFromPathWithFactory(ctx context.Context, path string, factory archive.ArchiveFactory) (*ArchiveFetcher, error) {
	pth, err := url.FromFilepath(path)
	if err != nil {
		return nil, err
	}

	a, err := factory.Open(ctx, pth, "")
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		archive: a,
	}, nil
}

func NewArchiveFetcherFromURLWithFactory(ctx context.Context, url url.URL, factory archive.ArchiveFactory) (*ArchiveFetcher, error) {
	a, err := factory.Open(ctx, url, "")
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		archive: a,
	}, nil
}

func NewArchiveFetcherFromURLWithFactoryAndContext(ctx context.Context, url url.URL, factory archive.SchemeSpecificArchiveFactory) (*ArchiveFetcher, error) {
	var a archive.Archive
	var err error
	if f, ok := factory.(archive.ArchiveFactory); ok {
		a, err = f.Open(ctx, url, "")
	} else {
		return nil, errors.New("factory does not implement ArchiveFactory")
	}
	if err != nil {
		return nil, err
	}
	return &ArchiveFetcher{
		archive: a,
	}, nil
}

// Resource from archive entry
type entryResource struct {
	link       manifest.Link
	entry      archive.Entry
	properties manifest.Properties
}

// File implements Resource
func (r *entryResource) File() string {
	return ""
}

// Close implements Resource
func (r *entryResource) Close() {
	// Nothing needs to be done at the moment
}

// Link implements Resource
func (r *entryResource) Link() manifest.Link {
	return r.link
}

// Properties implements Resource
func (r *entryResource) Properties() manifest.Properties {
	return r.properties
}

// Read implements Resource
func (r *entryResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
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
func (r *entryResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
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

// CompressedAs implements CompressedResource
func (r *entryResource) CompressedAs(compressionMethod archive.CompressionMethod) bool {
	return r.entry.CompressedAs(compressionMethod)
}

// CompressedLength implements CompressedResource
func (r *entryResource) CompressedLength(ctx context.Context) int64 {
	return int64(r.entry.CompressedLength())
}

// CRC32Checksum implements CompressedResource
func (r *entryResource) CRC32Checksum(ctx context.Context) *uint32 {
	return r.entry.CRC32Checksum()
}

// StreamCompressed implements CompressedResource
func (r *entryResource) StreamCompressed(ctx context.Context, w io.Writer) (int64, *ResourceError) {
	i, err := r.entry.StreamCompressed(w)
	if err == nil {
		return i, nil
	}
	return -1, Other(err)
}

// StreamCompressedGzip implements CompressedResource
func (r *entryResource) StreamCompressedGzip(ctx context.Context, w io.Writer) (int64, *ResourceError) {
	i, err := r.entry.StreamCompressedGzip(w)
	if err == nil {
		return i, nil
	}
	return -1, Other(err)
}

// ReadCompressed implements CompressedResource
func (r *entryResource) ReadCompressed(ctx context.Context) ([]byte, *ResourceError) {
	i, err := r.entry.ReadCompressed()
	if err == nil {
		return i, nil
	}
	return nil, Other(err)
}

// ReadCompressedGzip implements CompressedResource
func (r *entryResource) ReadCompressedGzip(ctx context.Context) ([]byte, *ResourceError) {
	i, err := r.entry.ReadCompressedGzip()
	if err == nil {
		return i, nil
	}
	return nil, Other(err)
}

// Length implements Resource
func (r *entryResource) Length(ctx context.Context) (int64, *ResourceError) {
	return int64(r.entry.Length()), nil
}
