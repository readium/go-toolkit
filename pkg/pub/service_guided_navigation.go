package pub

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util"
)

var GuidedNavigationLink = manifest.Link{
	Href:      "/~readium/guided-navigation.json{?ref}",
	Type:      mediatype.ReadiumGuidedNavigationDocument.String(),
	Templated: true,
}

// GuidedNavigationService implements Service
// Provides a way to access guided navigation documents for resources of a [Publication].
type GuidedNavigationService interface {
	Service
	GuideForResource(href string) (*manifest.GuidedNavigationDocument, error)
	HasGuideForResource(href string) bool
}

func GetForGuidedNavigationService(service GuidedNavigationService, link manifest.Link) (fetcher.Resource, bool) {
	// TODO: this is a shortcut to avoid full href parsing and template expansion
	// just just to check if the link is the guided navigation link. It should
	// probably be replaced by something better after the url utilities are updated.
	link.Href = strings.TrimPrefix(link.Href, "/")
	if !strings.HasPrefix(link.Href, "~readium/guided-navigation.json") {
		return nil, false
	}

	href := util.NewHREF(link.Href, "")
	params, err := href.QueryParameters()
	if err != nil {
		// Failed parsing query parameters
		return nil, false
	}
	ref := params.Get("ref")
	if ref == "" {
		// No ref parameter
		// TODO: support omission of ref to generate entire doc.
		// Waiting for guided navigation cache implementation to make this feasible
		return nil, false
	}

	// Check if the provided link's href matches the guided navigation link in expanded form
	expandedLink := GuidedNavigationLink.ExpandTemplate(map[string]string{
		"ref": ref,
	})
	if link.Href != strings.TrimPrefix(expandedLink.Href, "/") {
		return nil, false
	}

	// Check if the referenced resource has a guided navigation document
	if !service.HasGuideForResource(ref) {
		return fetcher.NewFailureResource(
			expandedLink, fetcher.NotFound(
				errors.New("referenced resource has no associated guided navigation document"),
			),
		), true
	}

	return fetcher.NewBytesResource(expandedLink, func() []byte {
		doc, err := service.GuideForResource(ref)
		if err != nil {
			// TODO: handle error somehow
			return nil
		}
		bin, _ := json.Marshal(doc)
		return bin
	}), true
}
