package pub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

func TestPerResourcePositionsServiceEmptyReadingOrder(t *testing.T) {
	service := PerResourcePositionsService{}
	assert.Equal(t, 0, len(service.Positions()))
}

func TestPerResourcePositionsServiceSingleReadingOrder(t *testing.T) {
	service := PerResourcePositionsService{
		readingOrder: manifest.LinkList{{Href: manifest.MustNewHREFFromString("res", false), MediaType: &mediatype.PNG}},
	}

	assert.Equal(t, []manifest.Locator{{
		Href:      url.MustURLFromString("res"),
		MediaType: mediatype.PNG,
		Locations: manifest.Locations{
			Position:         extensions.Pointer(uint(1)),
			TotalProgression: extensions.Pointer(float64(0.0)),
		},
	}}, service.Positions())
}

func TestPerResourcePositionsServiceMultiReadingOrder(t *testing.T) {
	service := PerResourcePositionsService{
		readingOrder: manifest.LinkList{
			{Href: manifest.MustNewHREFFromString("res", false)},
			{Href: manifest.MustNewHREFFromString("chap1", false), MediaType: &mediatype.PNG},
			{Href: manifest.MustNewHREFFromString("chap2", false), MediaType: &mediatype.PNG, Title: "Chapter 2"},
		},
		fallbackMediaType: mediatype.Binary,
	}

	assert.Equal(t, []manifest.Locator{
		{
			Href:      url.MustURLFromString("res"),
			MediaType: mediatype.Binary,
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(1)),
				TotalProgression: extensions.Pointer(float64(0.0)),
			},
		},
		{
			Href:      url.MustURLFromString("chap1"),
			MediaType: mediatype.PNG,
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(2)),
				TotalProgression: extensions.Pointer(float64(1.0 / 3.0)),
			},
		},
		{
			Href:      url.MustURLFromString("chap2"),
			MediaType: mediatype.PNG,
			Title:     "Chapter 2",
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(3)),
				TotalProgression: extensions.Pointer(float64(2.0 / 3.0)),
			},
		},
	}, service.Positions())
}

func TestPerResourcePositionsServiceMediaTypeFallback(t *testing.T) {
	service := PerResourcePositionsService{
		readingOrder:      manifest.LinkList{{Href: manifest.MustNewHREFFromString("res", false)}},
		fallbackMediaType: mediatype.MustNewOfString("image/*"),
	}

	mt, _ := mediatype.NewOfString("image/*")
	assert.Equal(t, []manifest.Locator{{
		Href:      url.MustURLFromString("res"),
		MediaType: mt,
		Locations: manifest.Locations{
			Position:         extensions.Pointer(uint(1)),
			TotalProgression: extensions.Pointer(float64(0.0)),
		},
	}}, service.Positions())
}
