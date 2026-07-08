package fetcher

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestS3Resource returns an s3Resource backed by a fake S3 endpoint serving
// the given object payload, honouring Range requests.
func newTestS3Resource(t *testing.T, payload []byte) (*s3Resource, *[]string) {
	t.Helper()

	// Records the Range header of every GetObject request ("" when absent).
	ranges := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*ranges = append(*ranges, r.Header.Get("Range"))

		body := payload
		if rng := r.Header.Get("Range"); rng != "" {
			var start, end int64
			n, err := fmt.Sscanf(rng, "bytes=%d-%d", &start, &end)
			require.NoError(t, err)
			require.Equal(t, 2, n)
			require.LessOrEqual(t, start, end)
			if end >= int64(len(payload)) {
				end = int64(len(payload)) - 1
			}
			body = payload[start : end+1]
			w.Header().Set("Content-Range",
				"bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.Itoa(len(payload)))
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)

	client := s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String(server.URL),
		UsePathStyle: true,
	})
	return &s3Resource{client: client, bucket: "bucket", key: "key"}, ranges
}

func TestS3ResourceReadRange(t *testing.T) {
	payload := []byte("0123456789abcdefghij")
	res, ranges := newTestS3Resource(t, payload)

	data, err := res.Read(t.Context(), 5, 9)
	require.Nil(t, err)
	assert.Equal(t, []byte("56789"), data, "a ranged read must return only the requested bytes")
	require.Len(t, *ranges, 1)
	assert.Equal(t, "bytes=5-9", (*ranges)[0], "a ranged read must send a Range header")
}

func TestS3ResourceReadWhole(t *testing.T) {
	payload := []byte("0123456789abcdefghij")
	res, ranges := newTestS3Resource(t, payload)

	data, err := res.Read(t.Context(), 0, 0)
	require.Nil(t, err)
	assert.Equal(t, payload, data)
	require.Len(t, *ranges, 1)
	assert.Equal(t, "", (*ranges)[0], "a whole-object read must not send a Range header")
}
