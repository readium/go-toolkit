package archive

import (
	"archive/zip"
	"context"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type GCSArchiveFactory struct {
	client *storage.Client
	config RemoteArchiveConfig
}

// Open implements RemoteArchiveFactory
func (e GCSArchiveFactory) OpenWithContext(ctx context.Context, location url.URL, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	absLocation, ok := location.(url.AbsoluteURL)
	if !ok {
		return nil, errors.New("GCS archive location is not an absolute URL")
	}
	handle, err := absLocation.ToGSObject(e.client)
	if err != nil {
		return nil, errors.Wrap(err, "invalid GCS archive location")
	}

	// Get object attributes
	attrs, err := handle.Attrs(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get GCS archive's attributes")
	}

	// Setup remote ZIP archive reading
	rdr := newRemoteZIPAdapter(RemoteArchiveReaderFromGCS(handle, attrs), e.config)
	r, err := zip.NewReader(rdr, attrs.Size)
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
func (e GCSArchiveFactory) CanOpen(scheme url.Scheme) bool {
	return scheme == url.SchemeGS
}

// Open implements ArchiveFactory
func (e GCSArchiveFactory) Open(location url.URL, password string) (Archive, error) {
	return nil, errors.New("GCS archives must be opened with OpenWithContext")
}

// OpenBytes implements ArchiveFactory
func (e GCSArchiveFactory) OpenBytes(data []byte, password string) (Archive, error) {
	return nil, errors.New("GCS archives must be opened with OpenWithContext")
}

// OpenReader implements ArchiveFactory
func (e GCSArchiveFactory) OpenReader(reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	return nil, errors.New("GCS archives must be opened with OpenWithContext")
}

func NewGCSArchiveFactory(client *storage.Client, config RemoteArchiveConfig) GCSArchiveFactory {
	return GCSArchiveFactory{
		client: client,
		config: config,
	}
}
