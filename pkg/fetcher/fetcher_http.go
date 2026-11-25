package fetcher

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/byterange"
	"github.com/readium/go-toolkit/pkg/internal/httprange"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type HTTPFetcher struct {
	href   string
	client *http.Client
	url    url.AbsoluteURL

	// sizes shares resource sizes (resolved URL -> int64) across the resources
	// created by Get, so serving many requests for the same resource — every
	// one of which needs the length — performs a single HEAD request over the
	// fetcher's lifetime instead of one per request.
	sizes sync.Map
}

func NewHTTPFetcher(href string, client *http.Client, url url.AbsoluteURL) *HTTPFetcher {
	if client == nil {
		panic("HTTPFetcher requires a non-nil client")
	}
	return &HTTPFetcher{
		href:   href,
		client: client,
		url:    url.Clone(),
	}
}

// Links implements Fetcher
func (f *HTTPFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	// It's impossible to determine what the items in a folder are on a remote HTTP server
	// This limits the parsers' abilities to realize that a folder is a certain type of publication
	if strings.HasSuffix(f.url.Path(), "/") {
		// Folder
		return manifest.LinkList{{
			Href:      manifest.NewHREF(url.MustURLFromString(f.href)),
			MediaType: &mediatype.Binary,
		}}, nil
	}

	// No slash, assume a file
	ext := path.Ext(f.url.Filename())
	if ext != "" {
		ext = ext[1:]
	}
	mt := mediatype.OfExtension(ext)
	if mt == nil {
		mt = &mediatype.Binary
	}

	return manifest.LinkList{{
		Href:      manifest.NewHREF(url.MustURLFromString(f.href)),
		MediaType: mt,
	}}, nil
}

// Get implements Fetcher
func (f *HTTPFetcher) Get(ctx context.Context, link manifest.Link) Resource {
	linkHref := link.Href.String()
	if after, ok := strings.CutPrefix(linkHref, f.href); ok {
		rurl, err := url.RelativeURLFromString(after)
		if err == nil {
			resolved := f.url.Resolve(rurl).(url.AbsoluteURL)
			// Keep the resource within the base URL: a `..` or root-absolute HREF must
			// not let an incoming request reach a path outside the fetcher's location.
			// Relativize returns a relative URL only when [resolved] is under [f.url].
			if _, ok := f.url.Relativize(resolved).(url.RelativeURL); ok {
				return &httpResource{
					link:   link,
					client: f.client,
					url:    resolved,
					sizes:  &f.sizes,
				}
			}
		}
	}

	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in HTTPFetcher paths")))
}

func (f *HTTPFetcher) Close() {
	// No-op for HTTP
}

// Location implements [Located].
func (r *httpResource) Location() url.AbsoluteURL {
	return r.url.Clone()
}

// Creates a [Resource] serving the contents of the given HTTP(S) URL.
func NewHTTPResource(link manifest.Link, client *http.Client, url url.AbsoluteURL) Resource {
	if client == nil {
		panic("NewHTTPResource requires a non-nil client")
	}
	return &httpResource{
		link:   link,
		client: client,
		url:    url.Clone(),
		sizes:  new(sync.Map),
	}
}

// Resource from HTTP
type httpResource struct {
	link   manifest.Link
	client *http.Client
	url    url.AbsoluteURL

	pinned atomic.Bool
	sizes  *sync.Map
}

// Link implements Resource
func (r *httpResource) Link() manifest.Link {
	return r.link
}

// Properties implements Resource
func (r *httpResource) Properties() manifest.Properties {
	return manifest.Properties{}
}

// Close implements Resource
func (r *httpResource) Close() {
	// No-op for HTTP
}

// File implements Resource
func (r *httpResource) File() string {
	return ""
}

// PinLocation binds HEAD and GET to the checked URL, before any redirect is
// followed. The caller's HTTP client is shared with unpinned resources.
func (r *httpResource) PinLocation() (url.URL, error) {
	r.pinned.Store(true)
	return r.Location(), nil
}

