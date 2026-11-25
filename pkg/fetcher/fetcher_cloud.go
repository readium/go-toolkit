//go:build !readium_nocloud

package fetcher

import (
	"errors"
	"path"
	"strings"
)

// Reports whether the slash-separated object key [key] stays within [root]: it is
// [root] itself or sits below it. Used by the object-store fetchers to keep a `..`
// in an HREF from addressing objects outside the fetcher's prefix, the way
// [FileFetcher] contains resources within its directory. An empty [root] is the
// bucket root, which only forbids upward (`..`) escapes.
func keyWithinRoot(root, key string) bool {
	root = path.Clean(root)
	key = path.Clean(key)
	if root == "." || root == "/" {
		return key != ".." && !strings.HasPrefix(key, "../")
	}
	return key == root || strings.HasPrefix(key, root+"/")
}

// listingPrefix returns the prefix of the names of the objects of a cloud
// storage "folder": the key followed by a slash, or nothing for the root of
// the bucket.
func listingPrefix(key string) string {
	if key == "" || strings.HasSuffix(key, "/") {
		return key
	}
	return key + "/"
}

// objectPath returns the path of a cloud storage object relative to the
// prefix of its listing, and false for a folder placeholder, whose name ends
// with a slash, or the object named after the prefix itself. Empty objects
// are files like any other. A name with empty, "." or ".." segments, such as
// "images//cover.jpg", is an error: it can't be a path of the package, and
// cleaned, it would be the path of another object.
func objectPath(name, prefix string) (string, bool, error) {
	rel := strings.TrimPrefix(name, prefix)
	if rel == "" || strings.HasSuffix(rel, "/") {
		return "", false, nil
	}
	if strings.HasPrefix(rel, "/") || path.Clean(rel) != rel {
		return "", false, errors.New("the object " + name + " can't be a path of the package")
	}
	return rel, true, nil
}

// maxListedObjects bounds the objects a cloud storage listing holds in
// memory: a prefix holding more is an error, rather than listed partially,
// which would make the listing silently incomplete.
const maxListedObjects = 100_000
