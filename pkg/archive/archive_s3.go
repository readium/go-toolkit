package archive

import (
	"archive/zip"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type S3ArchiveFactory struct {
	client *s3.Client
	config RemoteArchiveConfig
}

// Open implements RemoteArchiveFactory
func (e S3ArchiveFactory) OpenWithContext(ctx context.Context, location url.URL, password string) (Archive, error) {
	// Go's built-in zip reader doesn't support passwords.
	if password != "" {
		return nil, errors.New("password-protected archives not supported")
	}

	absLocation, ok := location.(url.AbsoluteURL)
	if !ok {
		return nil, errors.New("S3 archive location is not an absolute URL")
	}
	input, err := absLocation.ToS3Object()
	if err != nil {
		return nil, errors.Wrap(err, "invalid S3 archive location")
	}

	// Get object attributes
	output, err := e.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: input.Bucket,
		Key:    input.Key,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get S3 archive's attributes")
	}

	// Setup remote ZIP archive reading
	rdr := newRemoteZIPAdapter(RemoteArchiveReaderFromS3(e.client, *output, *input), e.config)
	r, err := zip.NewReader(rdr, *output.ContentLength)
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
func (e S3ArchiveFactory) CanOpen(scheme url.Scheme) bool {
	return scheme == url.SchemeS3
}

// Open implements ArchiveFactory
func (e S3ArchiveFactory) Open(location url.URL, password string) (Archive, error) {
	return nil, errors.New("S3 archives must be opened with OpenWithContext")
}

// OpenBytes implements ArchiveFactory
func (e S3ArchiveFactory) OpenBytes(data []byte, password string) (Archive, error) {
	return nil, errors.New("S3 archives must be opened with OpenWithContext")
}

// OpenReader implements ArchiveFactory
func (e S3ArchiveFactory) OpenReader(reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	return nil, errors.New("S3 archives must be opened with OpenWithContext")
}

func NewS3ArchiveFactory(client *s3.Client, config RemoteArchiveConfig) S3ArchiveFactory {
	return S3ArchiveFactory{
		client: client,
		config: config,
	}
}
