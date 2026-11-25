package fetcher

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fileNames are names which aren't valid URL paths as they are: they were
// taken for encoded hrefs, so that "page%20one.txt" named "page one.txt",
// "a#b.txt" named "a", and "100%.png" made the whole listing fail.
var fileNames = []string{
	"page one.txt", "page%20one.txt", "100%.png", "a#b.txt", "a?b.txt", "c:file.txt", "dir/x%2Fy.txt", "[x].txt",
}

// assertListsEachName checks that the links of the fetcher are the given
// names, each of them serving its own content, which is its name.
func assertListsEachName(t *testing.T, f Fetcher, names []string) {
	t.Helper()
	links, err := f.Links(t.Context())
	require.NoError(t, err)
	listed := make(map[string]bool)
	for _, link := range links {
		u := link.URL(nil, nil)
		require.NotNil(t, u, link.Href.String())
		name := u.Path()
		assert.False(t, listed[name], "%s is listed twice", name)
		listed[name] = true
		data, rerr := f.Get(t.Context(), link).Read(t.Context(), 0, 0)
		require.Nil(t, rerr, name)
		assert.Equal(t, name, string(data), "the link to %s serves another file", name)

		// Clients get the href serialized, e.g. "./c:file.txt".
		href, err := manifest.NewHREFFromString(link.Href.String(), false)
		require.NoError(t, err, link.Href.String())
		data, rerr = f.Get(t.Context(), manifest.Link{Href: href}).Read(t.Context(), 0, 0)
		require.Nil(t, rerr, "%s parsed back from %s", name, link.Href.String())
		assert.Equal(t, name, string(data), "%s parsed back from %s", name, link.Href.String())
	}
	for _, name := range names {
		assert.True(t, listed[name], "%s is not listed", name)
	}
}

func TestFileHREFKeepsListedSpellings(t *testing.T) {
	// Hrefs saved by clients, such as locators, must still match.
	for name, expected := range map[string]string{
		"chapter.xhtml":  "chapter.xhtml",
		"page one.txt":   "page%20one.txt",
		"[x].jpg":        "[x].jpg",
		"page%20one.txt": "page%2520one.txt",
		"a#b.txt":        "a%23b.txt",
	} {
		href, err := fileHREF(name)
		require.NoError(t, err, name)
		assert.Equal(t, expected, href.String(), name)
	}
}

func TestArchiveFetcherListsEachName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.zip")
	out, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(out)
	for _, name := range fileNames {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(name))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, out.Close())

	f, err := NewArchiveFetcherFromPath(t.Context(), path)
	require.NoError(t, err)
	defer f.Close()
	assertListsEachName(t, f, fileNames)

	// Dot segments are resolved, but paths escaping the archive are still
	// refused.
	for href, found := range map[string]bool{"dir/../page%20one.txt": true, "../page%20one.txt": false, "dir/../../page%20one.txt": false} {
		_, rerr := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
		assert.Equal(t, found, rerr == nil, href)
	}
}

func TestFileFetcherListsEachName(t *testing.T) {
	dir := t.TempDir()
	var names []string
	for _, name := range fileNames {
		if runtime.GOOS == "windows" && (filepath.Base(name) == "a?b.txt" || name == "c:file.txt") {
			continue
		}
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(name), 0o644))
		names = append(names, name)
	}
	assertListsEachName(t, NewFileFetcher("", dir), names)
}

func TestFileFetcherConcurrentGets(t *testing.T) {
	// Gets appended to the resources to close without a lock.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	f := NewFileFetcher("", dir)
	links, err := f.Links(t.Context())
	require.NoError(t, err)
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 50 {
				f.Get(t.Context(), links[0]).Close()
			}
		}()
	}
	for range 8 {
		<-done
	}
	f.Close()
}

func TestLocalFetchersRefuseRemoteURLs(t *testing.T) {
	// The path of any URL was looked up, whatever its scheme or authority:
	// "https://cdn.example.org/secret.txt" served the local secret.txt.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644))
	zipPath := filepath.Join(t.TempDir(), "package.zip")
	out, err := os.Create(zipPath)
	require.NoError(t, err)
	zw := zip.NewWriter(out)
	w, err := zw.Create("secret.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("secret"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, out.Close())
	archive, err := NewArchiveFetcherFromPath(t.Context(), zipPath)
	require.NoError(t, err)
	defer archive.Close()

	for name, f := range map[string]Fetcher{"file": NewFileFetcher("", dir), "archive": archive} {
		_, rerr := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("secret.txt", false)}).Read(t.Context(), 0, 0)
		require.Nil(t, rerr, name)
		for _, href := range []string{"https://cdn.example.org/secret.txt", "//cdn.example.org/secret.txt", "file:///secret.txt"} {
			data, rerr := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
			assert.NotNil(t, rerr, "%s served %q for %s", name, data, href)
		}
	}
}

func TestArchiveFetcherReportsIncompleteWalks(t *testing.T) {
	// The walk of an exploded archive stopped at an unreadable directory,
	// and the files listed until then were taken for all of them.
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test can't read")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	locked := filepath.Join(dir, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "b.txt"), []byte("b"), 0o644))
	require.NoError(t, os.Chmod(locked, 0))
	t.Cleanup(func() { assert.NoError(t, os.Chmod(locked, 0o755)) })

	f, err := NewArchiveFetcherFromPath(t.Context(), dir)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.Links(t.Context())
	assert.Error(t, err)
}
