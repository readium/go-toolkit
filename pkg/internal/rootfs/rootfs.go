// Package rootfs reads the files of a directory tree without ever leaving
// it, whatever the symbolic links it holds, using [os.Root].
//
// Publications exploded on the file system are untrusted input: a symbolic
// link inside one may point to any file of the system, which must not be
// served as a resource of the publication.
package rootfs

import (
	"errors"
	"io/fs"
	"os"
)

// ErrNotRegular is returned for files which are not regular files, such as
// directories, named pipes or devices.
var ErrNotRegular = errors.New("not a regular file")

// Stat returns information about the named file of dir, following symbolic
// links as long as they don't lead outside of dir. name is relative to dir.
func Stat(dir, name string) (fs.FileInfo, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.Stat(name)
}

// IsRegular reports whether the named file of dir is a regular file, or a
// symbolic link to a regular file of dir.
func IsRegular(dir, name string) bool {
	fi, err := Stat(dir, name)
	return err == nil && fi.Mode().IsRegular()
}

// Open opens the named regular file of dir for reading, following symbolic
// links as long as they don't lead outside of dir. name is relative to dir.
//
// Files which are not regular files are refused with [ErrNotRegular], before
// being opened: opening a named pipe blocks until something writes to it.
func Open(dir, name string) (*os.File, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	// Files opened through the root stay open after it is closed.
	defer func() { _ = root.Close() }()

	fi, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, &fs.PathError{Op: "open", Path: name, Err: ErrNotRegular}
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	// The file may have been replaced since it was checked.
	if fi, err = f.Stat(); err != nil || !fi.Mode().IsRegular() {
		_ = f.Close()
		if err == nil {
			err = &fs.PathError{Op: "open", Path: name, Err: ErrNotRegular}
		}
		return nil, err
	}
	return f, nil
}
