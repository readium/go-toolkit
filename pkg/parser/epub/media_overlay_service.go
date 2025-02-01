package epub

import (
	"slices"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

func MediaOverlayFactory() pub.ServiceFactory {
	return func(context pub.Context) pub.Service {
		// Process reading order to find and replace SMIL alternates
		smilMap := make(map[string]manifest.Link)
		var smilIndexes []string
		for i := range context.Manifest.ReadingOrder {
			alts := context.Manifest.ReadingOrder[i].Alternates
			for j := range alts {
				alt := context.Manifest.ReadingOrder[i].Alternates[j]
				if alt.MediaType.Equal(&mediatype.SMIL) {
					// SMIL alternate for reading order item found

					// Create a guided navigation link for the SMIL alt
					href := context.Manifest.ReadingOrder[i].Href.String()
					gnLink := pub.GuidedNavigationLink
					gnLink.Href = manifest.NewHREF(gnLink.URL(nil,
						map[string]string{
							"ref": href,
						},
					))

					// Store the original SMIL alt in an internal map
					smilMap[href] = alt
					smilIndexes = append(smilIndexes, href)

					// Swap the original SMIL alt with the new guided navigation link
					alts = append(append(alts[:j], gnLink), alts[j+1:]...)
				}
			}
		}
		if len(smilMap) == 0 {
			// No items anyway, don't set up service
			return nil
		}

		return &MediaOverlayService{
			fetcher:                context.Fetcher,
			originalSmilAlternates: smilMap,
			originalSmilIndexes:    smilIndexes,
		}
	}
}

type MediaOverlayService struct {
	fetcher                fetcher.Fetcher
	originalSmilAlternates map[string]manifest.Link
	originalSmilIndexes    []string
	// TODO: smil parsing cache
}

func (s *MediaOverlayService) Close() {
	clear(s.originalSmilAlternates)
	clear(s.originalSmilIndexes)
}

func (s *MediaOverlayService) Links() manifest.LinkList {
	return manifest.LinkList{pub.GuidedNavigationLink}
}

func (s *MediaOverlayService) HasGuideForResource(href string) bool {
	_, ok := s.originalSmilAlternates[href]
	return ok
}

func (s *MediaOverlayService) GuideForResource(href string) (*manifest.GuidedNavigationDocument, error) {
	// Check if the provided resource has a guided navigation document
	if link, ok := s.originalSmilAlternates[href]; ok {
		res := s.fetcher.Get(link)
		defer res.Close()

		n, rerr := res.ReadAsXML(map[string]string{
			NamespaceOPS:   "epub",
			NamespaceSMIL:  "smil",
			NamespaceSMIL2: "smil2",
		})
		if rerr != nil {
			return nil, rerr.Cause
		}

		// Convert SMIL to guided navigation document
		doc, err := ParseSMILDocument(n, link.URL(nil, nil))
		if err != nil {
			return nil, err
		}

		// Find the next and previous guided navigation docs in the readingOrder
		// Then enhance the document with additional next/prev links
		idx := slices.Index(s.originalSmilIndexes, href)
		if idx > 0 {
			l := pub.GuidedNavigationLink
			l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
				"ref": s.originalSmilIndexes[idx-1],
			}))
			l.Rels = append(l.Rels, "prev")
			doc.Links = append(doc.Links, l)
		}
		if idx < len(s.originalSmilIndexes)-1 {
			l := pub.GuidedNavigationLink
			l.Href = manifest.NewHREF(l.Href.Resolve(nil, map[string]string{
				"ref": s.originalSmilIndexes[idx+1],
			}))
			l.Rels = append(l.Rels, "next")
			doc.Links = append(doc.Links, l)
		}
		return doc, nil
	}
	return nil, nil
}

func (s *MediaOverlayService) Get(link manifest.Link) (fetcher.Resource, bool) {
	return pub.GetForGuidedNavigationService(s, link)
}
