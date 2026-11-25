//go:build !readium_nocloud

package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/readium/go-toolkit/pkg/internal/byterange"
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

	linksMu     sync.Mutex // guards cachedLinks
	cachedLinks manifest.LinkList

	// attrsCache shares object metadata (object name -> *storage.ObjectAttrs)
	// across the resources created by Get, so serving many requests for the
	// same object — every one of which needs the length — performs a single
	// attributes call over the fetcher's lifetime instead of one per request.
	attrsCache sync.Map
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

// Exhaustive implements [Exhaustive]: the links are every object under the
// name prefix.
func (f *GCSFetcher) Exhaustive() bool { return true }

// Links implements Fetcher
//
// Every object under the name prefix is listed, recursively rather than
// folder by folder, up to maxListedObjects. Callers get their own copy of the
// cached listing.
func (f *GCSFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	f.linksMu.Lock()
	defer f.linksMu.Unlock()
	if len(f.cachedLinks) > 0 {
		return slices.Clone(f.cachedLinks), nil
	}

	prefix := listingPrefix(f.handle.ObjectName())

	// List all items in the "folder", including those of its subfolders:
	// without a delimiter, the listing isn't split into prefixes.
	it := f.client.Bucket(f.handle.BucketName()).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})
	it.PageInfo().MaxSize = 1000 // per page
	var links manifest.LinkList
	listed := 0
	for {
		item, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		if listed++; listed > maxListedObjects {
			return nil, fmt.Errorf("more than %d objects under %s", maxListedObjects, prefix)
		}
		if item.Prefix != "" {
			continue
		}
		rel, ok, err := objectPath(item.Name, prefix)
		if err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		href, err := fileHREF(path.Join(f.href, rel))
		if err != nil {
			return nil, err
		}
		link := manifest.Link{Href: href}
		if ext := path.Ext(item.Name); ext != "" {
			if mt := mediatype.OfExtension(ext[1:]); mt != nil {
				link.MediaType = mt
			}
		}
		links = append(links, link)
	}

	if listed > 0 {
		f.cachedLinks = links
		return slices.Clone(f.cachedLinks), nil
	}

	// Empty directory
	if f.handle.ObjectName() == "" || strings.HasSuffix(f.handle.ObjectName(), "/") {
		return nil, nil
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
	return slices.Clone(f.cachedLinks), nil
}

// Get implements Fetcher
func (f *GCSFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	// Use the decoded path for the object lookup: GCS object names are raw strings, so a
	// percent-encoded HREF would never match, and queries/fragments don't belong in names.
	linkHref, ok := localPath(link)
	if !ok {
		return NewFailureResource(link, NotFound(errNotLocal))
	}
	if after, ok := strings.CutPrefix(linkHref, f.href); ok {
		resourceFile := path.Join(f.handle.ObjectName(), after)
		// Keep the resource within the fetcher's object-name prefix: a `..` in the HREF
		// must not let an incoming request reach objects elsewhere in the bucket.
		if keyWithinRoot(f.handle.ObjectName(), resourceFile) {
			return &gcsResource{
				handle:     f.client.Bucket(f.handle.BucketName()).Object(resourceFile),
				link:       link,
				attrsCache: &f.attrsCache,
			}
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
	attrsCache  *sync.Map // Optional fetcher-shared metadata cache, see GCSFetcher
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
	if r.cachedAttrs == nil && r.attrsCache != nil {
		if v, ok := r.attrsCache.Load(r.handle.ObjectName()); ok {
			r.cachedAttrs = v.(*storage.ObjectAttrs)
		}
	}
	if r.cachedAttrs == nil {
		head, err := r.handle.Attrs(ctx)
		if err != nil {
			return nil, gcsErrorToException(err)
		}
		r.cachedAttrs = head
		if r.attrsCache != nil {
			r.attrsCache.Store(r.handle.ObjectName(), head)
		}
	}
	return r.cachedAttrs, nil
}

// Read implements Resource
func (r *gcsResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	var rdr *storage.Reader
	var err error
	if start == 0 && end == 0 {
		rdr, err = r.handle.NewReader(ctx)
	} else {
		rdr, err = r.handle.NewRangeReader(ctx, start, byterange.Length(start, end))
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
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return -1, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	var rdr *storage.Reader
	var err error
	if start == 0 && end == 0 {
		rdr, err = r.handle.NewReader(ctx)
	} else {
		rdr, err = r.handle.NewRangeReader(ctx, start, byterange.Length(start, end))
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

// HasEfficientStream implements EfficientStreamer. Stream performs a single
// ranged object read and pipes the data through as it arrives.
func (r *gcsResource) HasEfficientStream() bool {
	return true
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
