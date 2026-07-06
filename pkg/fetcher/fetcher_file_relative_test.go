package fetcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRelativeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "manifest"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "audio"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest", "manifest.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest", "sibling.txt"), []byte("sibling"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "audio", "track.mp3"), []byte("track"), 0o644))
	return dir
}

func relativeFetcherRead(t *testing.T, f *RelativeFileFetcher, href string) (string, *ResourceError) {
	t.Helper()
	resource := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)})
	defer resource.Close()
	data, rerr := resource.Read(t.Context(), 0, 0)
	return string(data), rerr
}

func TestRelativeFileFetcherFromFile(t *testing.T) {
	dir := newRelativeFixture(t)
	base, err := url.FromFilepath(filepath.Join(dir, "manifest", "manifest.json"))
	require.NoError(t, err)
	f, err := NewRelativeFileFetcher(base.(url.AbsoluteURL))
	require.NoError(t, err)
	defer f.Close()

	// A sibling of the base file.
	data, rerr := relativeFetcherRead(t, f, "sibling.txt")
	require.Nil(t, rerr)
	assert.Equal(t, "sibling", data)

	// A resource in a parent directory, like a web server would resolve it.
	data, rerr = relativeFetcherRead(t, f, "../audio/track.mp3")
	require.Nil(t, rerr)
	assert.Equal(t, "track", data)

	// Queries and fragments are dropped.
	data, rerr = relativeFetcherRead(t, f, "../audio/track.mp3?token=abc#t=30")
	require.Nil(t, rerr)
	assert.Equal(t, "track", data)

	// Missing resources fail.
	_, rerr = relativeFetcherRead(t, f, "missing.txt")
	assert.Equal(t, NotFound(rerr.Cause), rerr)

	// Non-file absolute HREFs are not served.
	_, rerr = relativeFetcherRead(t, f, "https://example.com/track.mp3")
	assert.Equal(t, NotFound(rerr.Cause), rerr)
}

func TestRelativeFileFetcherFromDirectory(t *testing.T) {
	dir := newRelativeFixture(t)
	base, err := url.FromFilepath(filepath.Join(dir, "manifest") + string(filepath.Separator))
	require.NoError(t, err)
	f, err := NewRelativeFileFetcher(base.(url.AbsoluteURL))
	require.NoError(t, err)
	defer f.Close()

	// HREFs resolve inside the base directory.
	data, rerr := relativeFetcherRead(t, f, "sibling.txt")
	require.Nil(t, rerr)
	assert.Equal(t, "sibling", data)

	data, rerr = relativeFetcherRead(t, f, "../audio/track.mp3")
	require.Nil(t, rerr)
	assert.Equal(t, "track", data)
}

func TestRelativeFileFetcherLinks(t *testing.T) {
	dir := newRelativeFixture(t)
	base, err := url.FromFilepath(filepath.Join(dir, "manifest", "manifest.json"))
	require.NoError(t, err)
	f, err := NewRelativeFileFetcher(base.(url.AbsoluteURL))
	require.NoError(t, err)
	defer f.Close()

	links, err := f.Links(t.Context())
	require.NoError(t, err)
	hrefs := make([]string, 0, len(links))
	for _, link := range links {
		hrefs = append(hrefs, link.Href.String())
	}
	assert.ElementsMatch(t, []string{"manifest.json", "sibling.txt"}, hrefs)
}
