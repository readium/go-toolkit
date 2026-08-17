package pub

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// GuidedNavigationLink is the templated link of the guided navigation service,
// which generates guided navigation documents from the publication's (X)HTML
// documents. It is advertised in the manifest's links when the service is public.
var GuidedNavigationLink = manifest.Link{
	Href:      manifest.MustNewHREFFromString("~readium/guided-navigation.json{?ref}", true),
	MediaType: &mediatype.ReadiumGuidedNavigationDocument,
}

// MediaOverlayLink is the templated link of the media overlay service, which
// converts a resource's SMIL media overlay into a guided navigation document.
// It is never advertised in the manifest's links: the service replaces each SMIL
// alternate with an expansion of this link instead.
var MediaOverlayLink = manifest.Link{
	Href:      manifest.MustNewHREFFromString("~readium/media-overlay.json{?ref}", true),
	MediaType: &mediatype.ReadiumGuidedNavigationDocument,
}

// Pre-cached values of the service links' paths
var resolvedGuidedNavigation url.URL
var resolvedMediaOverlay url.URL

func init() {
	resolvedGuidedNavigation = GuidedNavigationLink.URL(nil, nil)
	resolvedMediaOverlay = MediaOverlayLink.URL(nil, nil)
}

// GuidedNavigationService implements Service
// Provides a way to access guided navigation documents for resources of a [Publication].
type GuidedNavigationService interface {
	Service
	GuideForResource(ctx context.Context, href string) (*guidednavigation.GuidedNavigationDocument, error)
	HasGuideForResource(href string) bool
}

// GetForGuidedNavigationService serves a [GuidedNavigationLink] expansion from the given service.
func GetForGuidedNavigationService(ctx context.Context, service GuidedNavigationService, link manifest.Link) (fetcher.Resource, bool) {
	return getForGuideService(ctx, service, link, GuidedNavigationLink, resolvedGuidedNavigation)
}

// GetForMediaOverlayService serves a [MediaOverlayLink] expansion from the given service.
func GetForMediaOverlayService(ctx context.Context, service GuidedNavigationService, link manifest.Link) (fetcher.Resource, bool) {
	return getForGuideService(ctx, service, link, MediaOverlayLink, resolvedMediaOverlay)
}

func getForGuideService(ctx context.Context, service GuidedNavigationService, link manifest.Link, template manifest.Link, resolved url.URL) (fetcher.Resource, bool) {
	u := link.URL(nil, nil)

	if u.Path() != resolved.Path() {
		// Not the service's link
		return nil, false
	}

	ref := u.Raw().Query().Get("ref")
	if ref == "" {
		// No ref parameter
		// TODO: support omission of ref to generate entire doc.
		// Waiting for guided navigation cache implementation to make this feasible
		return nil, false
	}

	// Overrride the link's href with the expanded service link
	expandedHref := template.URL(nil, map[string]string{
		"ref": ref,
	})
	link.Href = manifest.NewHREF(expandedHref)

	// Check if the referenced resource has a guided navigation document
	if !service.HasGuideForResource(ref) {
		return fetcher.NewFailureResource(
			link, fetcher.NotFound(
				errors.New("referenced resource has no associated guided navigation document"),
			),
		), true
	}

	return fetcher.NewBytesResource(link, func() []byte {
		doc, err := service.GuideForResource(ctx, ref)
		if err != nil {
			// TODO: handle error somehow
			return nil
		}
		bin, _ := json.Marshal(doc)
		return bin
	}), true
}
