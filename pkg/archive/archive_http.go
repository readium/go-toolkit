package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/httprange"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type HTTPArchiveFactory struct {
	client *http.Client
	config RemoteArchiveConfig
}

// Open implements ArchiveFactory
func (e HTTPArchiveFactory) Open(ctx context.Context, location url.URL, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	absLocation, ok := location.(url.AbsoluteURL)
	if !ok {
		return nil, errors.New("HTTP archive location is not an absolute URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, absLocation.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// If it's not code 200, the file doesn't exist
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("HTTP HEAD request failed with status code: %d", resp.StatusCode)
	}

	// HTTP server *must* support byte range requests
	arvs := resp.Header.Values("Accept-Ranges")
	if !slices.Contains(arvs, "bytes") {
		return nil, errors.New("HTTP server does not support byte range requests")
	}

	// HTTP server *must* return Content-Length header
	if resp.ContentLength <= 0 {
		return nil, errors.New("HTTP server returned zero content length")
	}

	// Setup remote ZIP archive reading
	rdr := newRemoteZIPAdapter(RemoteArchiveReaderFromHTTP(e.client, absLocation, resp.ContentLength), e.config)
	r, err := zip.NewReader(rdr, resp.ContentLength)
	if err != nil {
		return nil, err
	}
	rdr.makeReady()

	return &gozipArchive{
		zip:           r,
		minimizeReads: true,
		closer:        rdr.Close,
		size:          resp.ContentLength,
	}, nil
}

// CanOpen implements SchemeSpecificArchiveFactory
func (e HTTPArchiveFactory) CanOpen(scheme url.Scheme) bool {
	return scheme == url.SchemeHTTP || scheme == url.SchemeHTTPS
}

// OpenBytes implements ArchiveFactory
func (e HTTPArchiveFactory) OpenBytes(ctx context.Context, data []byte, password string) (Archive, error) {
	return nil, errors.New("HTTP archives must be opened with Open")
}

// OpenReader implements ArchiveFactory
func (e HTTPArchiveFactory) OpenReader(ctx context.Context, reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	return nil, errors.New("HTTP archives must be opened with Open")
}

func NewHTTPArchiveFactory(client *http.Client, config RemoteArchiveConfig) HTTPArchiveFactory {
	return HTTPArchiveFactory{
		client: client,
		config: config,
	}
}

// HTTP-specific reader
type remoteHTTPReader struct {
	client *http.Client
	url    string
	size   int64
}

func (r remoteHTTPReader) ReadRange(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if length == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	if offset < 0 || offset >= r.size {
		return nil, io.EOF
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return nil, err
	}
	remaining := r.size - offset
	if length < 0 || length > remaining {
		length = remaining
	}
	end := offset + (length - 1)
	req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-"+strconv.FormatInt(end, 10))

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusPartialContent {
		_ = resp.Body.Close()
		return nil, errors.New("unexpected HTTP status code: " + strconv.Itoa(resp.StatusCode))
	}

	n, err := httprange.Length(resp, offset, end, r.size)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, n), resp.Body}, nil
}

func (r remoteHTTPReader) Size() int64 {
	return r.size
}

func RemoteArchiveReaderFromHTTP(client *http.Client, url url.AbsoluteURL, size int64) RemoteArchiveReader {
	return &remoteHTTPReader{
		client: client,
		url:    url.String(),
		size:   size,
	}
}
