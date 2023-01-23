package pub

import (
	"encoding/json"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
)

var PositionsLink = manifest.Link{
	Href: "/~readium/positions.json",
	Type: "application/vnd.readium.position-list+json",
}

// PositionsService implements Service
// Provides a list of discrete locations in the publication, no matter what the original format is.
type PositionsService interface {
	Service
	PositionsByReadingOrder() [][]manifest.Locator // Returns the list of all the positions in the publication, grouped by the resource reading order index.
	Positions() []manifest.Locator                 // Returns the list of all the positions in the publication. (flattening of PositionsByReadingOrder)
}

// PerResourcePositionsService implements PositionsService
// Simple [PositionsService] which generates one position per [readingOrder] resource.
type PerResourcePositionsService struct {
	readingOrder      manifest.LinkList
	fallbackMediaType string
}

func GetForPositionsService(service PositionsService, link manifest.Link) (fetcher.Resource, bool) {
	if link.Href != PositionsLink.Href {
		return nil, false
	}

	return fetcher.NewBytesResource(PositionsLink, func() []byte {
		positions := service.Positions()
		bin, _ := json.Marshal(map[string]interface{}{
			"total":     len(positions),
			"positions": positions,
		})
		return bin
	}), true
}

func (s PerResourcePositionsService) Close() {}

func (s PerResourcePositionsService) Links() manifest.LinkList {
	return manifest.LinkList{PositionsLink}
}

func (s PerResourcePositionsService) Get(link manifest.Link) (fetcher.Resource, bool) {
	return GetForPositionsService(s, link)
}

func (s PerResourcePositionsService) Positions() []manifest.Locator {
	poss := s.PositionsByReadingOrder()
	positions := make([]manifest.Locator, len(poss))
	for i, v := range poss {
		positions[i] = v[0] // Always just one element
	}
	return positions
}

func (s PerResourcePositionsService) PositionsByReadingOrder() [][]manifest.Locator {
	positions := make([][]manifest.Locator, len(s.readingOrder))
	pageCount := len(s.readingOrder)
	for i, v := range s.readingOrder {
		typ := v.Type
		if typ == "" {
			typ = s.fallbackMediaType
		}
		positions[i] = []manifest.Locator{{
			Href:  v.Href,
			Type:  typ,
			Title: v.Title,
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(i) + 1),
				TotalProgression: extensions.Pointer(float64(i) / float64(pageCount)),
			},
		}}
	}
	return positions
}

func PerResourcePositionsServiceFactory(fallbackMediaType string) ServiceFactory {
	return func(context Context) Service {
		return PerResourcePositionsService{
			readingOrder:      context.Manifest.ReadingOrder,
			fallbackMediaType: fallbackMediaType,
		}
	}
}