func (r *httpResource) do(req *http.Request, pinned bool) (*http.Response, error) {
	if !pinned {
		return r.client.Do(req)
	}
	client := http.Client{
		Transport:     r.client.Transport,
		Jar:           r.client.Jar,
		Timeout:       r.client.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return client.Do(req)
}

func (r *httpResource) size(ctx context.Context) (int64, *ResourceError) {
	key := struct {
		url    string
		pinned bool
	}{r.url.String(), r.pinned.Load()}
	if v, ok := r.sizes.Load(key); ok {
		return v.(int64), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.url.String(), nil)
	if err != nil {
		return 0, Other(err)
	}
	resp, err := r.do(req, key.pinned)
	if err != nil {
		return 0, Other(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, unexpectedHTTPStatus(resp.StatusCode)
	}
	if !slices.Contains(resp.Header.Values("Accept-Ranges"), "bytes") {
		return 0, Other(errors.New("HTTP server does not support byte range requests"))
	}
	if resp.ContentLength < 0 {
		return 0, Other(errors.New("HTTP server did not return Content-Length"))
	}
	r.sizes.Store(key, resp.ContentLength)
	return resp.ContentLength, nil
}

func unexpectedHTTPStatus(status int) *ResourceError {
	if ex := httpStatusToException(status); ex != nil {
		return ex
	}
	return Other(errors.Errorf("unexpected HTTP status code: %d", status))
}

// Read implements Resource.
func (r *httpResource) Read(ctx context.Context, start, end int64) ([]byte, *ResourceError) {
	var data bytes.Buffer
	if _, err := r.Stream(ctx, &data, start, end); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// Stream implements Resource.
func (r *httpResource) Stream(ctx context.Context, w io.Writer, start, end int64) (int64, *ResourceError) {
	start, valid := byterange.Normalize(start, end)
	if !valid {
		return 0, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url.String(), nil)
	if err != nil {
		return 0, Other(err)
	}
	ranged := start != 0 || end != 0
	if ranged {
		header := "bytes=" + strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end, 10)
		req.Header.Set("Range", header)
	}
	resp, err := r.do(req, r.pinned.Load())
	if err != nil {
		return 0, Other(err)
	}
	defer resp.Body.Close()
	var n int64
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		if !ranged {
			end = -1
		}
		length, errRange := httprange.Length(resp, start, end, -1)
		if errRange != nil {
			return 0, Other(errRange)
		}
		n, err = io.CopyN(w, resp.Body, length)
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		if err == nil && resp.ContentLength < 0 {
			var extra [1]byte
			if _, eof := io.ReadFull(resp.Body, extra[:]); eof != io.EOF {
				return n, Other(errors.New("HTTP response exceeds its range or has an invalid ending"))
			}
		}
	case !ranged && resp.StatusCode == http.StatusOK:
		n, err = io.Copy(w, resp.Body)
	default:
		return 0, unexpectedHTTPStatus(resp.StatusCode)
	}
	if err != nil {
		return n, Other(err)
	}
	return n, nil
}

// HasEfficientStream implements EfficientStreamer. Stream performs a single
// ranged HTTP request and pipes the response body through as it arrives.
func (r *httpResource) HasEfficientStream() bool {
	return true
}

// Length implements Resource
func (r *httpResource) Length(ctx context.Context) (int64, *ResourceError) {
	size, rerr := r.size(ctx)
	if rerr != nil {
		return 0, rerr
	}
	return size, nil
}

func httpStatusToException(status int) *ResourceError {
	if status == 0 {
		return nil
	}

	switch status {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusPartialContent, http.StatusNoContent, http.StatusResetContent, http.StatusNotModified:
		return nil
	default:
		return NewResourceError(ResourceErrorCode(status))
	}
}
