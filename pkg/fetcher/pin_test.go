package fetcher

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPinnedResourcesOnlyReadTheirFile(t *testing.T) {
	link := manifest.Link{Href: manifest.MustNewHREFFromString("cover.jpg", false)}
	setup := func(t *testing.T) (dir string, fetchers map[string]Fetcher) {
		dir = t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte("cover in the clear"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), []byte("encrypted chapter"), 0o644))
		exploded, err := NewArchiveFetcherFromPath(t.Context(), dir)
		require.NoError(t, err)
		t.Cleanup(exploded.Close)
		return dir, map[string]Fetcher{
			"file":     NewFileFetcher("", dir),
			"exploded": exploded,
			// Transformations pass the pin on.
			"transforming": NewTransformingFetcher(NewFileFetcher("", dir), func(res Resource) Resource {
				return NewTransformingResource(res, false, func(b []byte) []byte { return b })
			}),
		}
	}
	read := func(res Resource) ([]byte, *ResourceError) {
		var buf bytes.Buffer
		if _, rerr := res.Stream(t.Context(), &buf, 0, 0); rerr != nil {
			return nil, rerr
		}
		return buf.Bytes(), nil
	}

	changes := map[string]func(dir string) error{
		"renamed": func(dir string) error {
			return os.Rename(filepath.Join(dir, "chapter.html"), filepath.Join(dir, "cover.jpg"))
		},
		"linked": func(dir string) error {
			if err := os.Remove(filepath.Join(dir, "cover.jpg")); err != nil {
				return err
			}
			return os.Symlink("chapter.html", filepath.Join(dir, "cover.jpg"))
		},
	}
	for change, apply := range changes {
		if change == "linked" && runtime.GOOS == "windows" {
			continue
		}
		dir, fetchers := setup(t)
		pins := make(map[string]Resource)
		for name, f := range fetchers {
			res := f.Get(t.Context(), link)
			fb, ok := res.(FilePinner)
			require.True(t, ok, name)
			info, err := fb.FileInfo()
			require.NoError(t, err, name)
			require.NoError(t, fb.PinFile(info), name)
			// The path of a pinned resource may name another file: consumers
			// opening it would bypass the pin.
			assert.Empty(t, res.File(), name)
			data, rerr := read(res)
			require.Nil(t, rerr, name)
			assert.Equal(t, "cover in the clear", string(data), name)
			pins[name] = res
		}
		require.NoError(t, apply(dir))
		for name, res := range pins {
			// The path of a pinned resource used to be opened again, whatever
			// file it names. Reads now fail, unless they use a file opened
			// before, which is the pinned one.
			for method, data := range map[string]func() ([]byte, *ResourceError){
				"Stream": func() ([]byte, *ResourceError) { return read(res) },
				"Read":   func() ([]byte, *ResourceError) { return res.Read(t.Context(), 0, 0) },
			} {
				got, rerr := data()
				if rerr != nil {
					assert.True(t, errors.Is(rerr, ErrFileChanged), "%s %s %s: %v", change, name, method, rerr)
				} else {
					assert.Equal(t, "cover in the clear", string(got), "%s %s %s", change, name, method)
				}
			}
			res.Close()
		}
		// Streams of files read from their path always open them again.
		_, rerr := read(pins["file"])
		assert.True(t, errors.Is(rerr, ErrFileChanged), change)
	}

	// Files changed in place are still the same file.
	dir, fetchers := setup(t)
	res := fetchers["file"].Get(t.Context(), link)
	info, err := res.(FilePinner).FileInfo()
	require.NoError(t, err)
	require.NoError(t, res.(FilePinner).PinFile(info))
	f, err := os.OpenFile(filepath.Join(dir, "cover.jpg"), os.O_WRONLY|os.O_APPEND, 0)
	require.NoError(t, err)
	_, err = io.WriteString(f, "!")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	data, rerr := read(res)
	require.Nil(t, rerr)
	assert.Equal(t, "cover in the clear!", string(data))

	// Resources which aren't read from a file can't be pinned.
	bytesResource := NewBytesResource(link, func() []byte { return []byte("x") })
	assert.ErrorIs(t, NewTransformingResource(bytesResource, false, func(b []byte) []byte { return b }).PinFile(info), ErrNotFileBacked)
}

type reportingResource struct {
	Resource
	source url.AbsoluteURL
}

func (r reportingResource) Location() url.AbsoluteURL { return r.source }

func TestPinLocationRequiresABinding(t *testing.T) {
	source, err := url.AbsoluteURLFromString("https://example.org/book/chapter")
	require.NoError(t, err)
	base := NewBytesResource(manifest.Link{}, func() []byte { return []byte("chapter") })
	reporting := reportingResource{base, source}
	for _, res := range []Resource{
		reporting,
		ProxyResource{Res: reporting},
		NewLazyResource(func() Resource { return reporting }),
		NewTransformingResource(reporting, false, func(b []byte) []byte { return b }),
	} {
		location, err := PinLocation(res)
		require.Error(t, err)
		assert.Nil(t, location)
	}
	location, err := PinLocation(base)
	assert.NoError(t, err)
	assert.Nil(t, location)
}
