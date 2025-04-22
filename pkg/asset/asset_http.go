package asset

import (
	"context"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Represents a publication stored on an Amazon S3-compatible remote server.
type HTTPAsset struct {
	url    url.AbsoluteURL
	client *http.Client

	mediatype      *mediatype.MediaType
	knownMediaType *mediatype.MediaType

	fileSize    int64
	contentType string
}

func HTTP(client *http.Client, url url.AbsoluteURL) *HTTPAsset {
	return &HTTPAsset{
		client: client,
		url:    url,
	}
}

// Creates a [HTTPAsset] from a [File] and an optional media type, when known.
func HTTPWithMediaType(client *http.Client, url url.AbsoluteURL, mediatype *mediatype.MediaType) *HTTPAsset {
	return &HTTPAsset{
		client:         client,
		url:            url,
		knownMediaType: mediatype,
	}
}

// Name implements PublicationAsset
func (a *HTTPAsset) Name() string {
	return path.Base(a.url.Path())
}

func (a *HTTPAsset) head(ctx context.Context) error {
	if a.fileSize > 0 {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, a.url.String(), nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// If it's not code 200, the file doesn't exist
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("HTTP HEAD request failed with status code: %d", resp.StatusCode)
	}

	// HTTP server *must* support byte range requests
	arvs := resp.Header.Values("Accept-Ranges")
	if !slices.Contains(arvs, "bytes") {
		return errors.New("HTTP server does not support byte range requests")
	}

	// HTTP server *must* return Content-Length header
	if resp.ContentLength <= 0 {
		return errors.New("HTTP server returned zero content length")
	}
	a.fileSize = resp.ContentLength

	// A good server will response with the correct content type for the file
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" {
		a.contentType = contentType
	}

	return nil
}

// MediaType implements PublicationAsset
func (a *HTTPAsset) MediaType(ctx context.Context) mediatype.MediaType {
	if a.mediatype == nil {
		if a.knownMediaType != nil {
			a.mediatype = a.knownMediaType
		} else {
			if err := a.head(ctx); err == nil {
				// Note how we are *not* using the file contents to sniff the media type.
				// We want to avoid unecessary requests at all costs.
				if a.contentType != "" {
					a.mediatype = mediatype.OfStringAndExtension(a.contentType, a.url.Extension())
				} else {
					a.mediatype = mediatype.OfExtension(a.url.Extension())
				}
			}
		}
		if a.mediatype == nil { // Still nothing found
			a.mediatype = &mediatype.Binary
		}
	}
	return *a.mediatype
}

// CreateFetcher implements PublicationAsset
func (a *HTTPAsset) CreateFetcher(ctx context.Context, dependencies Dependencies, credentials string) (fetcher.Fetcher, error) {
	// We can't determine if the provided path is a directory or not unless it ends in a "/"
	// because we can't expect HTTP servers to be listing directory indexes, and even then we
	// couldn't distinguish between a directory listing and a file. So no "/" is always a file.
	isDir := strings.HasSuffix(a.url.Path(), "/")

	if isDir || !a.MediaType(ctx).IsZIP() {
		base := ""
		if !isDir {
			// There's some problem checking for the file's existance
			if err := a.head(ctx); err != nil {
				return nil, err
			}

			base = a.Name()
		}
		return fetcher.NewHTTPFetcher(base, a.client, a.url), nil
	} else {
		factory, ok := dependencies.ArchiveFactory.(archive.SchemeSpecificArchiveFactory)
		if !ok {
			// It's not possible to determine if the factory actually supports archives through HTTP
			return nil, errors.New("provided ArchiveFactory does not implement SchemeSpecificArchiveFactory")
		}
		if !factory.CanOpen(url.SchemeHTTP) && !factory.CanOpen(url.SchemeHTTPS) {
			return nil, errors.New("provided ArchiveFactory does not support HTTP or HTTPS scheme")
		}

		return fetcher.NewArchiveFetcherFromURLWithFactoryAndContext(ctx, a.url, factory)
	}
}
