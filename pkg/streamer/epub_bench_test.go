package streamer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// BenchmarkEPUBOpen measures the complete local opening path, including archive
// creation, EPUB parsing, service construction, and publication cleanup. Set
// EPUB_BENCH_DIR to use another corpus. The default corpus is optional and is not
// needed to run tests on a checkout without local publications.
//
// KnownMediaType avoids media-type detection. SniffedMediaType creates a fresh asset
// on every iteration so the asset's media-type cache cannot hide sniffing costs.
// For .epub files, detection trusts the extension without inspecting ZIP content.
// WithPositions uses a known media type and also computes the complete default
// position list once for each freshly opened publication. This uses archive-entry
// sizes for reflowable content; it does not parse spine HTML or media overlays.
//
// Select individual modes and books with -bench, for example:
//
//	go test ./pkg/streamer -run '^$' -bench 'BenchmarkEPUBOpen/KnownMediaType/moby-dick' -benchmem
func BenchmarkEPUBOpen(b *testing.B) {
	files, pubsDir := epubBenchmarkFiles(b)
	for _, mode := range []string{"KnownMediaType", "SniffedMediaType", "WithPositions"} {
		b.Run(mode, func(b *testing.B) {
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
					s := New(Config{})
					ctx := b.Context()
					var positionCount int
					b.ReportAllocs()
					for b.Loop() {
						var a *asset.FileAsset
						if mode == "SniffedMediaType" {
							a = asset.File(uri)
						} else {
							a = asset.FileWithMediaType(uri, &mediatype.EPUB)
						}
						publication, err := s.Open(ctx, a, "")
						if err != nil {
							b.Fatal(err)
						}
						if mode == "WithPositions" {
							positionCount += len(publication.Positions(ctx))
						}
						publication.Close()
					}
					if mode == "WithPositions" {
						b.ReportMetric(float64(positionCount)/float64(b.N), "positions/op")
					}
				})
			}
		})
	}
}

func epubBenchmarkFiles(b *testing.B) ([]string, string) {
	b.Helper()
	pubsDir := os.Getenv("EPUB_BENCH_DIR")
	if pubsDir == "" {
		pubsDir = "../../publications"
	}
	var files []string
	err := filepath.WalkDir(pubsDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".epub") {
			files = append(files, path)
		}
		return nil
	})
	if os.IsNotExist(err) && os.Getenv("EPUB_BENCH_DIR") == "" {
		b.Skip("local EPUB corpus not found; set EPUB_BENCH_DIR")
	}
	if err != nil {
		b.Fatal(err)
	}
	if len(files) == 0 {
		b.Skip("no EPUBs in corpus")
	}
	return files, pubsDir
}
