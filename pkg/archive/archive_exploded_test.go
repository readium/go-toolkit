package archive

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplodedArchiveSymlinks(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("TOP SECRET"), 0o644))
	pub := filepath.Join(dir, "pub")
	require.NoError(t, os.Mkdir(pub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pub, "inside.txt"), []byte("inside"), 0o644))
	require.NoError(t, os.Symlink("inside.txt", filepath.Join(pub, "alias.txt")))
	require.NoError(t, os.Symlink(secret, filepath.Join(pub, "absolute.txt")))
	require.NoError(t, os.Symlink(filepath.Join("..", "secret.txt"), filepath.Join(pub, "relative.txt")))

	a := NewExplodedArchive(pub)
	var paths []string
	for _, e := range a.Entries() {
		paths = append(paths, e.Path())
	}
	assert.ElementsMatch(t, []string{"inside.txt", "alias.txt"}, paths)

	// Symbolic links within the directory are followed.
	e, err := a.Entry("alias.txt")
	require.NoError(t, err)
	assert.EqualValues(t, len("inside"), e.Length())
	data, err := e.Read(0, 0)
	require.NoError(t, err)
	assert.Equal(t, "inside", string(data))

	// Not those leading outside of it.
	for _, name := range []string{"absolute.txt", "relative.txt", "../secret.txt"} {
		_, err := a.Entry(name)
		assert.Error(t, err, name)
	}
}

func TestExplodedArchiveAtTheWorkingDirectory(t *testing.T) {
	// Entries were named by cutting the length of the directory and a
	// separator off their path, which isn't how paths under "." start: "ab"
	// was named "", and a one-character name panicked.
	dir := t.TempDir()
	for _, name := range []string{"a", "ab.txt", filepath.Join("sub", "c.txt")} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644))
	}
	t.Chdir(dir)
	a := NewExplodedArchive(".")
	defer a.Close()
	var names []string
	for _, e := range a.Entries() {
		names = append(names, e.Path())
	}
	slices.Sort(names)
	assert.Equal(t, []string{"a", "ab.txt", "sub/c.txt"}, names)
}
