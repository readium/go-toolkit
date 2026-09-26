package fetcher

import (
	"sync"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Properties are mutable extension metadata. Calls on one resource must keep
// returning the same maps, including when their first access is concurrent.
func TestArchiveResourcePropertiesConcurrent(t *testing.T) {
	f, err := NewArchiveFetcherFromPath(t.Context(), "./testdata/epub.epub")
	require.NoError(t, err)
	t.Cleanup(f.Close)
	r := f.Get(t.Context(), manifest.Link{
		Href: manifest.MustNewHREFFromString("EPUB/css/epub.css", false),
	})
	t.Cleanup(r.Close)

	const readers = 32
	properties := make([]manifest.Properties, readers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range properties {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			properties[i] = r.Properties()
		}()
	}
	close(start)
	wg.Wait()

	const archiveKey = "https://readium.org/webpub-manifest/properties#archive"
	for _, p := range properties {
		require.Equal(t, map[string]interface{}{
			"entryLength":       uint64(595),
			"isEntryCompressed": true,
		}, p[archiveKey])
	}

	// Mutate only after the concurrent readers finish: callers still need to
	// synchronize their own modifications to the returned maps.
	properties[0]["custom"] = "extension"
	properties[0][archiveKey].(map[string]interface{})["custom"] = "nested extension"
	for _, p := range properties {
		assert.Equal(t, "extension", p["custom"])
		assert.Equal(t, "nested extension", p[archiveKey].(map[string]interface{})["custom"])
	}
	assert.Equal(t, "extension", r.Properties()["custom"])
}

func TestArchiveResourcePropertiesIndependent(t *testing.T) {
	f, err := NewArchiveFetcherFromPath(t.Context(), "./testdata/epub.epub")
	require.NoError(t, err)
	t.Cleanup(f.Close)
	link := manifest.Link{Href: manifest.MustNewHREFFromString("mimetype", false)}
	first := f.Get(t.Context(), link)
	second := f.Get(t.Context(), link)
	t.Cleanup(first.Close)
	t.Cleanup(second.Close)

	const archiveKey = "https://readium.org/webpub-manifest/properties#archive"
	first.Properties()["custom"] = "extension"
	first.Properties()[archiveKey].(map[string]interface{})["entryLength"] = uint64(1)
	assert.Equal(t, manifest.Properties{
		archiveKey: map[string]interface{}{
			"entryLength":       uint64(20),
			"isEntryCompressed": false,
		},
	}, second.Properties())
}

// The archive stays open and its entry cache is warmed before timing. Each
// iteration gets a new resource, as happens when parsing metadata or generating
// positions, and either reads its length or asks for archive properties.
func BenchmarkArchiveFetcherGet(b *testing.B) {
	for _, entry := range []struct {
		name string
		path string
	}{
		{"Stored", "mimetype"},
		{"Deflated", "EPUB/css/epub.css"},
	} {
		b.Run(entry.name, func(b *testing.B) {
			f, err := NewArchiveFetcherFromPath(b.Context(), "./testdata/epub.epub")
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(f.Close)
			link := manifest.Link{Href: manifest.MustNewHREFFromString(entry.path, false)}
			warm := f.Get(b.Context(), link)
			length, rerr := warm.Length(b.Context())
			if rerr != nil {
				b.Fatal(rerr)
			}
			warm.Close()

			b.Run("Length", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					r := f.Get(b.Context(), link)
					n, rerr := r.Length(b.Context())
					r.Close()
					if rerr != nil || n != length {
						b.Fatalf("Length = %d, %v; want %d, nil", n, rerr, length)
					}
				}
			})
			b.Run("Properties", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					r := f.Get(b.Context(), link)
					p := r.Properties()
					r.Close()
					if len(p) != 1 {
						b.Fatalf("expected archive properties, got %v", p)
					}
				}
			})
		})
	}
}
