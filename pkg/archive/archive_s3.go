package archive

import (
	"archive/zip"
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type S3ArchiveFactory struct {
	client *s3.Client
	config RemoteArchiveConfig
}

// Open implements ArchiveFactory
func (e S3ArchiveFactory) Open(ctx context.Context, location url.URL, password string) (Archive, error) {
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

// OpenBytes implements ArchiveFactory
func (e S3ArchiveFactory) OpenBytes(ctx context.Context, data []byte, password string) (Archive, error) {
	return nil, errors.New("S3 archives must be opened with Open")
}

// OpenReader implements ArchiveFactory
func (e S3ArchiveFactory) OpenReader(ctx context.Context, reader ReaderAtCloser, size int64, password string, minimizeReads bool) (Archive, error) {
	return nil, errors.New("S3 archives must be opened with Open")
}

func NewS3ArchiveFactory(client *s3.Client, config RemoteArchiveConfig) S3ArchiveFactory {
	return S3ArchiveFactory{
		client: client,
		config: config,
	}
}

// S3-specific reader
type remoteS3Reader struct {
	client *s3.Client
	input  s3.GetObjectInput
	head   s3.HeadObjectOutput
}

func (r remoteS3Reader) ReadRange(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		return nil, io.EOF
	}

	var sb strings.Builder
	sb.WriteString("bytes=")
	sb.WriteString(strconv.FormatInt(offset, 10))
	sb.WriteString("-")
	if length >= 0 {
		sb.WriteString(strconv.FormatInt(offset+length-1, 10))
	}
	r.input.Range = aws.String(sb.String())
	result, err := r.client.GetObject(ctx, &r.input)
	if err != nil {
		return nil, err
	}

	// User is responsible for closing the body
	return result.Body, nil
}

func (r remoteS3Reader) Size() int64 {
	if r.head.ContentLength != nil {
		return *r.head.ContentLength
	}
	return 0
}

func RemoteArchiveReaderFromS3(client *s3.Client, output s3.HeadObjectOutput, input s3.GetObjectInput) RemoteArchiveReader {
	return &remoteS3Reader{
		client: client,
		input:  input,
		head:   output,
	}
}
