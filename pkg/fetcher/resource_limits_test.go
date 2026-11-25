package fetcher

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRangeReadsAreClampedBeforeAllocation(t *testing.T) {
	content := []byte("small cover")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), content, 0o644))
	zipPath := filepath.Join(t.TempDir(), "package.zip")
	zf, err := os.Create(zipPath)
	require.NoError(t, err)
	zw := zip.NewWriter(zf)
	for name, method := range map[string]uint16{"stored.jpg": zip.Store, "deflated.jpg": zip.Deflate} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, zf.Close())

	zipFetcher, err := NewArchiveFetcherFromPath(t.Context(), zipPath)
	require.NoError(t, err)
	defer zipFetcher.Close()
	exploded, err := NewArchiveFetcherFromPath(t.Context(), dir)
	require.NoError(t, err)
	defer exploded.Close()
	link := func(href string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}
	}
	resources := map[string]Resource{
		"stored ZIP entry":   zipFetcher.Get(t.Context(), link("stored.jpg")),
		"deflated ZIP entry": zipFetcher.Get(t.Context(), link("deflated.jpg")),
		"exploded file":      exploded.Get(t.Context(), link("cover.jpg")),
		"file":               NewFileResource(link("cover.jpg"), filepath.Join(dir, "cover.jpg")),
	}
	for name, res := range resources {
		// The range was allocated whole before being clamped: MaxInt64
		// panicked, and smaller ends could exhaust the memory.
		for _, end := range []int64{math.MaxInt64, math.MaxInt64 - 1, 1 << 40} {
			data, rerr := res.Read(t.Context(), 1, end)
			require.Nil(t, rerr, name)
			assert.Equal(t, content[1:], data, name)
		}
		data, rerr := res.Read(t.Context(), int64(len(content)), 1<<40)
		require.Nil(t, rerr, name)
		assert.Empty(t, data, name)
		data, rerr = res.Read(t.Context(), 2, 4)
		require.Nil(t, rerr, name)
		assert.Equal(t, content[2:5], data, name)
		res.Close()
	}
}

func TestReadResourceUpTo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.xml")
	require.NoError(t, os.WriteFile(path, []byte("<doc>content</doc>"), 0o644))
	res := NewFileResource(manifest.Link{Href: manifest.MustNewHREFFromString("doc.xml", false)}, path)

	data, rerr := ReadResourceUpTo(t.Context(), res, 18)
	require.Nil(t, rerr)
	assert.Equal(t, "<doc>content</doc>", string(data))
	doc, rerr := ReadResourceAsXMLUpTo(t.Context(), res, 18)
	require.Nil(t, rerr)
	assert.Equal(t, "content", doc.SelectElement("doc").InnerText())

	for _, read := range []func() *ResourceError{
		func() *ResourceError { _, rerr := ReadResourceUpTo(t.Context(), res, 17); return rerr },
		func() *ResourceError { _, rerr := ReadResourceAsXMLUpTo(t.Context(), res, 17); return rerr },
	} {
		rerr := read()
		require.NotNil(t, rerr)
		assert.Equal(t, CodeInsufficientStorage, rerr.Code)
		assert.True(t, errors.Is(rerr, ErrResourceTooLarge))
		assert.ErrorContains(t, rerr, "doc.xml is larger than 17 B")
	}
}

func TestReadResourceAsJSONUpTo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"title": "Test"}`), 0o644))
	res := NewFileResource(manifest.Link{Href: manifest.MustNewHREFFromString("manifest.json", false)}, path)

	object, rerr := ReadResourceAsJSONUpTo(t.Context(), res, 17)
	require.Nil(t, rerr)
	assert.Equal(t, map[string]any{"title": "Test"}, object)
	_, rerr = ReadResourceAsJSONUpTo(t.Context(), res, 16)
	require.NotNil(t, rerr)
	assert.True(t, errors.Is(rerr, ErrResourceTooLarge))
}

func TestNegativeRanges(t *testing.T) {
	content := []byte("small cover")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), content, 0o644))
	zipPath := filepath.Join(t.TempDir(), "package.zip")
	zf, err := os.Create(zipPath)
	require.NoError(t, err)
	zw := zip.NewWriter(zf)
	for name, method := range map[string]uint16{"stored.jpg": zip.Store, "deflated.jpg": zip.Deflate} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		require.NoError(t, err)
		_, err = w.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, zf.Close())
	zipFetcher, err := NewArchiveFetcherFromPath(t.Context(), zipPath)
	require.NoError(t, err)
	defer zipFetcher.Close()
	exploded, err := NewArchiveFetcherFromPath(t.Context(), dir)
	require.NoError(t, err)
	defer exploded.Close()
	link := func(href string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}
	}
	newResources := func() map[string]Resource {
		bytesResource := NewBytesResource(link("cover.jpg"), func() []byte { return content })
		return map[string]Resource{
			"stored ZIP entry":   zipFetcher.Get(t.Context(), link("stored.jpg")),
			"deflated ZIP entry": zipFetcher.Get(t.Context(), link("deflated.jpg")),
			"exploded file":      exploded.Get(t.Context(), link("cover.jpg")),
			"file":               NewFileResource(link("cover.jpg"), filepath.Join(dir, "cover.jpg")),
			"bytes":              bytesResource,
			"transforming":       NewTransformingResource(NewBytesResource(link("cover.jpg"), func() []byte { return content }), false, func(b []byte) []byte { return b }),
		}
	}
	for name, res := range newResources() {
		// Negative ends panicked, or allocated a negative length.
		for _, r := range [][2]int64{{-10, -5}, {-5, -1}, {-1, -1}} {
			_, rerr := res.Read(t.Context(), r[0], r[1])
			require.NotNil(t, rerr, "%s %v", name, r)
			assert.Equal(t, CodeRequestedRangeNotSatisfiable, rerr.Code, "%s %v", name, r)
			_, rerr = res.Stream(t.Context(), io.Discard, r[0], r[1])
			require.NotNil(t, rerr, "%s %v", name, r)
		}
		// Negative starts are the start of the resource.
		data, rerr := res.Read(t.Context(), -5, 3)
		require.Nil(t, rerr, name)
		assert.Equal(t, content[:4], data, name)
		var buf bytes.Buffer
		_, rerr = res.Stream(t.Context(), &buf, -5, 3)
		require.Nil(t, rerr, name)
		assert.Equal(t, content[:4], buf.Bytes(), name)
		// Ends beyond the resource are clamped, also when streaming.
		buf.Reset()
		_, rerr = res.Stream(t.Context(), &buf, 1, 1<<40)
		require.Nil(t, rerr, name)
		assert.Equal(t, content[1:], buf.Bytes(), name)
		buf.Reset()
		_, rerr = res.Stream(t.Context(), &buf, 0, math.MaxInt64)
		require.Nil(t, rerr, name)
		assert.Equal(t, content, buf.Bytes(), name)
		res.Close()
	}
}
