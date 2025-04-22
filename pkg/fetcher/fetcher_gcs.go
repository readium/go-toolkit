package fetcher

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

type GCSFetcher struct {
	href   string
	client *storage.Client
	handle *storage.ObjectHandle

	cachedLinks manifest.LinkList
}

func NewGCSFetcher(href string, client *storage.Client, handle *storage.ObjectHandle) *GCSFetcher {
	if client == nil || handle == nil {
		panic("GCSFetcher requires a non-nil client and handle")
	}
	return &GCSFetcher{
		client: client,
		href:   href,
		handle: handle,
	}
}

// Links implements Fetcher
func (f *GCSFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	if len(f.cachedLinks) > 0 {
		return f.cachedLinks, nil
	}

	prefix := f.handle.ObjectName()
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// List all items in the "folder"
	it := f.client.Bucket(f.handle.BucketName()).Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	})
	it.PageInfo().MaxSize = 1000 // Should be enough. We can see about increasing this based on implementer feedback.
	itemAttrs, err := it.Next()
	if err == nil {
		f.cachedLinks = make(manifest.LinkList, 0, it.PageInfo().Remaining()+1)
		processItem := func(item *storage.ObjectAttrs) error {
			if item.Size == 0 {
				return nil
			}

			href, err := manifest.NewHREFFromString(path.Join(f.href, strings.TrimPrefix(item.Name, prefix)), false)
			if err != nil {
				return err
			}
			link := manifest.Link{
				Href: href,
			}

			ext := path.Ext(item.Name)
			if ext != "" {
				mt := mediatype.OfExtension(ext[1:])
				if mt != nil {
					link.MediaType = mt
				}
			}
			f.cachedLinks = append(f.cachedLinks, link)
			return nil
		}
		if err := processItem(itemAttrs); err != nil {
			return nil, err
		}
		for {
			itemAttrs, err = it.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return nil, err
			}
			if err := processItem(itemAttrs); err != nil {
				return nil, err
			}
		}
	} else if err == iterator.Done {
		// Empty directory
		if strings.HasSuffix(f.handle.ObjectName(), "/") {
			return f.cachedLinks, nil
		}

		ext := path.Ext(f.handle.ObjectName())
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
	} else {
		// Something else than EOF
		return nil, err
	}

	return f.cachedLinks, nil
}

// Get implements Fetcher
func (f *GCSFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	linkHref := link.Href.String()
	if strings.HasPrefix(linkHref, f.href) {
		resourceFile := path.Join(f.handle.ObjectName(), strings.TrimPrefix(linkHref, f.href))
		return &gcsResource{
			handle: f.client.Bucket(f.handle.BucketName()).Object(resourceFile),
			link:   link,
		}
	}

	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in GCSFetcher paths")))
}

func (f *GCSFetcher) Close() {
	// No-op for GCS
}

// Resource from GCS
type gcsResource struct {
	link        manifest.Link
	handle      *storage.ObjectHandle
	cachedAttrs *storage.ObjectAttrs
}

// Link implements Resource
func (r *gcsResource) Link() manifest.Link {
	return r.link
}

// Properties implements Resource
func (r *gcsResource) Properties() manifest.Properties {
	return manifest.Properties{}
}

// Close implements Resource
func (r *gcsResource) Close() {
	// No-op for GCS
}

// File implements Resource
func (r *gcsResource) File() string {
	return ""
}

func (r *gcsResource) attrs(ctx context.Context) (*storage.ObjectAttrs, *ResourceError) {
	if r.cachedAttrs == nil {
		head, err := r.handle.Attrs(ctx)
		if err != nil {
			return nil, gcsErrorToException(err)
		}
		r.cachedAttrs = head
	}
	return r.cachedAttrs, nil
}

// Read implements Resource
func (r *gcsResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	if end < start {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	var rdr *storage.Reader
	var err error
	if start == 0 && end == 0 {
		rdr, err = r.handle.NewReader(ctx)
	} else {
		rdr, err = r.handle.NewRangeReader(ctx, start, end-start+1)
	}
	if err != nil {
		return nil, gcsErrorToException(err)
	}
	defer rdr.Close()

	var data []byte
	if rdr.Remain() >= 0 {
		data = make([]byte, rdr.Remain())
		_, err = io.ReadFull(rdr, data)
	} else {
		data, err = io.ReadAll(rdr)
	}
	if err != nil {
		return nil, Other(err)
	}
	return data, nil
}

// Stream implements Resource
func (r *gcsResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	if end < start {
		return -1, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	var rdr *storage.Reader
	var err error
	if start == 0 && end == 0 {
		rdr, err = r.handle.NewReader(ctx)
	} else {
		rdr, err = r.handle.NewRangeReader(ctx, start, end-start+1)
	}
	if err != nil {
		return -1, gcsErrorToException(err)
	}
	defer rdr.Close()

	n, err := io.Copy(w, rdr)
	if err != nil {
		return -1, Other(err)
	}
	return n, nil
}

// Length implements Resource
func (r *gcsResource) Length(ctx context.Context) (int64, *ResourceError) {
	attrs, rerr := r.attrs(ctx)
	if rerr != nil {
		return 0, rerr
	}
	return attrs.Size, nil
}

func gcsErrorToException(err error) *ResourceError {
	if gErr, ok := err.(*googleapi.Error); ok {
		switch gErr.Code {
		case http.StatusNotFound:
			return NotFound(err)
		case http.StatusForbidden:
			return Forbidden(err)
		case http.StatusBadRequest:
			return BadRequest(err)
		}
	}

	return Other(err)
}
