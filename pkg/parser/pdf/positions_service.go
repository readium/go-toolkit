package pdf

import (
	"fmt"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Positions Service for an PDF.
type PositionsService struct {
	link            manifest.Link        // The [Link] to the PDF document in the [Publication].
	pageCount       uint                 // Total page count in the PDF document.
	tableOfContents manifest.LinkList    // Table of contents used to compute the position titles.
	positions       [][]manifest.Locator // Cached calculated positions
}

func (s *PositionsService) Close() {}

func (s *PositionsService) Links() manifest.LinkList {
	return manifest.LinkList{pub.PositionsLink}
}

func (s *PositionsService) Get(link manifest.Link) (fetcher.Resource, bool) {
	return pub.GetForPositionsService(s, link)
}

// Positions implements pub.PositionsService
func (s *PositionsService) Positions() []manifest.Locator {
	poss := s.PositionsByReadingOrder()
	var positions []manifest.Locator
	for _, v := range poss {
		positions = append(positions, v...)
	}
	return positions
}

// PositionsByReadingOrder implements PositionsService
func (s *PositionsService) PositionsByReadingOrder() [][]manifest.Locator {
	if len(s.positions) == 0 {
		s.positions = s.computePositions()
	}
	return s.positions
}

func (s *PositionsService) computePositions() [][]manifest.Locator {
	if s.pageCount <= 0 {
		// Not suppsed to happen
		return [][]manifest.Locator{}
	}

	positions := make([][]manifest.Locator, s.pageCount)
	for i := uint(0); i < s.pageCount; i++ {
		progression := float64(i) / float64(s.pageCount)
		typ := s.link.Type
		if typ == "" {
			typ = mediatype.PDF.String()
		}
		position := i + 1
		fragment := fmt.Sprintf("page=%d", i+1)

		var title string
		if link := s.tableOfContents.FirstWithHref(s.link.Href + "#" + fragment); link != nil {
			title = link.Title
		}

		positions[i] = []manifest.Locator{{
			Href: s.link.Href,
			Type: s.link.Type,
			Locations: manifest.Locations{
				Fragments:        []string{fragment},
				Progression:      &progression,
				TotalProgression: &progression,
				Position:         &position,
			},
			Title: title,
		}}
	}
	return positions
}

func PositionsServiceFactory() pub.ServiceFactory {
	return func(context pub.Context) pub.Service {
		if len(context.Manifest.ReadingOrder) == 0 {
			return nil
		}

		var count uint
		if context.Manifest.Metadata.NumberOfPages != nil {
			count = *context.Manifest.Metadata.NumberOfPages
		}

		return &PositionsService{
			link:            context.Manifest.ReadingOrder[0],
			pageCount:       count,
			tableOfContents: context.Manifest.TableOfContents,
		}
	}
}
