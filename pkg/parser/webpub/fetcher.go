package webpub

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Serves the resources of a bare (exploded) Readium Web Publication Manifest:
// relative HREFs are resolved against the manifest's location by the [relative]
// fetcher, while absolute HTTP(S) HREFs are fetched with the [client], since a
// manifest is free to reference resources hosted anywhere.
type manifestResourceFetcher struct {
	relative fetcher.Fetcher
	client   *http.Client
}

// Links implements fetcher.Fetcher
func (f *manifestResourceFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	return f.relative.Links(ctx)
}

// Get implements fetcher.Fetcher
func (f *manifestResourceFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	if !link.Href.IsTemplated() {
		if u, ok := link.URL(nil, nil).(url.AbsoluteURL); ok {
			if u.IsHTTP() && f.client != nil {
				return fetcher.NewHTTPResource(link, f.client, u)
			}
			return fetcher.NewFailureResource(link, fetcher.NotFound(
				errors.New("cannot fetch absolute HREF "+u.String()),
			))
		}
	}
	return f.relative.Get(ctx, link)
}

// Close implements fetcher.Fetcher
func (f *manifestResourceFetcher) Close() {
	f.relative.Close()
}
