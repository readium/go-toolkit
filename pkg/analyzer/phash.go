package analyzer

// Adapted from github.com/azr/phash v0.2.0, copyright (c) 2020 Adrien Delorme.
// See LICENSE.phash for the MIT license.

import (
	"image"
	"math"
	"slices"

	"github.com/disintegration/imaging"
)

const phashSize = 32
const phashFrequencies = 8

// The reference hash uses frequencies 1 through 8 on both axes. Cache their
// cosine values, preserving the reference expression's floating-point order.
var phashCosines = func() [phashFrequencies][phashSize]float64 {
	var cosines [phashFrequencies][phashSize]float64
	for frequency := range cosines {
		for sample := range cosines[frequency] {
			index := (2*sample + 1) * (frequency + 1)
			cosines[frequency][sample] = math.Cos(float64(index) / float64(2*phashSize) * math.Pi)
		}
	}
	return cosines
}()

// perceptualHash produces the same hash as phash.DTC, computing only the 64
// coefficients it retains instead of all 1024. Keep the pixel traversal and
// summation order: reassociating the DCT can change bits near the median.
func perceptualHash(img image.Image) uint64 {
	if img == nil {
		return 0
	}
	thumbnail := imaging.Resize(img, phashSize, phashSize, imaging.Lanczos)
	var pixels [phashSize * phashSize]float64
	for x := range phashSize {
		for y := range phashSize {
			// NRGBAAt avoids boxing each pixel as color.Color. RGBA preserves
			// the reference's 16-bit, alpha-premultiplied color conversion.
			r, g, b, _ := thumbnail.NRGBAAt(x, y).RGBA()
			pixels[x*phashSize+y] = 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
		}
	}

	var coefficients [phashFrequencies * phashFrequencies]float64
	for u := range phashFrequencies {
		for v := range phashFrequencies {
			var sum float64
			for x := range phashSize {
				for y := range phashSize {
					sum += phashCosines[u][x] * phashCosines[v][y] * pixels[x*phashSize+y]
				}
			}
			// The normalization coefficients are both 1 for nonzero frequencies.
			coefficients[u*phashFrequencies+v] = sum * 0.25
		}
	}

	sorted := coefficients
	slices.Sort(sorted[:])
	median := sorted[len(sorted)/2]
	var hash uint64
	for bit, coefficient := range coefficients {
		if coefficient > median {
			hash |= uint64(1) << bit
		}
	}
	return hash
}
