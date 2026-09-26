package analyzer

import (
	"bytes"
	"image"
	"os"
	"testing"
	"testing/fstest"

	"github.com/azr/phash"
	"github.com/bbrks/go-blurhash"
	"github.com/disintegration/imaging"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

var benchmarkHashImplementations = []struct {
	name string
	hash func(image.Image) uint64
}{
	{"Reference", phash.DTC},
	{"Optimized", perceptualHash},
}

func benchmarkImageFixture(b *testing.B) ([]byte, image.Image) {
	b.Helper()
	data, err := os.ReadFile("testdata/catsink.jpg")
	if err != nil {
		b.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	return data, img
}

// BenchmarkImageAnalysis isolates each stage. The hash comparison includes its
// internal 32x32 resize; fixture decoding and the initial 128-pixel resize happen
// before timing. Reference uses the pinned phash dependency's original full DCT.
func BenchmarkImageAnalysis(b *testing.B) {
	data, img := benchmarkImageFixture(b)
	thumbnail := imaging.Resize(img, 128, 0, imaging.Lanczos)
	b.Run("DecodeJPEG", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Resize128", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			imaging.Resize(img, 128, 0, imaging.Lanczos)
		}
	})
	b.Run("PhashDCT", func(b *testing.B) {
		want := phash.DTC(thumbnail)
		for _, impl := range benchmarkHashImplementations {
			b.Run(impl.name, func(b *testing.B) {
				b.ReportAllocs()
				var hash uint64
				for b.Loop() {
					hash = impl.hash(thumbnail)
				}
				if hash != want {
					b.Fatalf("hash = %016x, reference = %016x", hash, want)
				}
			})
		}
	})
	b.Run("BlurHash5x5", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := blurhash.Encode(5, 5, thumbnail); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkVisualHashPipeline compares the original and optimized hash within
// the same decode/resize/hash pipeline. Every iteration decodes the JPEG, resizes
// it to 128 pixels, and computes the selected hashes. File I/O and manifest work
// are excluded; BenchmarkInspectImage below measures the full current API.
func BenchmarkVisualHashPipeline(b *testing.B) {
	data, img := benchmarkImageFixture(b)
	thumbnail := imaging.Resize(img, 128, 0, imaging.Lanczos)
	wantHash := phash.DTC(thumbnail)
	wantBlur, err := blurhash.Encode(5, 5, thumbnail)
	if err != nil {
		b.Fatal(err)
	}
	for _, mode := range []string{"Phash", "Both"} {
		b.Run(mode, func(b *testing.B) {
			for _, impl := range benchmarkHashImplementations {
				b.Run(impl.name, func(b *testing.B) {
					b.ReportAllocs()
					var hash uint64
					var blur string
					for b.Loop() {
						decoded, _, err := image.Decode(bytes.NewReader(data))
						if err != nil {
							b.Fatal(err)
						}
						resized := imaging.Resize(decoded, 128, 0, imaging.Lanczos)
						hash = impl.hash(resized)
						if mode == "Both" {
							blur, err = blurhash.Encode(5, 5, resized)
							if err != nil {
								b.Fatal(err)
							}
						}
					}
					if hash != wantHash || (mode == "Both" && blur != wantBlur) {
						b.Fatal("hashes differ from the reference pipeline")
					}
				})
			}
		})
	}
}

// BenchmarkInspectImage measures the current public API using an in-memory file
// system to keep disk latency out of the result. It includes image decoding,
// resizing, visual hashing, and manifest property construction.
func BenchmarkInspectImage(b *testing.B) {
	data, err := os.ReadFile("testdata/catsink.jpg")
	if err != nil {
		b.Fatal(err)
	}
	fs := fstest.MapFS{"catsink.jpg": &fstest.MapFile{Data: data}}
	link := manifest.Link{
		Href:      manifest.MustNewHREFFromString("catsink.jpg", false),
		MediaType: &mediatype.JPEG,
	}
	for _, tc := range []struct {
		name       string
		algorithms []manifest.HashAlgorithm
	}{
		{"Phash", []manifest.HashAlgorithm{manifest.HashAlgorithmPhashDCT}},
		{"BlurHash", []manifest.HashAlgorithm{blurHashAlgorithm}},
		{"Both", []manifest.HashAlgorithm{manifest.HashAlgorithmPhashDCT, blurHashAlgorithm}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := InspectImage(fs, link, tc.algorithms); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
