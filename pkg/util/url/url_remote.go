package url

import (
	"errors"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Turns an absolute URL into an S3 object
// We could theoretically accept https S3 URLs like s3.amazonaws.com,
// but the potential endpoints are way to complex, and this would also
// exclude third-party services that have S3 compatibility. Instead,
// the user of the toolkit should turn their data into an s3 URI, meaning
// the structure s3://<bucket>/<key>
func (u AbsoluteURL) ToS3Object() (*s3.GetObjectInput, error) {
	if u.scheme != "s3" {
		return nil, errors.New("not an s3 url")
	}

	path := strings.TrimPrefix(path.Clean(u.Path()), "/")
	return &s3.GetObjectInput{
		Bucket: &u.url.Host,
		Key:    &path,
	}, nil
}

func (u AbsoluteURL) ToGSObject(client *storage.Client) (*storage.ObjectHandle, error) {
	if u.scheme != "gs" {
		return nil, errors.New("not a gs url")
	}

	path := strings.TrimPrefix(path.Clean(u.Path()), "/")
	return client.Bucket(u.url.Host).Object(path), nil
}
