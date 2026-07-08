package epub

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/guidednavigation/converter"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

func MediaOverlayFactory() pub.ServiceFactory {
	return func(context pub.Context, public bool) pub.Service {
		// Process reading order to find and replace SMIL alternates
		smilMap := make(map[string]manifest.Link)
		htmlMap := make(map[string]manifest.Link)
		var guideIndexes []string
		for i := range context.Manifest.ReadingOrder {
			href := context.Manifest.ReadingOrder[i].Href.String()
			hasGuide := false

			alts := context.Manifest.ReadingOrder[i].Alternates
			for j := range alts {
				alt := context.Manifest.ReadingOrder[i].Alternates[j]
				if alt.MediaType.Equal(&mediatype.SMIL) {
					// SMIL alternate for reading order item found

					// Create a guided navigation link for the SMIL alt
					gnLink := pub.GuidedNavigationLink
					gnLink.Href = manifest.NewHREF(gnLink.URL(nil,
						map[string]string{
							"ref": href,
						},
					))

					// Store the original SMIL alt in an internal map
					smilMap[href] = alt
					hasGuide = true

					// Swap the original SMIL alt with the new guided navigation link
					alts = append(append(alts[:j], gnLink), alts[j+1:]...)
				}
			}

			if !hasGuide {
				if mt := context.Manifest.ReadingOrder[i].MediaType; mt != nil && mt.IsHTML() {
					// No SMIL alternate, but a guided navigation document can still
					// be generated from the (X)HTML resource's content
					htmlMap[href] = context.Manifest.ReadingOrder[i]
					hasGuide = true
				}
			}
			if hasGuide {
				guideIndexes = append(guideIndexes, href)
			}
		}
		if len(guideIndexes) == 0 {
			// No items anyway, don't set up service
			return nil
		}

		return &MediaOverlayService{
			fetcher:                context.Fetcher,
			originalSmilAlternates: smilMap,
			htmlResources:          htmlMap,
			guideIndexes:           guideIndexes,
			public:                 public,
		}
	}
}

// MediaOverlayService provides guided navigation documents for a publication's
// reading order: from a SMIL media overlay when the resource has one, otherwise
// by converting the (X)HTML resource's content.
type MediaOverlayService struct {
	public                 bool
	fetcher                fetcher.Fetcher
	originalSmilAlternates map[string]manifest.Link
	htmlResources          map[string]manifest.Link
	guideIndexes           []string
	// TODO: smil parsing cache
}

func (s *MediaOverlayService) Close() {
	clear(s.originalSmilAlternates)
	clear(s.htmlResources)
	clear(s.guideIndexes)
}

func (s *MediaOverlayService) Links() manifest.LinkList {
	if !s.public {
		return nil
	}
	return manifest.LinkList{pub.GuidedNavigationLink}
}

func (s *MediaOverlayService) HasGuideForResource(href string) bool {
	if _, ok := s.originalSmilAlternates[href]; ok {
		return true
	}
	_, ok := s.htmlResources[href]
	return ok
}

func (s *MediaOverlayService) GuideForResource(ctx context.Context, href string) (*guidednavigation.GuidedNavigationDocument, error) {
	var doc *guidednavigation.GuidedNavigationDocument
	if link, ok := s.originalSmilAlternates[href]; ok {
		// The resource has a SMIL media overlay
		res := s.fetcher.Get(ctx, link)
		defer res.Close()

		n, rerr := fetcher.ReadResourceAsXML(ctx, res)
		if rerr != nil {
			return nil, rerr.Cause
		}

		// Convert SMIL to guided navigation document
		var err error
		doc, err = ParseSMILDocument(n, link.URL(nil, nil))
		if err != nil {
			return nil, err
		}
	} else if link, ok := s.htmlResources[href]; ok {
		// Fall back to converting the (X)HTML resource's content
		res := s.fetcher.Get(ctx, link)
		defer res.Close()

		var err error
		doc, err = converter.Do(ctx, res, manifest.Locator{
			Href:      link.URL(nil, nil),
			MediaType: *link.MediaType,
			Title:     link.Title,
		})
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("resource cannot be converted to a guided navigation document")
	}

	// Find the next and previous guided navigation docs in the readingOrder
	// Then enhance the document with additional next/prev links
	idx := slices.Index(s.guideIndexes, href)
	if idx > 0 {
		l := pub.GuidedNavigationLink
		l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
			"ref": s.guideIndexes[idx-1],
		}))
		l.Rels = append(l.Rels, "prev")
		doc.Links = append(doc.Links, l)
	}
	if idx < len(s.guideIndexes)-1 {
		l := pub.GuidedNavigationLink
		l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
			"ref": s.guideIndexes[idx+1],
		}))
		l.Rels = append(l.Rels, "next")
		doc.Links = append(doc.Links, l)
	}
	return doc, nil
}

func (s *MediaOverlayService) Get(ctx context.Context, link manifest.Link) (fetcher.Resource, bool) {
	if !s.public {
		return nil, false
	}
	return pub.GetForGuidedNavigationService(ctx, s, link)
}
