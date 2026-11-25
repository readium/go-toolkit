//go:build !readium_nocloud

package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/readium/go-toolkit/pkg/internal/byterange"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type S3Fetcher struct {
	href   string
	client *s3.Client
	bucket string
	key    string

	linksMu     sync.Mutex // guards cachedLinks
	cachedLinks manifest.LinkList

	// heads shares object metadata (key -> *s3.HeadObjectOutput) across the
	// resources created by Get, so serving many requests for the same object —
	// every one of which needs the length — performs a single HeadObject call
	// over the fetcher's lifetime instead of one per request.
	heads sync.Map
}

func NewS3Fetcher(href string, client *s3.Client, bucket, key string) *S3Fetcher {
	if client == nil {
		panic("S3Fetcher requires a non-nil client")
	}
	return &S3Fetcher{
		href:   href,
		client: client,
		bucket: bucket,
		key:    key,
	}
}

// Exhaustive implements [Exhaustive]: the links are every object under the
// key prefix.
func (f *S3Fetcher) Exhaustive() bool { return true }

// Links implements Fetcher
//
// Every object under the key prefix is listed, through every page of the
// listing, up to maxListedObjects. Callers get their own copy of the cached
// listing.
func (f *S3Fetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	f.linksMu.Lock()
	defer f.linksMu.Unlock()
	if len(f.cachedLinks) > 0 {
		return slices.Clone(f.cachedLinks), nil
	}

	prefix := listingPrefix(f.key)

	// List all items in the "folder", which may take several pages.
	var links manifest.LinkList
	listed := 0
	pages := s3.NewListObjectsV2Paginator(f.client, &s3.ListObjectsV2Input{
		Bucket: &f.bucket,
		Prefix: &prefix,
	})
	for pages.HasMorePages() {
		out, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range out.Contents {
			if listed++; listed > maxListedObjects {
				return nil, fmt.Errorf("more than %d objects under %s", maxListedObjects, prefix)
			}
			if v.Key == nil {
				continue
			}
			rel, ok, err := objectPath(*v.Key, prefix)
			if err != nil {
				return nil, err
			} else if !ok {
				continue
			}
			href, err := fileHREF(path.Join(f.href, rel))
			if err != nil {
				return nil, err
			}
			link := manifest.Link{Href: href, MediaType: &mediatype.Binary}
			if ext := path.Ext(*v.Key); ext != "" {
				if mt := mediatype.OfExtension(ext[1:]); mt != nil {
					link.MediaType = mt
				}
			}
			links = append(links, link)
		}
	}

	if listed > 0 {
		f.cachedLinks = links
		return slices.Clone(f.cachedLinks), nil
	}

	// Empty directory
	if f.key == "" || strings.HasSuffix(f.key, "/") {
		return nil, nil
	}

	ext := path.Ext(f.key)
	if ext != "" {
		ext = ext[1:]
	}
	mt := mediatype.OfExtension(ext)
	if mt == nil {
		mt = &mediatype.Binary
	}

	// Not a directory, just a single file
	f.cachedLinks = manifest.LinkList{{
		Href:      manifest.NewHREF(url.MustURLFromString(f.href)),
		MediaType: mt,
	}}
	return slices.Clone(f.cachedLinks), nil
}

// Get implements Fetcher
func (f *S3Fetcher) Get(ctx context.Context, link manifest.Link) Resource {
	// Use the decoded path for the object lookup: S3 keys are raw strings, so a
	// percent-encoded HREF would never match, and queries/fragments don't belong in keys.
	linkHref, ok := localPath(link)
	if !ok {
		return NewFailureResource(link, NotFound(errNotLocal))
	}
	if after, ok := strings.CutPrefix(linkHref, f.href); ok {
		resourceFile := path.Join(f.key, after)
		// Keep the resource within the fetcher's key prefix: a `..` in the HREF must
		// not let an incoming request reach objects elsewhere in the bucket.
		if keyWithinRoot(f.key, resourceFile) {
			return &s3Resource{
				link:   link,
				client: f.client,
				bucket: f.bucket,
				key:    resourceFile,
				heads:  &f.heads,
			}
		}
	}

	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in S3Fetcher paths")))
}

func (f *S3Fetcher) Close() {
	// No-op for S3
}

