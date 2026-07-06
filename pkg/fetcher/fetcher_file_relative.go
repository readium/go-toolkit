package fetcher

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"strings"
	"weak"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Provides access to resources on the local file system with HREFs resolved against a
// base file URL the way a web server would: a HREF `a/file.xhtml` points next to the
// base file, and `../covers/image.jpg` inside the base's parent directory.
//
// Unlike [FileFetcher], resolution is not sandboxed to a single directory: `..` segments
// can reach anywhere on the file system. Only use this fetcher for trusted publications,
// e.g. a Readium Web Publication Manifest whose resources live in sibling directories.
type RelativeFileFetcher struct {
	base      url.AbsoluteURL
	resources []weak.Pointer[FileResource]
}

func NewRelativeFileFetcher(base url.AbsoluteURL) (*RelativeFileFetcher, error) {
	if !base.IsFile() {
		return nil, errors.New("RelativeFileFetcher requires a file:// base URL")
	}
	return &RelativeFileFetcher{base: base}, nil
}

// Links implements Fetcher
// Lists the files in the base's directory. Resources only reachable through parent
// (`..`) HREFs are not included; as documented on [Fetcher], the list is not exhaustive.
func (f *RelativeFileFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	dir := f.base.Path()
	if !strings.HasSuffix(dir, "/") {
		dir = path.Dir(dir)
	}
	ff := &FileFetcher{paths: map[string]string{"": filepath.FromSlash(dir)}}
	return ff.Links(ctx)
}

// Get implements Fetcher
func (f *RelativeFileFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	u := link.URL(nil, nil)
	if u == nil {
		return NewFailureResource(link, NotFound(errors.New("link has no URL")))
	}
	resolved, ok := f.base.Resolve(u).(url.AbsoluteURL)
	if !ok || !resolved.IsFile() {
		return NewFailureResource(link, NotFound(errors.New("couldn't resolve "+u.String()+" to a local file")))
	}
	resource := NewFileResource(link, resolved.ToFilepath())
	f.resources = append(f.resources, weak.Make(resource))
	return resource
}

// Close implements Fetcher
func (f *RelativeFileFetcher) Close() {
	// Safety mechanism to cleanup any os.File handles still open
	for _, res := range f.resources {
		if r := res.Value(); r != nil {
			r.Close()
		}
	}
	f.resources = nil
}
