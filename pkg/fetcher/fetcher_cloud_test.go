package fetcher

import (
	"testing"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Object keys are raw strings on S3 and GCS: HREFs must be decoded before the lookup,
// and queries/fragments dropped.
func TestS3FetcherGetKey(t *testing.T) {
	f := NewS3Fetcher("", &s3.Client{}, "bucket", "pub")
	for _, tt := range []struct {
		href string
		key  string
	}{
		{"track.mp3", "pub/track.mp3"},
		{"my%20track.mp3", "pub/my track.mp3"},
		{"my track.mp3", "pub/my track.mp3"},
		{"caf%C3%A9.mp3", "pub/café.mp3"},
		{"track.mp3#t=30", "pub/track.mp3"},
		{"track.mp3?token=abc", "pub/track.mp3"},
		{"audio/track.mp3", "pub/audio/track.mp3"},
	} {
		res := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(tt.href, false)})
		s3res, ok := res.(*s3Resource)
		require.Truef(t, ok, "expected an s3Resource for %q", tt.href)
		assert.Equalf(t, tt.key, s3res.key, "key mismatch for href %q", tt.href)
	}
}

func TestGCSFetcherGetObjectName(t *testing.T) {
	client := &storage.Client{}
	f := NewGCSFetcher("", client, client.Bucket("bucket").Object("pub"))
	for _, tt := range []struct {
		href string
		name string
	}{
		{"track.mp3", "pub/track.mp3"},
		{"my%20track.mp3", "pub/my track.mp3"},
		{"caf%C3%A9.mp3", "pub/café.mp3"},
		{"track.mp3#t=30", "pub/track.mp3"},
	} {
		res := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(tt.href, false)})
		gcsres, ok := res.(*gcsResource)
		require.Truef(t, ok, "expected a gcsResource for %q", tt.href)
		assert.Equalf(t, tt.name, gcsres.handle.ObjectName(), "object name mismatch for href %q", tt.href)
	}
}
