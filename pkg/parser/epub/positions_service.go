package epub

import (
	"context"
	"math"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Positions Service for an EPUB from its [readingOrder] and [fetcher].
//
// The [presentation] is used to apply different calculation strategy if the resource has a
// reflowable or fixed layout.
//
// https://github.com/readium/architecture/blob/master/models/locators/best-practices/format.md#epub
// https://github.com/readium/architecture/issues/101
type PositionsService struct {
	public             bool                 // Whether the service exposes itself via Links() and Get()
	readingOrder       manifest.LinkList    // The reading order of the publication
	layout             manifest.Layout      // The publication's layout
	fetcher            fetcher.Fetcher      // The publication's fetcher
	reflowableStrategy ReflowableStrategy   // How to compute positions in reflowable resources
	positions          [][]manifest.Locator // Cached calculated positions
}

func (s *PositionsService) Close() {}

func (s *PositionsService) Links() manifest.LinkList {
	if !s.public {
		return nil
	}
	return manifest.LinkList{pub.PositionsLink}
}

func (s *PositionsService) Get(ctx context.Context, link manifest.Link) (fetcher.Resource, bool) {
	if !s.public {
		return nil, false
	}
	return pub.GetForPositionsService(ctx, s, link)
}

// Positions implements pub.PositionsService
func (s *PositionsService) Positions(ctx context.Context) []manifest.Locator {
	poss := s.PositionsByReadingOrder(ctx)
	positions := make([]manifest.Locator, 0, len(poss)) // At least 1 link per RO element
	for _, v := range poss {
		positions = append(positions, v...)
	}
	return positions
}

// PositionsByReadingOrder implements PositionsService
func (s *PositionsService) PositionsByReadingOrder(ctx context.Context) [][]manifest.Locator {
	if len(s.positions) == 0 {
		s.positions = s.computePositions(ctx)
	}
	return s.positions
}

func (s *PositionsService) computePositions(ctx context.Context) [][]manifest.Locator {
	var lastPositionOfPreviousResource uint
	positions := make([][]manifest.Locator, len(s.readingOrder))
	for i, link := range s.readingOrder {
		var lpositions []manifest.Locator
		if s.layout == manifest.LayoutFixed {
			lpositions = s.createFixed(link, lastPositionOfPreviousResource)
		} else {
			lpositions = s.createReflowable(ctx, link, lastPositionOfPreviousResource, s.fetcher)
		}
		if len(lpositions) > 0 {
			pos := lpositions[len(lpositions)-1].Locations.Position
			if pos != nil {
				lastPositionOfPreviousResource = *pos
			}
		}
		positions[i] = lpositions
	}

	// Calculate totalProgression
	var totalPageCount int
	for _, p := range positions {
		totalPageCount += len(p)
	}
	for i, p := range positions {
		for j, locator := range p {
			position := locator.Locations.Position
			if position != nil {
				positions[i][j].Locations.TotalProgression = extensions.Pointer(float64((*position)-1) / float64(totalPageCount))
			}
		}
	}

	return positions
}

func (s *PositionsService) createFixed(link manifest.Link, startPosition uint) []manifest.Locator {
	return []manifest.Locator{s.createLocator(link, 0, startPosition+1)}
}

func (s *PositionsService) createReflowable(ctx context.Context, link manifest.Link, startPosition uint, fetcher fetcher.Fetcher) []manifest.Locator {
	resource := fetcher.Get(ctx, link)
	defer resource.Close()
	positionCount := s.reflowableStrategy.PositionCount(resource)

	positions := make([]manifest.Locator, positionCount)
	for p := uint(0); p < positionCount; p++ {
		positions[p] = s.createLocator(
			link,
			float64(p)/float64(positionCount),
			startPosition+p+1,
		)
	}
	return positions
}

func (s *PositionsService) createLocator(link manifest.Link, progression float64, position uint) manifest.Locator {
	mt := link.MediaType
	if mt == nil {
		mt = &mediatype.HTML
	}
	loc := manifest.Locator{
		Href:      link.URL(nil, nil),
		MediaType: *mt,
		Title:     link.Title,
		Locations: manifest.Locations{
			Progression: extensions.Pointer(progression),
			Position:    extensions.Pointer(position),
		},
	}
	return loc
}

func PositionsServiceFactory(reflowableStrategy ReflowableStrategy) pub.ServiceFactory {
	if reflowableStrategy == nil {
		reflowableStrategy = RecommendedReflowableStrategy
	}

	return func(context pub.Context, public bool) pub.Service {
		return &PositionsService{
			public:             public,
			readingOrder:       context.Manifest.ReadingOrder,
			layout:             context.Manifest.Metadata.Layout,
			fetcher:            context.Fetcher,
			reflowableStrategy: reflowableStrategy,
		}
	}
}

// Strategy used to calculate the number of positions in a reflowable resource.
//
// Note that a fixed-layout resource always has a single position.
type ReflowableStrategy interface {
	PositionCount(resource fetcher.Resource) uint // Returns the number of positions in the given [resource] according to the strategy.
}

// Use the original length of each resource (before compression and encryption) and split it by the given [PageLength].
type OriginalLength struct {
	PageLength int
}

// PositionCount implements ReflowableStrategy
func (l OriginalLength) PositionCount(ctx context.Context, resource fetcher.Resource) uint {
	var length int64
	lnk := resource.Link()
	if enc := lnk.Properties.Encryption(); enc != nil {
		length = enc.OriginalLength
	} else {
		length, _ = resource.Length(ctx)
	}

	return uint(math.Min(math.Ceil(float64(length)/float64(l.PageLength)), 1))
}

// Use the archive entry length (whether it is compressed or stored) and split it by the given [PageLength].
type ArchiveEntryLength struct {
	PageLength int
}

// PositionCount implements ReflowableStrategy
func (l ArchiveEntryLength) PositionCount(resource fetcher.Resource) uint {
	var length uint64
	props := resource.Properties()
	if p := props.Get("https://readium.org/webpub-manifest/properties#archive"); p != nil {
		if pm, ok := p.(map[string]interface{}); ok {
			if el, ok := pm["entryLength"].(uint64); ok {
				length = el
			}
		}
	}

	return uint(math.Max(math.Ceil(float64(length)/float64(l.PageLength)), 1))
}

// Recommended historical strategy: archive entry length split by 1024 bytes pages.
//
// This strategy is used by Adobe RMSDK as well.
// See https://github.com/readium/architecture/issues/123
var RecommendedReflowableStrategy = ArchiveEntryLength{PageLength: 1024}
