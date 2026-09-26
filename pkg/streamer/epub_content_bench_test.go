package streamer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation/converter"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// BenchmarkEPUBContent measures the lazy content work excluded by
// BenchmarkEPUBOpen. For each book it selects the largest declared HTML or SMIL
// resource by uncompressed length, outside the timer. Each iteration fetches,
// decompresses, parses, and converts that resource to guided navigation again.
// It bypasses publication service caches and closes the resource each time.
//
// The archive remains open between iterations, matching repeated content access
// within one publication. Publication opening, resource selection, and closing
// are excluded. EPUB_BENCH_DIR selects an optional local corpus as for
// BenchmarkEPUBOpen. Books without a matching resource are skipped.
func BenchmarkEPUBContent(b *testing.B) {
	files, pubsDir := epubBenchmarkFiles(b)
	for _, kind := range []string{"HTML", "SMIL"} {
		b.Run(kind, func(b *testing.B) {
			for _, path := range files {
				name, err := filepath.Rel(pubsDir, path)
				if err != nil {
					b.Fatal(err)
				}
				b.Run(filepath.ToSlash(name), func(b *testing.B) {
					uri, err := url.FromFilepath(path)
					if err != nil {
						b.Fatal(err)
					}
					ctx := b.Context()
					s := New(Config{})
					publication, err := s.Open(ctx, asset.FileWithMediaType(uri, &mediatype.EPUB), "")
					if err != nil {
						b.Fatal(err)
					}
					b.Cleanup(publication.Close)
					link, length := largestEPUBBenchmarkResource(b, ctx, publication, kind)
					locator := manifest.Locator{
						Href:      link.URL(nil, nil),
						MediaType: *link.MediaType,
						Title:     link.Title,
					}
					b.ReportAllocs()
					b.SetBytes(length)
					for b.Loop() {
						resource := publication.Fetcher.Get(ctx, link)
						if kind == "HTML" {
							document, err := converter.Do(ctx, resource, locator)
							resource.Close()
							if err != nil {
								b.Fatalf("%s: %v", link.Href.String(), err)
							}
							if document == nil {
								b.Fatal("HTML conversion returned no document")
							}
						} else {
							node, rerr := fetcher.ReadResourceAsXML(ctx, resource)
							resource.Close()
							if rerr != nil {
								b.Fatalf("%s: %v", link.Href.String(), rerr)
							}
							document, err := epub.ParseSMILDocument(node, locator.Href)
							if err != nil {
								b.Fatalf("%s: %v", link.Href.String(), err)
							}
							if document == nil {
								b.Fatal("SMIL conversion returned no document")
							}
						}
					}
					b.ReportMetric(float64(length), "input-bytes/op")
				})
			}
		})
	}
}

func largestEPUBBenchmarkResource(b *testing.B, ctx context.Context, publication *pub.Publication, kind string) (manifest.Link, int64) {
	b.Helper()
	var largest manifest.Link
	var maxLength int64 = -1
	for _, links := range []manifest.LinkList{publication.Manifest.ReadingOrder, publication.Manifest.Resources} {
		for _, link := range links {
			if link.MediaType == nil {
				continue
			}
			matches := kind == "HTML" && link.MediaType.IsHTML() || kind == "SMIL" && link.MediaType.Equal(&mediatype.SMIL)
			if !matches {
				continue
			}
			resource := publication.Fetcher.Get(ctx, link)
			length, err := resource.Length(ctx)
			resource.Close()
			if err != nil {
				b.Fatalf("reading %s length: %v", link.Href.String(), err)
			}
			if length > maxLength || length == maxLength && link.Href.String() < largest.Href.String() {
				largest, maxLength = link, length
			}
		}
	}
	if maxLength < 0 {
		b.Skipf("publication declares no %s resources", kind)
	}
	return largest, maxLength
}
