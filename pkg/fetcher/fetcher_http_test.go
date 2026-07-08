package fetcher

import (
	"net/http"
	"testing"

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
