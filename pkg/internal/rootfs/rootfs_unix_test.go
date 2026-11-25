//go:build unix

package rootfs

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRefusesNamedPipes(t *testing.T) {
	pub := tree(t)
	require.NoError(t, syscall.Mkfifo(filepath.Join(pub, "fifo"), 0o644))

	// Opening the pipe would block forever.
	_, err := Open(pub, "fifo")
	assert.ErrorIs(t, err, ErrNotRegular)
	assert.False(t, IsRegular(pub, "fifo"))
}
