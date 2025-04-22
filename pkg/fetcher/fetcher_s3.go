package fetcher

import (
	"context"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type S3Fetcher struct {
	href   string
	client *s3.Client
	bucket string
	key    string

	cachedLinks manifest.LinkList
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

// Links implements Fetcher
func (f *S3Fetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	if len(f.cachedLinks) > 0 {
		return f.cachedLinks, nil
	}

	prefix := f.key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// List all items in the "folder"
	out, err := f.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: &f.bucket,
		Prefix: &prefix,
		// MaxKeys is omitted, can list up to 1000 files by default. Should be enough
		// and serve as a sanity check. We can see about increasing this based on implementer feedback.
	})
	if err != nil {
		return nil, err
	}
	if len(out.Contents) > 0 {
		f.cachedLinks = make(manifest.LinkList, len(out.Contents))
		for i, v := range out.Contents {
			if v.Size != nil && *v.Size == 0 {
				continue
			}

			href, err := manifest.NewHREFFromString(path.Join(f.href, strings.TrimPrefix(*v.Key, prefix)), false)
			if err != nil {
				return nil, err
			}
			f.cachedLinks[i].Href = href

			ext := path.Ext(*v.Key)
			f.cachedLinks[i].MediaType = &mediatype.Binary
			if ext != "" {
				mt := mediatype.OfExtension(ext[1:])
				if mt != nil {
					f.cachedLinks[i].MediaType = mt
				}
			}
		}
	} else {
		// Empty directory
		if strings.HasSuffix(f.key, "/") {
			return f.cachedLinks, nil
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
	}

	return f.cachedLinks, nil
}

// Get implements Fetcher
func (f *S3Fetcher) Get(ctx context.Context, link manifest.Link) Resource {
	linkHref := link.Href.String()
	if strings.HasPrefix(linkHref, f.href) {
		resourceFile := path.Join(f.key, strings.TrimPrefix(linkHref, f.href))
		return &s3Resource{
			link:   link,
			client: f.client,
			bucket: f.bucket,
			key:    resourceFile,
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
	if r.cachedHead == nil {
		head, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: &r.bucket,
			Key:    &r.key,
		})
		if err != nil {
			return nil, awsErrorToException(err)
		}
		r.cachedHead = head
	}
	return r.cachedHead, nil
}

// Read implements Resource
func (r *s3Resource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	if end < start {
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

	output, err := r.client.GetObject(ctx, r.object())
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
	if end < start {
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
