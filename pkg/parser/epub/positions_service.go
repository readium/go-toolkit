package epub

import (
	"math"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/service"
)

// Positions Service for an EPUB from its [readingOrder] and [fetcher].
//
// The [presentation] is used to apply different calculation strategy if the resource has a
// reflowable or fixed layout.
//
// https://github.com/readium/architecture/blob/master/models/locators/best-practices/format.md#epub
// https://github.com/readium/architecture/issues/101
type PositionsService struct {
	readingOrder       []manifest.Link
	presentation       *manifest.Presentation
	fetcher            fetcher.Fetcher
	reflowableStrategy ReflowableStrategy
	positions          [][]manifest.Locator
}

func (s *EPUBPositionsService) Close() {}

func (s *EPUBPositionsService) Links() []manifest.Link {
	return []manifest.Link{service.PositionsLink}
}

func (s *EPUBPositionsService) Get(link manifest.Link) (fetcher.Resource, bool) {
	return service.GetForPositionsService(s, link)
}

// Positions implements pub.PositionsService
func (s *EPUBPositionsService) Positions() []manifest.Locator {
	poss := s.PositionsByReadingOrder()
	var positions []manifest.Locator
	for _, v := range poss {
		positions = append(positions, v...)
	}
	return positions
}

// PositionsByReadingOrder implements PositionsService
func (s *EPUBPositionsService) PositionsByReadingOrder() [][]manifest.Locator {
	if len(s.positions) == 0 {
		s.positions = s.computePositions()
	}
	return s.positions
}

func (s *EPUBPositionsService) computePositions() [][]manifest.Locator {
	var lastPositionOfPreviousResource uint
	positions := make([][]manifest.Locator, len(s.readingOrder))
	for i, link := range s.readingOrder {
		var lpositions []manifest.Locator
		if manifest.LayoutOf(link) == manifest.EPUBLayoutFixed {
			lpositions = s.createFixed(link, lastPositionOfPreviousResource)
		} else {
			lpositions = s.createReflowable(link, lastPositionOfPreviousResource, s.fetcher)
		}
		if len(lpositions) > 0 {
			pos := lpositions[len(lpositions)-1].Locations.Position
			if pos > 0 {
				lastPositionOfPreviousResource = pos
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
			if position != 0 {
				positions[i][j].Locations.TotalProgression = float64(position-1) / float64(totalPageCount)
			}
		}
	}

	return positions
}

func (s *EPUBPositionsService) createFixed(link manifest.Link, startPosition uint) []manifest.Locator {
	return []manifest.Locator{s.createLocator(link, 0, startPosition+1)}
}

func (s *EPUBPositionsService) createReflowable(link manifest.Link, startPosition uint, fetcher fetcher.Fetcher) []manifest.Locator {
	resource := fetcher.Get(link)
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

func (s *EPUBPositionsService) createLocator(link manifest.Link, progression float64, position uint) manifest.Locator {
	loc := manifest.Locator{
		Href:  link.Href,
		Type:  link.Type,
		Title: link.Title,
		Locations: &manifest.Locations{
			Progression: progression,
			Position:    position,
		},
	}
	if loc.Type == "" {
		loc.Type = "text/html"
	}
	return loc
}

func EPUBPositionsServiceFactory(reflowableStrategy ReflowableStrategy) service.ServiceFactory {
	return func(context service.Context) service.Service {
		if reflowableStrategy == nil {
			reflowableStrategy = RecommendedReflowableStrategy
		}
		return &EPUBPositionsService{
			readingOrder:       context.Manifest.ReadingOrder,
			presentation:       context.Manifest.Metadata.Presentation,
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
func (l OriginalLength) PositionCount(resource fetcher.Resource) uint {
	var length int64
	lnk := resource.Link()
	if enc := lnk.Properties.Encryption(); enc != nil {
		length = enc.OriginalLength
	} else {
		length, _ = resource.Length()
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
	props := resource.Link().Properties
	if p := props.Get("https://readium.org/webpub-manifest/properties#archive"); p != nil {
		if pm, ok := p.(manifest.Properties); ok {
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
