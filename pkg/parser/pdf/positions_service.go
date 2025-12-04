package pdf

import (
	"context"
	"fmt"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Positions Service for an PDF.
type PositionsService struct {
	public          bool                 // Whether the service exposes itself via Links() and Get()
	link            manifest.Link        // The [Link] to the PDF document in the [Publication].
	pageCount       uint                 // Total page count in the PDF document.
	tableOfContents manifest.LinkList    // Table of contents used to compute the position titles.
	positions       [][]manifest.Locator // Cached calculated positions
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
	var positions []manifest.Locator
	for _, v := range poss {
		positions = append(positions, v...)
	}
	return positions
}

// PositionsByReadingOrder implements PositionsService
func (s *PositionsService) PositionsByReadingOrder(ctx context.Context) [][]manifest.Locator {
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
		typ := s.link.MediaType
		if typ == nil {
			typ = &mediatype.PDF
		}
		position := i + 1
		fragment := fmt.Sprintf("page=%d", i+1)

		u := s.link.URL(nil, nil)

		var title string
		if link := s.tableOfContents.FirstWithHref(url.MustURLFromString(u.String() + "#" + fragment)); link != nil {
			title = link.Title
		}

		positions[i] = []manifest.Locator{{
			Href:      u,
			MediaType: *typ,
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
	return func(context pub.Context, public bool) pub.Service {
		if len(context.Manifest.ReadingOrder) == 0 {
			return nil
		}

		var count uint
		if context.Manifest.Metadata.NumberOfPages != nil {
			count = *context.Manifest.Metadata.NumberOfPages
		}

		return &PositionsService{
			public:          public,
			link:            context.Manifest.ReadingOrder[0],
			pageCount:       count,
			tableOfContents: context.Manifest.TableOfContents,
		}
	}
}
