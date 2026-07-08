package epub

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/stretchr/testify/require"
)

func BenchmarkParser(b *testing.B) {
	pubsDir := "../../../publications"
	files, err := os.ReadDir(pubsDir)
	if err != nil {
		b.Skip("publications directory not found")
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".epub" {
			continue
		}

		b.Run(file.Name(), func(b *testing.B) {
			path := filepath.Join(pubsDir, file.Name())

			parser := NewParser(nil)
			asset := fakeEPUBAsset{name: file.Name()}
			for i := 0; i < b.N; i++ {
				// We recreate fetcher in each iteration because in real world we parse from fresh fetcher.
				f, err := fetcher.NewArchiveFetcherFromPath(context.Background(), path)
				require.NoError(b, err)
				func() {
					defer f.Close()
					builder, err := parser.Parse(context.Background(), asset, f)
					require.NoError(b, err)
					require.NotNil(b, builder)
				}()
			}
		})
	}
}
