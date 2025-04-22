package asset

import (
	"context"
	"errors"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"google.golang.org/api/iterator"
)

// Represents a publication stored on an Amazon S3-compatible remote server.
type GCSAsset struct {
	uri    url.AbsoluteURL
	client *storage.Client

	mediatype      *mediatype.MediaType
	knownMediaType *mediatype.MediaType

	isDir *bool
	attrs *storage.ObjectAttrs
}

func GCS(client *storage.Client, uri url.AbsoluteURL) *GCSAsset {
	return &GCSAsset{
		client: client,
		uri:    uri,
	}
}

// Creates a [S3Asset] from a [File] and an optional media type, when known.
func GCSWithMediaType(client *storage.Client, uri url.AbsoluteURL, mediatype *mediatype.MediaType) *GCSAsset {
	return &GCSAsset{
		client:         client,
		uri:            uri,
		knownMediaType: mediatype,
	}
}

// Name implements PublicationAsset
func (a *GCSAsset) Name() string {
	return path.Base(a.uri.Path())
}

func (a *GCSAsset) handle() (*storage.ObjectHandle, error) {
	return a.uri.ToGSObject(a.client)
}

func (a *GCSAsset) head(ctx context.Context) error {
	if a.attrs != nil {
		return nil
	}
	handle, err := a.handle()
	if err != nil {
		return err
	}
	a.attrs, err = handle.Attrs(ctx)
	return err
}

// MediaType implements PublicationAsset
func (a *GCSAsset) MediaType(ctx context.Context) mediatype.MediaType {
	if a.mediatype == nil {
		if a.knownMediaType != nil {
			a.mediatype = a.knownMediaType
		} else {
			if err := a.head(ctx); err == nil {
				// Note how we are *not* using the file contents to sniff the media type.
				// We want to avoid unecessary requests at all costs.
				if a.attrs.ContentType != "" {
					a.mediatype = mediatype.OfStringAndExtension(a.attrs.ContentType, a.uri.Extension())
				} else {
					a.mediatype = mediatype.OfExtension(a.uri.Extension())
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
func (a *GCSAsset) CreateFetcher(ctx context.Context, dependencies Dependencies, credentials string) (fetcher.Fetcher, error) {
	handle, err := a.handle()
	if err != nil {
		return nil, err
	}

	var isDir bool
	if a.isDir != nil {
		isDir = *a.isDir
	} else {
		if strings.HasSuffix(handle.ObjectName(), "/") {
			// Path ends in a slash, so it's a folder
			isDir = true
		} else {
			// Not sure if it's a folder or a file, need to check
			it := a.client.Bucket(handle.BucketName()).Objects(ctx, &storage.Query{
				Prefix:    handle.ObjectName() + "/",
				Delimiter: "/",
			})
			_, err := it.Next()
			if err == nil {
				// Found a file with the same prefix, so it's a folder
				isDir = true
			} else if err != iterator.Done {
				// Something else than EOF
				return nil, err
			}
		}
		a.isDir = &isDir
	}

	if isDir || !a.MediaType(ctx).IsZIP() {
		base := ""
		if !isDir {
			// There's some problem checking for the file's existance
			if err = a.head(ctx); err != nil {
				return nil, err
			}

			base = a.Name()
		}
		return fetcher.NewGCSFetcher(base, a.client, handle), nil
	} else {
		factory, ok := dependencies.ArchiveFactory.(archive.SchemeSpecificArchiveFactory)
		if !ok {
			// It's not possible to determine if the factory actually supports archives on GCS
			return nil, errors.New("provided ArchiveFactory does not implement SchemeSpecificArchiveFactory")
		}
		if !factory.CanOpen(url.SchemeGS) {
			return nil, errors.New("provided ArchiveFactory does not support GS scheme")
		}

		return fetcher.NewArchiveFetcherFromURLWithFactoryAndContext(ctx, a.uri, factory)
	}
}
