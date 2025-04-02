package archive

import (
	"archive/zip"
	"context"
	"net/http"
	"slices"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type HTTPArchiveFactory struct {
	client *http.Client
	config RemoteArchiveConfig
}

// Open implements RemoteArchiveFactory
func (e HTTPArchiveFactory) OpenWithContext(ctx context.Context, location url.URL, password string) (Archive, error) {
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
	}, nil
}

// CanOpen implements SchemeSpecificArchiveFactory
func (e HTTPArchiveFactory) CanOpen(scheme url.Scheme) bool {
	return scheme == url.SchemeHTTP || scheme == url.SchemeHTTPS
}

// Open implements ArchiveFactory
func (e HTTPArchiveFactory) Open(location url.URL, password string) (Archive, error) {
	return nil, errors.New("HTTP archives must be opened with OpenWithContext")
}

// OpenBytes implements ArchiveFactory
func (e HTTPArchiveFactory) OpenBytes(data []byte, password string) (Archive, error) {
	return nil, errors.New("HTTP archives must be opened with OpenWithContext")
}

// OpenReader implements ArchiveFactory
func (e HTTPArchiveFactory) OpenReader(reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	return nil, errors.New("HTTP archives must be opened with OpenWithContext")
}

func NewHTTPArchiveFactory(client *http.Client, config RemoteArchiveConfig) HTTPArchiveFactory {
	return HTTPArchiveFactory{
		client: client,
		config: config,
	}
}
