//go:build !readium_nocloud

package asset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

// bucketObjects are the objects of the fake buckets, at their root.
var bucketObjects = []string{"manifest.json", "images/cover.jpg"}

// listed returns the names of the objects starting with prefix.
func listed(prefix string) []string {
	var names []string
	for _, name := range bucketObjects {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names
}

// linkPaths returns the sorted decoded paths of the links of the asset's
// fetcher.
func linkPaths(t *testing.T, a PublicationAsset) []string {
	t.Helper()
	f, err := a.CreateFetcher(t.Context(), Dependencies{ArchiveFactory: archive.NewArchiveFactory()}, "")
	require.NoError(t, err)
	defer f.Close()
	links, err := f.Links(t.Context())
	require.NoError(t, err)
	var paths []string
	for _, l := range links {
		paths = append(paths, l.URL(nil, nil).Path())
	}
	slices.Sort(paths)
	return paths
}

func TestCloudAssetsOfTheBucketRoot(t *testing.T) {
	// The root of a bucket was probed as the folder "/", which no object
	// name starts with: it was taken for an object without a name.
	s3server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("list-type") != "2" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name>`)
		for _, name := range listed(r.URL.Query().Get("prefix")) {
			fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>1</Size></Contents>`, name)
		}
		b.WriteString(`<IsTruncated>false</IsTruncated></ListBucketResult>`)
		w.Header().Set("Content-Type", "application/xml")
		_, err := w.Write([]byte(b.String()))
		assert.NoError(t, err)
	}))
	t.Cleanup(s3server.Close)
	s3client := s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String(s3server.URL),
		UsePathStyle: true,
	})
	// With or without a trailing slash: "s3://bucket" was the object ".".
	for _, root := range []string{"s3://bucket/", "s3://bucket"} {
		s3root, err := url.AbsoluteURLFromString(root)
		require.NoError(t, err)
		assert.Equal(t, []string{"images/cover.jpg", "manifest.json"}, linkPaths(t, S3(s3client, s3root)), root)
	}

	gcsserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var items []map[string]string
		var prefixes []string
		for _, name := range listed(q.Get("prefix")) {
			rest := strings.TrimPrefix(name, q.Get("prefix"))
			if d := q.Get("delimiter"); d != "" && strings.Contains(rest, d) {
				prefixes = append(prefixes, q.Get("prefix")+rest[:strings.Index(rest, d)+1])
				continue
			}
			items = append(items, map[string]string{"name": name, "bucket": "bucket", "size": "1"})
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"kind": "storage#objects", "items": items, "prefixes": prefixes}))
	}))
	t.Cleanup(gcsserver.Close)
	gcsclient, err := storage.NewClient(t.Context(), option.WithEndpoint(gcsserver.URL+"/storage/v1/"), option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, gcsclient.Close()) })
	for _, root := range []string{"gs://bucket/", "gs://bucket"} {
		gcsroot, err := url.AbsoluteURLFromString(root)
		require.NoError(t, err)
		assert.Equal(t, []string{"images/cover.jpg", "manifest.json"}, linkPaths(t, GCS(gcsclient, gcsroot)), root)
	}
}
