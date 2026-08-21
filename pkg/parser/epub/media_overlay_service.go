package epub

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// MediaOverlayFactory creates a service converting the SMIL media overlays of an
// EPUB/WebPub publication into guided navigation documents. Each SMIL alternate in
// the manifest is replaced with an expansion of [pub.MediaOverlayLink] referencing
// its resource — those swapped alternates are the only way the service surfaces in
// the manifest, it is never advertised in the manifest's links. It does not fall
// back to converting (X)HTML content (see [pub.HTMLGuidedNavigationServiceFactory]
// for that). When the publication has no SMIL alternates, no service is created.
func MediaOverlayFactory() pub.ServiceFactory {
	return func(context pub.Context, public bool) pub.Service {
		smilMap := make(map[string]manifest.Link)
		htmlMap := make(map[string]manifest.Link)
		var guideIndexes []string
		for i := range context.Manifest.ReadingOrder {
			href := context.Manifest.ReadingOrder[i].Href.String()
			hasGuide := false

			alts := context.Manifest.ReadingOrder[i].Alternates
			for j := range alts {
				alt := context.Manifest.ReadingOrder[i].Alternates[j]
				if alt.MediaType != nil && alt.MediaType.Equal(&mediatype.SMIL) {
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

		// Find and replace SMIL alternates with media overlay links
		process := func(link *manifest.Link) (hasOverlay bool) {
			href := link.Href.String()
			for j := range link.Alternates {
				alt := link.Alternates[j]
				if alt.MediaType == nil || !alt.MediaType.Equal(&mediatype.SMIL) {
					continue
				}
				// SMIL alternate for the item found

				// Create a media overlay link for the SMIL alt
				moLink := pub.MediaOverlayLink
				moLink.Href = manifest.NewHREF(moLink.URL(nil,
					map[string]string{
						"ref": href,
					},
				))

				// Store the original SMIL alt in an internal map
				smilMap[href] = alt
				hasOverlay = true

				// Swap the original SMIL alt with the new media overlay link
				link.Alternates[j] = moLink
			}
			return hasOverlay
		}

		// Only reading order items are chained through next/prev links
		var guideIndexes []string
		for i := range context.Manifest.ReadingOrder {
			if process(&context.Manifest.ReadingOrder[i]) {
				guideIndexes = append(guideIndexes, context.Manifest.ReadingOrder[i].Href.String())
			}
		}
		for i := range context.Manifest.Resources {
			process(&context.Manifest.Resources[i])
		}
		if len(smilMap) == 0 {
			// No media overlays anyway, don't set up service
			return nil
		}

		return &MediaOverlayService{
			fetcher:        context.Fetcher,
			smilAlternates: smilMap,
			guideIndexes:   guideIndexes,
		}
	}
}

// MediaOverlayService converts the SMIL media overlays of a publication's reading
// order and resources into guided navigation documents.
type MediaOverlayService struct {
	fetcher        fetcher.Fetcher
	smilAlternates map[string]manifest.Link
	guideIndexes   []string
	// TODO: smil parsing cache
}

func (s *MediaOverlayService) Close() {
	clear(s.smilAlternates)
	clear(s.guideIndexes)
}

func (s *MediaOverlayService) Links() manifest.LinkList {
	// The service is only surfaced through the swapped alternate links
	return nil
}

func (s *MediaOverlayService) HasGuideForResource(href string) bool {
	_, ok := s.smilAlternates[href]
	return ok
}

func (s *MediaOverlayService) GuideForResource(ctx context.Context, href string) (*guidednavigation.GuidedNavigationDocument, error) {
	link, ok := s.smilAlternates[href]
	if !ok {
		return nil, errors.New("resource has no media overlay")
	}
	res := s.fetcher.Get(ctx, link)
	defer res.Close()

	n, rerr := fetcher.ReadResourceAsXML(ctx, res)
	if rerr != nil {
		return nil, rerr.Cause
	}

	// Convert SMIL to guided navigation document
	doc, err := ParseSMILDocument(n, link.URL(nil, nil))
	if err != nil {
		return nil, err
	}

	// Find the next and previous media overlays in the readingOrder
	// Then enhance the document with additional next/prev links.
	// Resources outside the reading order aren't part of the chain
	if idx := slices.Index(s.guideIndexes, href); idx >= 0 {
		if idx > 0 {
			l := pub.MediaOverlayLink
			l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
				"ref": s.guideIndexes[idx-1],
			}))
			l.Rels = append(l.Rels, "prev")
			doc.Links = append(doc.Links, l)
		}
		if idx < len(s.guideIndexes)-1 {
			l := pub.MediaOverlayLink
			l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
				"ref": s.guideIndexes[idx+1],
			}))
			l.Rels = append(l.Rels, "next")
			doc.Links = append(doc.Links, l)
		}
	}
	return doc, nil
}

func (s *MediaOverlayService) Get(ctx context.Context, link manifest.Link) (fetcher.Resource, bool) {
	// Not gated on the service being public: the swapped alternate links are in
	// the manifest unconditionally, so they must always be servable
	return pub.GetForMediaOverlayService(ctx, s, link)
}
