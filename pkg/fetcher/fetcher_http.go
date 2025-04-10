package fetcher

import (
	"context"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type HTTPFetcher struct {
	href   string
	client *http.Client
	url    url.AbsoluteURL
}

func NewHTTPFetcher(href string, client *http.Client, url url.AbsoluteURL) *HTTPFetcher {
	if client == nil {
		panic("HTTPFetcher requires a non-nil client")
	}
	return &HTTPFetcher{
		href:   href,
		client: client,
		url:    url,
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
	if strings.HasPrefix(linkHref, f.href) {
		rurl, err := url.RelativeURLFromString(strings.TrimPrefix(linkHref, f.href))
		if err == nil {
			return &httpResource{
				link:   link,
				client: f.client,
				url:    f.url.Resolve(rurl).(url.AbsoluteURL),
			}
		}
	}

	return NewFailureResource(link, NotFound(errors.New("couldn't find "+linkHref+" in HTTPFetcher paths")))
}

func (f *HTTPFetcher) Close() {
	// No-op for HTTP
}

// Resource from HTTP
type httpResource struct {
	link   manifest.Link
	client *http.Client
	url    url.AbsoluteURL

	cachedSize *int64
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

func (r *httpResource) size(ctx context.Context) (int64, *ResourceError) {
	if r.cachedSize == nil {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, r.url.String(), nil)
		if err != nil {
			return 0, Other(err)
		}
		resp, err := r.client.Do(req)
		if err != nil {
			return 0, Other(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return 0, httpStatusToException(resp.StatusCode)
		}

		// HTTP server *must* support byte range requests
		arvs := resp.Header.Values("Accept-Ranges")
		if !slices.Contains(arvs, "bytes") {
			return 0, Other(errors.New("HTTP server does not support byte range requests"))
		}

		// HTTP server *must* return Content-Length header
		lengthStr := resp.Header.Get("Content-Length")
		if lengthStr == "" {
			return 0, Other(errors.New("HTTP server did not return Content-Length header"))
		}
		length, err := strconv.ParseInt(lengthStr, 10, 64)
		if err != nil {
			return 0, Other(errors.Wrap(err, "failed to parse Content-Length header"))
		}
		r.cachedSize = &length

	}
	return *r.cachedSize, nil
}

// Read implements Resource
func (r *httpResource) Read(ctx context.Context, start int64, end int64) ([]byte, *ResourceError) {
	if end < start {
		return nil, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url.String(), nil)
	if err != nil {
		return nil, Other(err)
	}

	if start != 0 || end != 0 {
		var sb strings.Builder
		sb.WriteString("bytes=")
		sb.WriteString(strconv.FormatInt(start, 10))
		sb.WriteString("-")
		if end > 0 {
			sb.WriteString(strconv.FormatInt(end, 10))
		}
		req.Header.Set("Range", sb.String())
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, Other(err)
	}
	if resp.StatusCode != http.StatusPartialContent {
		ex := httpStatusToException(resp.StatusCode)
		if ex == nil {
			return nil, Other(errors.New("unexpected HTTP status code: " + strconv.Itoa(resp.StatusCode)))
		}
		return nil, ex
	}
	defer resp.Body.Close()

	var data []byte
	if resp.ContentLength >= 0 {
		data = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, data)
	} else {
		data, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, Other(err)
	}
	return data, nil
}

// Stream implements Resource
func (r *httpResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *ResourceError) {
	if end < start {
		return -1, RangeNotSatisfiable(errors.New("end of range smaller than start"))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url.String(), nil)
	if err != nil {
		return -1, Other(err)
	}

	if start != 0 || end != 0 {
		var sb strings.Builder
		sb.WriteString("bytes=")
		sb.WriteString(strconv.FormatInt(start, 10))
		sb.WriteString("-")
		if end > 0 {
			sb.WriteString(strconv.FormatInt(end, 10))
		}
		req.Header.Set("Range", sb.String())
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return -1, Other(err)
	}
	if resp.StatusCode != http.StatusPartialContent {
		ex := httpStatusToException(resp.StatusCode)
		if ex == nil {
			return -1, Other(errors.New("unexpected HTTP status code: " + strconv.Itoa(resp.StatusCode)))
		}
		return -1, ex
	}
	defer resp.Body.Close()

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return -1, Other(err)
	}
	return n, nil
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
