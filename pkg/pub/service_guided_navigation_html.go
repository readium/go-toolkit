package pub

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/guidednavigation/converter"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// HTMLGuidedNavigationServiceFactory creates a service generating guided navigation
// documents from the (X)HTML documents of a publication's reading order and
// resources. It never touches SMIL media overlays (see the EPUB parser's media
// overlay service for those) and adds nothing to the manifest besides its own
// templated link when public. The options are passed to the HTML converter, e.g.
// [converter.WithTextRefLocators]. When the publication has no (X)HTML documents,
// no service is created.
func HTMLGuidedNavigationServiceFactory(opts ...converter.Option) ServiceFactory {
	return func(context Context, public bool) Service {
		htmlMap := make(map[string]manifest.Link)

		// Only reading order items are chained through next/prev links
		var guideIndexes []string
		for _, link := range context.Manifest.ReadingOrder {
			if mt := link.MediaType; mt != nil && mt.IsHTML() {
				href := link.Href.String()
				htmlMap[href] = link
				guideIndexes = append(guideIndexes, href)
			}
		}
		for _, link := range context.Manifest.Resources {
			if mt := link.MediaType; mt != nil && mt.IsHTML() {
				htmlMap[link.Href.String()] = link
			}
		}
		if len(htmlMap) == 0 {
			// No convertible documents, don't set up service
			return nil
		}

		return &HTMLGuidedNavigationService{
			fetcher:          context.Fetcher,
			htmlResources:    htmlMap,
			guideIndexes:     guideIndexes,
			converterOptions: opts,
			public:           public,
		}
	}
}

// HTMLGuidedNavigationService provides guided navigation documents generated from
// the (X)HTML documents of a publication's reading order and resources.
type HTMLGuidedNavigationService struct {
	public           bool
	fetcher          fetcher.Fetcher
	htmlResources    map[string]manifest.Link
	guideIndexes     []string
	converterOptions []converter.Option
}

func (s *HTMLGuidedNavigationService) Close() {
	clear(s.htmlResources)
	clear(s.guideIndexes)
}

func (s *HTMLGuidedNavigationService) Links() manifest.LinkList {
	if !s.public {
		return nil
	}
	return manifest.LinkList{GuidedNavigationLink}
}

func (s *HTMLGuidedNavigationService) HasGuideForResource(href string) bool {
	_, ok := s.htmlResources[href]
	return ok
}

func (s *HTMLGuidedNavigationService) GuideForResource(ctx context.Context, href string) (*guidednavigation.GuidedNavigationDocument, error) {
	link, ok := s.htmlResources[href]
	if !ok {
		return nil, errors.New("resource cannot be converted to a guided navigation document")
	}
	res := s.fetcher.Get(ctx, link)
	defer res.Close()

	doc, err := converter.Do(ctx, res, manifest.Locator{
		Href:      link.URL(nil, nil),
		MediaType: *link.MediaType,
		Title:     link.Title,
	}, s.converterOptions...)
	if err != nil {
		return nil, err
	}

	// Find the next and previous guided navigation docs in the readingOrder
	// Then enhance the document with additional next/prev links.
	// Resources outside the reading order aren't part of the chain
	if idx := slices.Index(s.guideIndexes, href); idx >= 0 {
		if idx > 0 {
			l := GuidedNavigationLink
			l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
				"ref": s.guideIndexes[idx-1],
			}))
			l.Rels = append(l.Rels, "prev")
			doc.Links = append(doc.Links, l)
		}
		if idx < len(s.guideIndexes)-1 {
			l := GuidedNavigationLink
			l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
				"ref": s.guideIndexes[idx+1],
			}))
			l.Rels = append(l.Rels, "next")
			doc.Links = append(doc.Links, l)
		}
	}
	return doc, nil
}

func (s *HTMLGuidedNavigationService) Get(ctx context.Context, link manifest.Link) (fetcher.Resource, bool) {
	if !s.public {
		return nil, false
	}
	return GetForGuidedNavigationService(ctx, s, link)
}
