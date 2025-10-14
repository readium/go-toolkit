package pub

import (
	"context"
	"encoding/json"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

var PositionsLink = manifest.Link{
	Href:      manifest.MustNewHREFFromString("~readium/positions.json", false),
	MediaType: &mediatype.ReadiumPositionList,
}

// PositionsService implements Service
// Provides a list of discrete locations in the publication, no matter what the original format is.
type PositionsService interface {
	Service
	PositionsByReadingOrder(ctx context.Context) [][]manifest.Locator // Returns the list of all the positions in the publication, grouped by the resource reading order index.
	Positions(ctx context.Context) []manifest.Locator                 // Returns the list of all the positions in the publication. (flattening of PositionsByReadingOrder)
}

// PerResourcePositionsService implements PositionsService
// Simple [PositionsService] which generates one position per [readingOrder] resource.
type PerResourcePositionsService struct {
	readingOrder      manifest.LinkList
	fallbackMediaType mediatype.MediaType
	public            bool
}

func GetForPositionsService(ctx context.Context, service PositionsService, link manifest.Link) (fetcher.Resource, bool) {
	if !link.URL(nil, nil).Equivalent(PositionsLink.URL(nil, nil)) {
		return nil, false
	}

	return fetcher.NewBytesResource(PositionsLink, func() []byte {
		positions := service.Positions(ctx)
		bin, _ := json.Marshal(map[string]interface{}{
			"total":     len(positions),
			"positions": positions,
		})
		return bin
	}), true
}

func (s PerResourcePositionsService) Close() {}

func (s PerResourcePositionsService) Links() manifest.LinkList {
	if !s.public {
		return nil
	}
	return manifest.LinkList{PositionsLink}
}

func (s PerResourcePositionsService) Get(ctx context.Context, link manifest.Link) (fetcher.Resource, bool) {
	if !s.public {
		return nil, false
	}
	return GetForPositionsService(ctx, s, link)
}

func (s PerResourcePositionsService) Positions(ctx context.Context) []manifest.Locator {
	poss := s.PositionsByReadingOrder(ctx)
	positions := make([]manifest.Locator, len(poss))
	for i, v := range poss {
		positions[i] = v[0] // Always just one element
	}
	return positions
}

func (s PerResourcePositionsService) PositionsByReadingOrder(ctx context.Context) [][]manifest.Locator {
	positions := make([][]manifest.Locator, len(s.readingOrder))
	pageCount := len(s.readingOrder)
	for i, v := range s.readingOrder {
		typ := v.MediaType
		if typ == nil {
			typ = &s.fallbackMediaType
		}
		positions[i] = []manifest.Locator{{
			Href:      v.Href.Resolve(nil, nil),
			MediaType: *typ,
			Title:     v.Title,
			Locations: manifest.Locations{
				Position:         extensions.Pointer(uint(i) + 1),
				TotalProgression: extensions.Pointer(float64(i) / float64(pageCount)),
			},
		}}
	}
	return positions
}

func PerResourcePositionsServiceFactory(fallbackMediaType mediatype.MediaType) ServiceFactory {
	return func(context Context, public bool) Service {
		return PerResourcePositionsService{
			readingOrder:      context.Manifest.ReadingOrder,
			fallbackMediaType: fallbackMediaType,
			public:            public,
		}
	}
}
