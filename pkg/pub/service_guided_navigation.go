package pub

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var GuidedNavigationLink = manifest.Link{
	Href:      manifest.MustNewHREFFromString("~readium/guided-navigation.json{?ref}", true),
	MediaType: &mediatype.ReadiumGuidedNavigationDocument,
}

// Pre-cached value of the guided navigation link's path
var resolvedGuidedNavigation url.URL

func init() {
	resolvedGuidedNavigation = GuidedNavigationLink.URL(nil, nil)
}

// GuidedNavigationService implements Service
// Provides a way to access guided navigation documents for resources of a [Publication].
type GuidedNavigationService interface {
	Service
	GuideForResource(href string) (*manifest.GuidedNavigationDocument, error)
	HasGuideForResource(href string) bool
}

func GetForGuidedNavigationService(service GuidedNavigationService, link manifest.Link) (fetcher.Resource, bool) {
	u := link.URL(nil, nil)

	if u.Path() != resolvedGuidedNavigation.Path() {
		// Not the guided navigation link
		return nil, false
	}

	ref := u.Raw().Query().Get("ref")
	if ref == "" {
		// No ref parameter
		// TODO: support omission of ref to generate entire doc.
		// Waiting for guided navigation cache implementation to make this feasible
		return nil, false
	}

	// Overrride the link's href with the expanded guided navigation link
	expandedHref := GuidedNavigationLink.URL(nil, map[string]string{
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
		doc, err := service.GuideForResource(ref)
		if err != nil {
			// TODO: handle error somehow
			return nil
		}
		bin, _ := json.Marshal(doc)
		return bin
	}), true
}
