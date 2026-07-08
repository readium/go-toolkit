package fetcher

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The HTTP fetcher resolves HREFs against its base URL, but a `..` or root-absolute
// HREF must not let a request reach a path outside the base, matching the containment
// of the file and object-store fetchers.
func TestHTTPFetcherContainsPath(t *testing.T) {
	base, err := url.AbsoluteURLFromString("http://example.com/pub/")
	require.NoError(t, err)
	f := NewHTTPFetcher("", http.DefaultClient, base)

	// Escaping HREFs do not resolve to a network resource.
	for _, href := range []string{"../secret.txt", "../../secret.txt", "/secret.txt", "audio/../../secret.txt"} {
		res := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)})
		_, ok := res.(*httpResource)
		assert.Falsef(t, ok, "escaping href %q should not resolve", href)
	}

	// HREFs within the base resolve to the expected absolute URL.
	for _, tt := range []struct{ href, url string }{
		{"audio/track.mp3", "http://example.com/pub/audio/track.mp3"},
		{"audio/../cover.jpg", "http://example.com/pub/cover.jpg"},
	} {
		res := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(tt.href, false)})
		hres, ok := res.(*httpResource)
		require.Truef(t, ok, "href %q should resolve", tt.href)
		assert.Equal(t, tt.url, hres.url.String())
	}
}

// The bare-file remote resources advertise efficient streaming; archive
// entries advertise it based on their compression method.
var (
	_ EfficientStreamer = (*httpResource)(nil)
	_ EfficientStreamer = (*s3Resource)(nil)
	_ EfficientStreamer = (*gcsResource)(nil)
	_ EfficientStreamer = (*entryResource)(nil)
)

// Stream must perform exactly one upstream request per call: an un-ranged GET
// (answered with 200) when streaming the whole resource, and a single ranged
// GET (answered with 206) when streaming a range.
func TestHTTPResourceStream(t *testing.T) {
	payload := make([]byte, 100_000)
	for i := range payload {
		payload[i] = byte(i % 251)
	}

	var mu sync.Mutex
	var ranges []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		ranges = append(ranges, r.Header.Get("Range"))
		mu.Unlock()
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

	u, err := url.AbsoluteURLFromString(srv.URL + "/file.bin")
	require.NoError(t, err)
	res := NewHTTPResource(manifest.Link{}, srv.Client(), u)

	// Whole resource: no Range header is sent, and the origin's 200 response
	// must be accepted.
	var buf bytes.Buffer
	n, rerr := res.Stream(t.Context(), &buf, 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, int64(len(payload)), n)
	assert.Equal(t, payload, buf.Bytes())
	mu.Lock()
	assert.Equal(t, []string{""}, ranges)
	ranges = nil
	mu.Unlock()

	// Ranged: a single ranged request, answered with 206.
	buf.Reset()
	n, rerr = res.Stream(t.Context(), &buf, 100, 199)
	require.Nil(t, rerr)
	assert.Equal(t, int64(100), n)
	assert.Equal(t, payload[100:200], buf.Bytes())
	mu.Lock()
	assert.Equal(t, []string{"bytes=100-199"}, ranges)
	mu.Unlock()
}
