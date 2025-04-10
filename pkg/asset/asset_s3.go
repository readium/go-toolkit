package asset

import (
	"context"
	"errors"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Represents a publication stored on an Amazon S3-compatible remote server.
type S3Asset struct {
	uri    url.AbsoluteURL
	client *s3.Client

	mediatype      *mediatype.MediaType
	knownMediaType *mediatype.MediaType

	isDir    *bool
	headData *s3.HeadObjectOutput
}

func S3(client *s3.Client, uri url.AbsoluteURL) *S3Asset {
	return &S3Asset{
		client: client,
		uri:    uri,
	}
}

// Creates a [S3Asset] from a [File] and an optional media type, when known.
func S3WithMediaType(client *s3.Client, uri url.AbsoluteURL, mediatype *mediatype.MediaType) *S3Asset {
	return &S3Asset{
		client:         client,
		uri:            uri,
		knownMediaType: mediatype,
	}
}

// Name implements PublicationAsset
func (a *S3Asset) Name() string {
	return path.Base(a.uri.Path())
}

func (a *S3Asset) object() (*s3.GetObjectInput, error) {
	return a.uri.ToS3Object()
}

func (a *S3Asset) head(ctx context.Context) error {
	if a.headData != nil {
		return nil
	}
	obj, err := a.object()
	if err != nil {
		return err
	}
	output, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: obj.Bucket,
		Key:    obj.Key,
	})
	if err != nil {
		return err
	}
	a.headData = output
	return nil
}

// MediaType implements PublicationAsset
func (a *S3Asset) MediaType(ctx context.Context) mediatype.MediaType {
	if a.mediatype == nil {
		if a.knownMediaType != nil {
			a.mediatype = a.knownMediaType
		} else {
			if err := a.head(ctx); err == nil {
				// Note how we are *not* using the file contents to sniff the media type.
				// We want to avoid unecessary requests at all costs.
				if a.headData.ContentType != nil {
					a.mediatype = mediatype.OfStringAndExtension(*a.headData.ContentType, a.uri.Extension())
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
func (a *S3Asset) CreateFetcher(ctx context.Context, dependencies Dependencies, credentials string) (fetcher.Fetcher, error) {
	obj, err := a.object()
	if err != nil {
		return nil, err
	}

	var isDir bool
	if a.isDir != nil {
		isDir = *a.isDir
	} else {
		if strings.HasSuffix(*obj.Key, "/") {
			// Path ends in a slash, so it's a folder
			isDir = true
		} else {
			// Not sure if it's a folder or a file, need to check
			prefix := *obj.Key + "/"
			max := int32(1)
			out, err := a.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
				Bucket:  obj.Bucket,
				Prefix:  &prefix,
				MaxKeys: &max,
			})
			if err != nil {
				return nil, err
			}
			if len(out.Contents) > 0 {
				isDir = true
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
		return fetcher.NewS3Fetcher(base, a.client, *obj.Bucket, *obj.Key), nil
	} else {
		factory, ok := dependencies.ArchiveFactory.(archive.SchemeSpecificArchiveFactory)
		if !ok {
			// It's not possible to determine if the factory actually supports archives on S3
			return nil, errors.New("provided ArchiveFactory does not implement SchemeSpecificArchiveFactory")
		}
		if !factory.CanOpen(url.SchemeS3) {
			return nil, errors.New("provided ArchiveFactory does not support S3 scheme")
		}

		return fetcher.NewArchiveFetcherFromURLWithFactoryAndContext(ctx, a.uri, factory)
	}

}
