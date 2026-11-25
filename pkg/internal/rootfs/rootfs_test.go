package rootfs

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tree creates a directory "pub" holding a file, a symbolic link to it, a
// symbolic link to a file outside of "pub", and a subdirectory. It returns
// the path of "pub".
func tree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644))
	pub := filepath.Join(dir, "pub")
	require.NoError(t, os.MkdirAll(filepath.Join(pub, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pub, "sub", "inside.txt"), []byte("inside"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join("sub", "inside.txt"), filepath.Join(pub, "alias.txt")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "secret.txt"), filepath.Join(pub, "absolute.txt")))
	require.NoError(t, os.Symlink(filepath.Join("..", "secret.txt"), filepath.Join(pub, "relative.txt")))
	return pub
}

func read(t *testing.T, dir, name string) (string, error) {
	t.Helper()
	f, err := Open(dir, name)
	if err != nil {
		return "", err
	}
	defer func() { assert.NoError(t, f.Close()) }()
	data, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(data), nil
}

func TestOpen(t *testing.T) {
	pub := tree(t)

	data, err := read(t, pub, filepath.Join("sub", "inside.txt"))
	require.NoError(t, err)
	assert.Equal(t, "inside", data)

	// Symbolic links within the directory are followed.
	data, err = read(t, pub, "alias.txt")
	require.NoError(t, err)
	assert.Equal(t, "inside", data)
	assert.True(t, IsRegular(pub, "alias.txt"))

	// Not outside of it, nor any path.
	for _, name := range []string{"absolute.txt", "relative.txt", filepath.Join("..", "secret.txt"), filepath.Join("sub", "..", "..", "secret.txt")} {
		_, err := read(t, pub, name)
		assert.Error(t, err, name)
		assert.False(t, IsRegular(pub, name), name)
	}

	_, err = read(t, pub, "missing.txt")
	assert.ErrorIs(t, err, os.ErrNotExist)

	_, err = read(t, pub, "sub")
	assert.ErrorIs(t, err, ErrNotRegular)
	assert.False(t, IsRegular(pub, "sub"))
}
