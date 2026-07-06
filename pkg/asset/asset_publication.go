package asset

import (
	"context"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type Dependencies struct {
	archive.ArchiveFactory
}

// Represents a digital medium (e.g. a file) offering access to a publication.
type PublicationAsset interface {
	Name() string                                                                                              // Name of the asset, e.g. a filename.
	MediaType(ctx context.Context) mediatype.MediaType                                                         // Media type of the asset. If unknown, fallback on `MediaType.Binary`.
	CreateFetcher(ctx context.Context, dependencies Dependencies, credentials string) (fetcher.Fetcher, error) // Creates a fetcher used to access the asset's content.
}

// A [PublicationAsset] which can also provide access to resources located relative to itself,
// e.g. the resources referenced by a bare (exploded) Readium Web Publication Manifest.
type RelativePublicationAsset interface {
	PublicationAsset

	// The URL of the asset within its medium, e.g. a file path URL, or an
	// HTTP(S), S3 or GS URL. HREFs relative to the asset resolve against it.
	Location() url.URL

	// Creates a fetcher serving the resources surrounding this asset, rooted at [root]:
	// a link with HREF `a/file.xhtml` is fetched from `a/file.xhtml` under the root
	// directory, whether the asset lives on a local file system or on a remote server.
	//
	// [root] must be the URL of a directory containing the asset, typically derived
	// from [Location] by resolving a `./` or `../` reference against it.
	CreateRelativeFetcher(ctx context.Context, root url.URL) (fetcher.Fetcher, error)
}