// Resource from S3
type s3Resource struct {
	link   manifest.Link
	client *s3.Client
	bucket string
	key    string

	cachedHead *s3.HeadObjectOutput
	heads      *sync.Map // Optional fetcher-shared metadata cache, see S3Fetcher
}

// Link implements Resource
func (r *s3Resource) Link() manifest.Link {
	return r.link
}

// Properties implements Resource
func (r *s3Resource) Properties() manifest.Properties {
	return manifest.Properties{}
}

// Close implements Resource
func (r *s3Resource) Close() {
	// No-op for S3
}

// File implements Resource
func (r *s3Resource) File() string {
	return ""
}

func (r *s3Resource) object() *s3.GetObjectInput {
	return &s3.GetObjectInput{
		Bucket: &r.bucket,
		Key:    &r.key,
	}
}

func (r *s3Resource) head(ctx context.Context) (*s3.HeadObjectOutput, *ResourceError) {
	if r.cachedHead == nil && r.heads != nil {
		if v, ok := r.heads.Load(r.key); ok {
			r.cachedHead = v.(*s3.HeadObjectOutput)
		}
	}
	if r.cachedHead == nil {
		head, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: &r.bucket,
			Key:    &r.key,
		})
		if err != nil {
			return nil, awsErrorToException(err)
		}
		r.cachedHead = head
		if r.heads != nil {
			r.heads.Store(r.key, head)
		}
	}
	return r.cachedHead, nil
}

// Read implements Resource
func (r *s3Resource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	obj := r.object()
	if start != 0 || end != 0 {
		var sb strings.Builder
		sb.WriteString("bytes=")
		sb.WriteString(strconv.FormatInt(start, 10))
		sb.WriteString("-")
		if end > 0 {
			sb.WriteString(strconv.FormatInt(end, 10))
		}
		obj.Range = aws.String(sb.String())
	}

	output, err := r.client.GetObject(ctx, obj)
	if err != nil {
		return nil, awsErrorToException(err)
	}
	defer output.Body.Close()

	var data []byte
	if output.ContentLength != nil && *output.ContentLength >= 0 {
		data = make([]byte, *output.ContentLength)
		_, err = io.ReadFull(output.Body, data)
	} else {
		data, err = io.ReadAll(output.Body)
	}
	if err != nil {
		return nil, Other(err)
	}
	return data, nil
}

// Stream implements Resource
func (r *s3Resource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return -1, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	obj := r.object()
	if start != 0 || end != 0 {
		var sb strings.Builder
		sb.WriteString("bytes=")
		sb.WriteString(strconv.FormatInt(start, 10))
		sb.WriteString("-")
		if end > 0 {
			sb.WriteString(strconv.FormatInt(end, 10))
		}
		obj.Range = aws.String(sb.String())
	}

	output, err := r.client.GetObject(ctx, obj)
	if err != nil {
		return -1, awsErrorToException(err)
	}
	defer output.Body.Close()

	n, err := io.Copy(w, output.Body)
	if err != nil {
		return -1, Other(err)
	}
	return n, nil
}

// HasEfficientStream implements EfficientStreamer. Stream performs a single
// ranged GetObject request and pipes the response body through as it arrives.
func (r *s3Resource) HasEfficientStream() bool {
	return true
}

// Length implements Resource
func (r *s3Resource) Length(ctx context.Context) (int64, *ResourceError) {
	head, rerr := r.head(ctx)
	if rerr != nil {
		return 0, rerr
	}
	if head.ContentLength == nil {
		return 0, Other(errors.New("object does not have length"))
	}
	return *head.ContentLength, nil
}

func awsErrorToException(err error) *ResourceError {
	var notFound *types.NotFound
	var noSuchKey *types.NoSuchKey
	var noSuchBucket *types.NoSuchBucket
	var invalidObjectState *types.InvalidObjectState
	if errors.As(err, &notFound) || errors.As(err, &noSuchKey) || errors.As(err, &noSuchBucket) {
		return NotFound(err)
	} else if errors.As(err, &invalidObjectState) {
		return BadRequest(err)
	} else {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "AccessDenied" {
				return Forbidden(err)
			}
		}
	}

	return Other(err)
}
