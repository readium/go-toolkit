package epub

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
)

// BenchmarkParser measures a fresh archive open and EPUB parse for each book in
// /publications. Set EPUB_BENCH_DIR to use another local corpus. Each operation
// closes its fetcher; archive-entry and publication caches cannot leak across
// iterations. This does not read spine content or invoke lazy services.
func BenchmarkParser(b *testing.B) {
	files, pubsDir := epubBenchmarkFiles(b)
	for _, path := range files {
		name, err := filepath.Rel(pubsDir, path)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.ToSlash(name), func(b *testing.B) {
			p := NewParser(nil)
			a := fakeEPUBAsset{name: filepath.Base(path)}
			ctx := b.Context()
			b.ReportAllocs()
			for b.Loop() {
				f, err := fetcher.NewArchiveFetcherFromPath(ctx, path)
				if err != nil {
					b.Fatal(err)
				}
				builder, err := p.Parse(ctx, a, f)
				f.Close()
				if err != nil {
					b.Fatal(err)
				}
				if builder == nil {
					b.Fatal("EPUB was not recognized")
				}
			}
		})
	}
}

func epubBenchmarkFiles(b *testing.B) ([]string, string) {
	b.Helper()
	pubsDir := os.Getenv("EPUB_BENCH_DIR")
	if pubsDir == "" {
		pubsDir = "../../../publications"
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
