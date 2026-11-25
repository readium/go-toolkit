//go:build unix

package lcp

import (
	"io/fs"
	"syscall"
)

// fileKey returns the device and inode of a file, which os.SameFile compares.
func fileKey(info fs.FileInfo) (any, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, false
	}
	return [2]uint64{uint64(st.Dev), uint64(st.Ino)}, true //nolint:unconvert // Dev and Ino types vary across platforms.
}
