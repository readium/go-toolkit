//go:build !unix

package lcp

import "io/fs"

// fileKey reports that files can't be identified on this platform, whose
// file information doesn't expose what os.SameFile compares.
func fileKey(fs.FileInfo) (any, bool) {
	return nil, false
}
