//go:build unix

package lcp

import (
	"io/fs"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSameFilesGroupsByIdentity(t *testing.T) {
	// Files identified by device and inode are grouped without being
	// compared in pairs, however many share a size and modification time.
	ids := make([]fs.FileInfo, 20000)
	for i := range ids {
		ids[i] = fakeFileInfo{size: 100, mtime: mustTime("2020-01-01T00:00:00Z"), sys: &syscall.Stat_t{Ino: uint64(i + 1)}}
	}
	ids[17] = fakeFileInfo{size: 100, mtime: mustTime("2020-01-01T00:00:00Z"), sys: &syscall.Stat_t{Ino: 5}}
	limit := maxFileComparisons
	t.Cleanup(func() { maxFileComparisons = limit })
	maxFileComparisons = 0
	groups, err := sameFiles(t.Context(), ids)
	require.NoError(t, err)
	assert.Equal(t, [][]int{{4, 17}}, groups)
}
