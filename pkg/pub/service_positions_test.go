package pub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func TestPerResourcePositionsServiceEmptyReadingOrder(t *testing.T) {
	service := PerResourcePositionsService{}
	assert.Equal(t, 0, len(service.Positions()))
}

func TestPerResourcePositionsServiceSingleReadingOrder(t *testing.T) {
	service := PerResourcePositionsService{
		readingOrder: manifest.LinkList{{Href: "res", Type: "image/png"}},
	}

	assert.Equal(t, []manifest.Locator{{
		Href: "res",
		Type: "image/png",
		Locations: manifest.Locations{
			Position:         extensions.Pointer(uint(1)),
			TotalProgression: extensions.Pointer(float64(0.0)),
		},
	}}, service.Positions())
}

func TestPerResourcePositionsServiceMultiReadingOrder(t *testing.T) {
	service := PerResourcePositionsService{
		readingOrder: manifest.LinkList{
			{Href: "res"},
			{Href: "chap1", Type: "image/png"},
			{Href: "chap2", Type: "image/png", Title: "Chapter 2"},
		},
	}

	assert.Equal(t, []manifest.Locator{
		{
			Href: "res",
			Type: "",
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(1)),
				TotalProgression: extensions.Pointer(float64(0.0)),
			},
		},
		{
			Href: "chap1",
			Type: "image/png",
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(2)),
				TotalProgression: extensions.Pointer(float64(1.0 / 3.0)),
			},
		},
		{
			Href:  "chap2",
			Type:  "image/png",
			Title: "Chapter 2",
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(3)),
				TotalProgression: extensions.Pointer(float64(2.0 / 3.0)),
			},
		},
	}, service.Positions())
}

func TestPerResourcePositionsServiceMediaTypeFallback(t *testing.T) {
	service := PerResourcePositionsService{
		readingOrder:      manifest.LinkList{{Href: "res"}},
		fallbackMediaType: "image/*",
	}

	assert.Equal(t, []manifest.Locator{{
		Href: "res",
		Type: "image/*",
		Locations: manifest.Locations{
			Position:         extensions.Pointer(uint(1)),
			TotalProgression: extensions.Pointer(float64(0.0)),
		},
	}}, service.Positions())
}
