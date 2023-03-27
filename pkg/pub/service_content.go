package pub

import (
	"github.com/readium/go-toolkit/pkg/content"
	"github.com/readium/go-toolkit/pkg/content/element"
	"github.com/readium/go-toolkit/pkg/content/iterator"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// TODO content iterator special ~readium link

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

func (s DefaultContentService) Get(link manifest.Link) (fetcher.Resource, bool) {
	// TODO special API
	return nil, false
}

func (s DefaultContentService) Links() manifest.LinkList {
	return manifest.LinkList{} // TODO special API link
}

func (s DefaultContentService) Close() {}

func (s DefaultContentService) Content(start *manifest.Locator) content.Content {
	return ContentImplementation{
		context:                          s.context,
		start:                            start,
		resourceContentIteratorFactories: s.resourceContentIteratorFactories,
	}
}

type ContentImplementation struct {
	context                          Context
	start                            *manifest.Locator
	resourceContentIteratorFactories []iterator.ResourceContentIteratorFactory
}

func (c ContentImplementation) Iterator() iterator.Iterator {
	return iterator.NewPublicationContent(
		c.context.Manifest,
		c.context.Fetcher,
		c.start,
		c.resourceContentIteratorFactories,
	)
}

func (c ContentImplementation) Elements() []element.Element {
	return content.ContentElements(c)
}

func (c ContentImplementation) Text(separator *string) string {
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
