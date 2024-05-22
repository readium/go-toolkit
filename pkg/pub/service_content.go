package pub

import (
	"encoding/json"

	"github.com/readium/go-toolkit/pkg/content"
	"github.com/readium/go-toolkit/pkg/content/element"
	"github.com/readium/go-toolkit/pkg/content/iterator"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// TODO content iterator special ~readium link

var ContentLink = manifest.Link{
	Href: "/~readium/content.json",
	Type: "application/vnd.readium.content+json",
}

// TODO uri template or something so we're not just dumping entire content
// progression, href, cssselector, text context

// PositionsService implements Service
// Provides a way to extract the raw [Content] of a [Publication].
type ContentService interface {
	Service
	Content(start *manifest.Locator) content.Content // Creates a [Content] starting from the given [start] location.
}

// Implements ContentService
type DefaultContentService struct {
	context                          Context
	resourceContentIteratorFactories []iterator.ResourceContentIteratorFactory
}

func GetForContentService(service ContentService, link manifest.Link) (fetcher.Resource, bool) {
	if link.Href != ContentLink.Href {
		return nil, false
	}

	elements, err := content.ContentElements(service.Content(nil))
	if err != nil {
		return fetcher.NewFailureResource(ContentLink, fetcher.Other(err)), false
	}

	return fetcher.NewBytesResource(ContentLink, func() []byte {
		// Warning: this can be a massive payload since it's the entire content of the publication right now
		bin, _ := json.Marshal(elements)
		return bin
	}), true
}

func (s DefaultContentService) Close() {}

func (s DefaultContentService) Links() manifest.LinkList {
	return manifest.LinkList{ContentLink}
}

func (s DefaultContentService) Get(link manifest.Link) (fetcher.Resource, bool) {
	return GetForContentService(s, link)
}

func (s DefaultContentService) Content(start *manifest.Locator) content.Content {
	return contentImplementation{
		context:                          s.context,
		start:                            start,
		resourceContentIteratorFactories: s.resourceContentIteratorFactories,
	}
}

type contentImplementation struct {
	context                          Context
	start                            *manifest.Locator
	resourceContentIteratorFactories []iterator.ResourceContentIteratorFactory
}

func (c contentImplementation) Iterator() iterator.Iterator {
	return iterator.NewPublicationContent(
		c.context.Manifest,
		c.context.Fetcher,
		c.start,
		c.resourceContentIteratorFactories,
	)
}

func (c contentImplementation) Elements() ([]element.Element, error) {
	return content.ContentElements(c)
}

func (c contentImplementation) Text(separator *string) (string, error) {
	return content.ContentText(c, separator)
}

func DefaultContentServiceFactory(resourceContentIteratorFactories []iterator.ResourceContentIteratorFactory) ServiceFactory {
	return func(context Context) Service {
		return DefaultContentService{
			context:                          context,
			resourceContentIteratorFactories: resourceContentIteratorFactories,
		}
	}
}
