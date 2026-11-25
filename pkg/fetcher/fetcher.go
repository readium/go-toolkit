package fetcher

import (
	"context"
	"errors"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// localPath returns the decoded path a link points to, relative to the root
// of a fetcher of local resources, such as files, archive entries or cloud
// storage objects, and false for a URL with a scheme or an authority. Such a
// URL points elsewhere, whatever its path: "https://example.com/secret.txt"
// isn't the local secret.txt. Templates are expanded without parameters.
func localPath(link manifest.Link) (string, bool) {
	u := link.Href.Resolve(nil, nil)
	if u == nil {
		return link.Href.String(), true
	}
	if url.IsRemoteReference(u) {
		return "", false
	}
	return u.Path(), true
}

// errNotLocal is the error of a link to a URL with a scheme or an authority,
// given to a fetcher of local resources: see localPath.
var errNotLocal = errors.New("not a local resource: the URL has a scheme or an authority")

// Exhaustive is implemented by fetchers which can report whether their Links
// list every resource they serve, recursively, such as the entries of an
// archive, or the files of a directory or of a cloud storage prefix. Others,
// such as fetchers of remote HTTP folders, which can't be listed, don't
// implement it. Fetchers wrapping others, which may serve other resources,
// don't implement it either.
type Exhaustive interface {
	Exhaustive() bool
}

// Fetcher provides access to a Resource from a Link.
type Fetcher interface {

	/**
	 * Known resources available in the medium, such as file paths on the file system
	 * or entries in a ZIP archive. This list is not exhaustive, and additional
	 * unknown resources might be reachable.
	 *
	 * If the medium has an inherent resource order, it should be followed.
	 * Otherwise, HREFs are sorted alphabetically.
	 */
	Links(ctx context.Context) (manifest.LinkList, error)

	/**
	 * Returns the [Resource] at the given [link]'s HREF.
	 *
	 * A [Resource] is always returned, since for some cases we can't know if it exists before
	 * actually fetching it, such as HTTP. Therefore, errors are handled at the Resource level.
	 */
	Get(ctx context.Context, link manifest.Link) Resource

	// Closes this object and releases any resources associated with it.
	// If the object is already closed then invoking this method has no effect.
	Close()
}

// A [Fetcher] providing no resources at all.
type EmptyFetcher struct{}

func (f EmptyFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	return manifest.LinkList{}, nil
}

func (f EmptyFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	return NewFailureResource(link, NotFound(nil))
}

func (f EmptyFetcher) Close() {}
