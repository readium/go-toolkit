package analyzer

import (
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"os"
	"testing"

	"github.com/azr/phash"
	"github.com/disintegration/imaging"
)

func TestPerceptualHashCompatibility(t *testing.T) {
	check := func(t *testing.T, img image.Image) {
		t.Helper()
		want := phash.DTC(img)
		if got := perceptualHash(img); got != want {
			t.Fatalf("hash = %016x, reference = %016x", got, want)
		}
	}
	t.Run("nil", func(t *testing.T) { check(t, nil) })
	for _, name := range []string{"catsink.jpg", "frame1.png", "frame2.png", "animated.png"} {
		t.Run(name, func(t *testing.T) {
			f, err := os.Open("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			img, _, err := image.Decode(f)
			if err != nil {
				t.Fatal(err)
			}
			check(t, img)
			// InspectImage first resizes wide images to 128 pixels.
			check(t, imaging.Resize(img, 128, 0, imaging.Lanczos))
		})
	}

	// Constant and nearly constant images exercise coefficients near the median,
	// where even small floating-point changes could flip a hash bit.
	for _, c := range []color.NRGBA{
		{0, 0, 0, 255}, {255, 255, 255, 255}, {127, 127, 127, 255},
		{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255},
		{200, 80, 35, 0}, {200, 80, 35, 1}, {200, 80, 35, 128},
	} {
		t.Run(fmt.Sprintf("solid-%v", c), func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
			for y := 0; y < 32; y++ {
				for x := 0; x < 32; x++ {
					img.SetNRGBA(x, y, c)
				}
			}
			check(t, img)
			img.SetNRGBA(13, 19, color.NRGBA{c.R ^ 1, c.G, c.B, c.A})
			check(t, img)
		})
	}

	for seed := int64(0); seed < 32; seed++ {
		t.Run(fmt.Sprintf("random-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			width, height := 1+rng.Intn(160), 1+rng.Intn(160)
			bounds := image.Rect(-3, 5, width-3, height+5)
			nrgba := image.NewNRGBA(bounds)
			rgba := image.NewRGBA(bounds)
			gray := image.NewGray(bounds)
			nrgba64 := image.NewNRGBA64(bounds)
			paletted := image.NewPaletted(bounds, color.Palette{color.Transparent, color.Black, color.White, color.NRGBA{255, 40, 100, 128}})
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					c := color.NRGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256))}
					nrgba.SetNRGBA(x, y, c)
					rgba.Set(x, y, c)
					gray.Set(x, y, c)
					nrgba64.Set(x, y, c)
					paletted.SetColorIndex(x, y, uint8(rng.Intn(4)))
				}
			}
			for _, img := range []image.Image{nrgba, rgba, gray, nrgba64, paletted} {
				check(t, img)
			}
			if width > 2 && height > 2 {
				// Subimages keep the parent's stride and have nonzero origins.
				check(t, nrgba.SubImage(bounds.Inset(1)))
				check(t, rgba.SubImage(bounds.Inset(1)))
			}
		})
	}
}
