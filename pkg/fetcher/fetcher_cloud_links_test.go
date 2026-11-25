//go:build !readium_nocloud

package fetcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

// listedPaths returns the decoded paths of the links, failing on empty ones.
func listedPaths(t *testing.T, links manifest.LinkList) []string {
	t.Helper()
	var paths []string
	for _, link := range links {
		u := link.URL(nil, nil)
		require.NotNil(t, u)
		require.NotEmpty(t, u.Path(), "empty link")
		paths = append(paths, u.Path())
	}
	slices.Sort(paths)
	return paths
}

// cloudObject is an object of a fake bucket.
type cloudObject struct {
	name string
	size int
}

// matching returns the objects whose name starts with prefix, in order.
func matching(objects []cloudObject, prefix string) []cloudObject {
	var matched []cloudObject
	for _, o := range objects {
		if strings.HasPrefix(o.name, prefix) {
			matched = append(matched, o)
		}
	}
	return matched
}

// fakeS3 returns a client of a fake S3 bucket holding the objects, listing
// them two per page, as ListObjectsV2 does.
func fakeS3(t *testing.T, objects []cloudObject) *s3.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		listed := matching(objects, q.Get("prefix"))
		page := 0
		if token := q.Get("continuation-token"); token != "" {
			if _, err := fmt.Sscanf(token, "page%d", &page); err != nil || page < 0 {
				http.Error(w, "invalid page token", http.StatusBadRequest)
				return
			}
		}
		start, end := min(page*2, len(listed)), min(page*2+2, len(listed))
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bucket</Name>`)
		for _, o := range listed[start:end] {
			fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>%d</Size></Contents>`, o.name, o.size)
		}
		if end < len(listed) {
			fmt.Fprintf(&b, `<IsTruncated>true</IsTruncated><NextContinuationToken>page%d</NextContinuationToken>`, page+1)
		} else {
			b.WriteString(`<IsTruncated>false</IsTruncated>`)
		}
		b.WriteString(`</ListBucketResult>`)
		w.Header().Set("Content-Type", "application/xml")
		_, err := w.Write([]byte(b.String()))
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	return s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  aws.AnonymousCredentials{},
		BaseEndpoint: aws.String(server.URL),
		UsePathStyle: true,
	})
}

// fakeGCS returns a client of a fake GCS bucket holding the objects, listing
// them two per page, and splitting them into prefixes with a delimiter, as
// the JSON API does.
func fakeGCS(t *testing.T, objects []cloudObject) *storage.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		type item struct {
			Name   string `json:"name"`
			Bucket string `json:"bucket"`
			Size   string `json:"size"`
		}
		var items []item
		var prefixes []string
		for _, o := range matching(objects, q.Get("prefix")) {
			rest := strings.TrimPrefix(o.name, q.Get("prefix"))
			if d := q.Get("delimiter"); d != "" && strings.Contains(rest, d) {
				p := q.Get("prefix") + rest[:strings.Index(rest, d)+1]
				if !slices.Contains(prefixes, p) {
					prefixes = append(prefixes, p)
				}
				continue
			}
			items = append(items, item{Name: o.name, Bucket: "bucket", Size: fmt.Sprint(o.size)})
		}
		page := 0
		if token := q.Get("pageToken"); token != "" {
			if _, err := fmt.Sscanf(token, "page%d", &page); err != nil || page < 0 {
				http.Error(w, "invalid page token", http.StatusBadRequest)
				return
			}
		}
		start, end := min(page*2, len(items)), min(page*2+2, len(items))
		body := map[string]any{"kind": "storage#objects", "items": items[start:end]}
		if page == 0 && len(prefixes) > 0 {
			body["prefixes"] = prefixes
		}
		if end < len(items) {
			body["nextPageToken"] = fmt.Sprintf("page%d", page+1)
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(body))
	}))
	t.Cleanup(server.Close)
	client, err := storage.NewClient(t.Context(), option.WithEndpoint(server.URL+"/storage/v1/"), option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, client.Close()) })
	return client
}

