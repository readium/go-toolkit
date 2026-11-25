package fetcher

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// symlinkTree creates a publication directory holding a file, a symbolic
// link to it, and symbolic links to a file outside of the directory.
func symlinkTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("TOP SECRET"), 0o644))
	pub := filepath.Join(dir, "pub")
	require.NoError(t, os.Mkdir(pub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pub, "inside.txt"), []byte("inside"), 0o644))
	require.NoError(t, os.Symlink("inside.txt", filepath.Join(pub, "alias.txt")))
	require.NoError(t, os.Symlink(secret, filepath.Join(pub, "absolute.txt")))
	require.NoError(t, os.Symlink(filepath.Join("..", "secret.txt"), filepath.Join(pub, "relative.txt")))
	return pub
}

func TestFileFetcherSymlinks(t *testing.T) {
	f := NewFileFetcher("", symlinkTree(t))
	defer f.Close()
	get := func(href string) Resource {
		return f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)})
	}

	// Symbolic links within the directory are followed.
	data, rerr := get("alias.txt").Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, "inside", string(data))

	// Not those leading outside of it.
	for _, href := range []string{"absolute.txt", "relative.txt"} {
		res := get(href)
		_, rerr := res.Read(t.Context(), 0, 0)
		assert.NotNil(t, rerr, href)
		var buf bytes.Buffer
		_, rerr = res.Stream(t.Context(), &buf, 0, 0)
		assert.NotNil(t, rerr, href)
		assert.NotContains(t, buf.String(), "TOP SECRET", href)
		_, rerr = res.Length(t.Context())
		assert.NotNil(t, rerr, href)
		assert.Empty(t, res.File(), href)
	}

	links, err := f.Links(t.Context())
	require.NoError(t, err)
	var hrefs []string
	for _, l := range links {
		hrefs = append(hrefs, l.Href.String())
	}
	assert.ElementsMatch(t, []string{"inside.txt", "alias.txt"}, hrefs)
}

func TestTransformingResourceHidesFile(t *testing.T) {
	res := NewFileFetcher("", symlinkTree(t)).Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("inside.txt", false)})
	require.NotEmpty(t, res.File())
	assert.Empty(t, NewTransformingResource(res, false, bytes.ToUpper).File())
}

func TestFileBacked(t *testing.T) {
	pub := symlinkTree(t)
	link := func(href string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}
	}
	sameFile := func(a, b Resource) bool {
		ai, err := a.(FileBacked).FileInfo()
		require.NoError(t, err)
		bi, err := b.(FileBacked).FileInfo()
		require.NoError(t, err)
		return os.SameFile(ai, bi)
	}

	// Through a symbolic link, from a directory or an exploded archive, and
	// through transformations.
	files := NewFileFetcher("", pub)
	exploded := NewArchiveFetcher(archive.NewExplodedArchive(pub))
	for name, f := range map[string]Fetcher{"directory": files, "exploded archive": exploded} {
		inside, alias := f.Get(t.Context(), link("inside.txt")), f.Get(t.Context(), link("alias.txt"))
		assert.True(t, sameFile(inside, alias), name)
		transformed := NewTransformingResource(alias, false, bytes.ToUpper)
		assert.True(t, sameFile(inside, ProxyResource{Res: transformed}), name)
	}

	// Not from a ZIP archive.
	zipped, err := NewArchiveFetcherFromPath(t.Context(), "./testdata/epub.epub")
	require.NoError(t, err)
	defer zipped.Close()
	links, err := zipped.Links(t.Context())
	require.NoError(t, err)
	_, err = zipped.Get(t.Context(), links[0]).(FileBacked).FileInfo()
	assert.Error(t, err)
}