// cloudFetchers returns an S3 and a GCS fetcher of the objects under key.
func cloudFetchers(t *testing.T, objects []cloudObject, key string) map[string]Fetcher {
	gcs := fakeGCS(t, objects)
	return map[string]Fetcher{
		"S3":  NewS3Fetcher("", fakeS3(t, objects), "bucket", key),
		"GCS": NewGCSFetcher("", gcs, gcs.Bucket("bucket").Object(key)),
	}
}

func TestCloudFetchersListEveryObject(t *testing.T) {
	// S3 only read the first page of the listing, and skipped folder
	// placeholders left empty links. GCS listed with the "/" delimiter, so
	// objects in subfolders weren't listed. Both skipped empty files.
	objects := []cloudObject{
		{"pub/a.txt", 1},
		{"pub/dir/", 0},
		{"pub/dir/page one.txt", 3},
		{"pub/dir/page%20one.txt", 4},
		{"pub/dir/a#b.txt", 5},
		{"pub/empty.txt", 0},
		{"other/x.txt", 1},
	}
	expected := []string{"a.txt", "dir/a#b.txt", "dir/page one.txt", "dir/page%20one.txt", "empty.txt"}
	for name, f := range cloudFetchers(t, objects, "pub") {
		assert.True(t, f.(Exhaustive).Exhaustive(), name)
		links, err := f.Links(t.Context())
		require.NoError(t, err, name)
		assert.Equal(t, expected, listedPaths(t, links), name)

		// The cached listing isn't shared with callers, who may change theirs.
		links[0] = manifest.Link{}
		again, err := f.Links(t.Context())
		require.NoError(t, err, name)
		assert.Equal(t, expected, listedPaths(t, again), name)
	}
}

func TestCloudFetchersListTheBucketRoot(t *testing.T) {
	// The empty prefix of the root of a bucket became "/", which no
	// object name starts with.
	objects := []cloudObject{{"a.txt", 1}, {"dir/b.txt", 2}}
	for name, f := range cloudFetchers(t, objects, "") {
		links, err := f.Links(t.Context())
		require.NoError(t, err, name)
		assert.Equal(t, []string{"a.txt", "dir/b.txt"}, listedPaths(t, links), name)
	}
}

func TestCloudFetchersRefuseUnrepresentableNames(t *testing.T) {
	// path.Join made distinct object names the same path.
	for _, object := range []string{"pub/images//cover.jpg", "pub/images/./cover.jpg", "pub/images/../cover.jpg"} {
		objects := []cloudObject{{"pub/images/cover.jpg", 1}, {object, 2}}
		for name, f := range cloudFetchers(t, objects, "pub") {
			_, err := f.Links(t.Context())
			assert.ErrorContains(t, err, "can't be a path of the package", "%s %s", name, object)
		}
	}
}

func TestCloudResourceRanges(t *testing.T) {
	const content = "chapter"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "chapter.txt", time.Time{}, strings.NewReader(content))
	}))
	t.Cleanup(server.Close)
	gcs, err := storage.NewClient(t.Context(), storage.WithJSONReads(), option.WithEndpoint(server.URL+"/storage/v1/"), option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, gcs.Close()) })
	s3client := s3.New(s3.Options{Region: "us-east-1", Credentials: aws.AnonymousCredentials{}, BaseEndpoint: aws.String(server.URL), UsePathStyle: true})
	link := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.txt", false)}
	for name, f := range map[string]Fetcher{
		"s3":  NewS3Fetcher("", s3client, "bucket", ""),
		"gcs": NewGCSFetcher("", gcs, gcs.Bucket("bucket").Object("")),
	} {
		r := f.Get(t.Context(), link)
		for _, tc := range []struct {
			start, end int64
			want       string
		}{
			{-1, 0, content}, {-1, 2, "cha"}, {0, math.MaxInt64, content},
		} {
			got, rerr := r.Read(t.Context(), tc.start, tc.end)
			require.Nil(t, rerr, name)
			assert.Equal(t, tc.want, string(got), name)
			var buf bytes.Buffer
			_, rerr = r.Stream(t.Context(), &buf, tc.start, tc.end)
			require.Nil(t, rerr, name)
			assert.Equal(t, tc.want, buf.String(), name)
		}
		r.Close()
	}
}
