package iterator

import (
	"context"

	"github.com/readium/go-toolkit/pkg/content/element"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

type ResourceContentIteratorFactory = func(fetcher.Resource, manifest.Locator) Iterator

type PublicationContentIterator struct {
	manifest                         manifest.Manifest
	fetcher                          fetcher.Fetcher
	startLocator                     *manifest.Locator
	resourceContentIteratorFactories []ResourceContentIteratorFactory

	_currentIterator *IndexedIterator
	currentElement   *ElementInDirection
}

// TODO maybe wrap manifest/fetcher in something that doesn't depend on pub package
func NewPublicationContent(manifest manifest.Manifest, fetcher fetcher.Fetcher, startLocator *manifest.Locator, resourceContentIteratorFactories []ResourceContentIteratorFactory) *PublicationContentIterator {
	return &PublicationContentIterator{
		manifest:                         manifest,
		fetcher:                          fetcher,
		startLocator:                     startLocator,
		resourceContentIteratorFactories: resourceContentIteratorFactories,
	}
}

func (it *PublicationContentIterator) HasPrevious(ctx context.Context) (bool, error) {
	e, err := it.nextIn(ctx, Backward)
	if err != nil {
		return false, err
	}
	it.currentElement = e
	return it.currentElement != nil, nil
}

func (it *PublicationContentIterator) Previous() element.Element {
	if it.currentElement == nil || it.currentElement.Dir != Backward {
		panic("Previous() in PublicationContentIterator called without successful call to HasPrevious() first") // TODO should this be a panic?
	}
	return it.currentElement.El
}

func (it *PublicationContentIterator) HasNext(ctx context.Context) (bool, error) {
	e, err := it.nextIn(ctx, Foward)
	if err != nil {
		return false, err
	}
	it.currentElement = e
	return it.currentElement != nil, nil
}

func (it *PublicationContentIterator) Next() element.Element {
	if it.currentElement == nil || it.currentElement.Dir != Foward {
		panic("Next() in PublicationContentIterator called without successful call to HasNext() first") // TODO should this be a panic?
	}
	return it.currentElement.El
}

func (it *PublicationContentIterator) nextIn(ctx context.Context, direction Direction) (*ElementInDirection, error) {
	iterator := it.currentIterator(ctx)
	if iterator == nil {
		return nil, nil
	}

	content, err := iterator.NextContentIn(ctx, direction)
	if err != nil {
		return nil, err
	}
	if content == nil {
		if ni := it.nextIteratorIn(ctx, direction, iterator.index); ni != nil {
			it._currentIterator = ni
			return it.nextIn(ctx, direction)
		}
		return nil, nil
	}
	return &ElementInDirection{
		El:  content,
		Dir: direction,
	}, nil
}

// Returns the [Iterator] for the current [Resource] in the reading order.
func (it *PublicationContentIterator) currentIterator(ctx context.Context) *IndexedIterator {
	if it._currentIterator == nil {
		it._currentIterator = it.initialIterator(ctx)
	}
	return it._currentIterator
}

// Returns the first iterator starting at [startLocator] or the beginning of the publication.
func (it *PublicationContentIterator) initialIterator(ctx context.Context) *IndexedIterator {
	var index int
	var ii *IndexedIterator
	if it.startLocator != nil {
		if i := it.manifest.ReadingOrder.IndexOfFirstWithHref(it.startLocator.Href); i > 0 {
			index = i
		}
		ii = it.loadIteratorAt(ctx, index, *it.startLocator)
	} else {
		ii = it.loadIteratorAtProgression(ctx, index, 0)
	}

	if ii == nil {
		return it.nextIteratorIn(ctx, Foward, index)
	}
	return ii
}

// Returns the next resource iterator in the given [direction], starting from [fromIndex]
func (it *PublicationContentIterator) nextIteratorIn(ctx context.Context, direction Direction, fromIndex int) *IndexedIterator {
	index := fromIndex + direction.Delta()
	if index < 0 || index >= len(it.manifest.ReadingOrder) {
		return nil
	}

	var progression float64
	if direction == Backward {
		progression = 1
	}

	if it := it.loadIteratorAtProgression(ctx, index, progression); it != nil {
		return it
	}
	return it.nextIteratorIn(ctx, direction, index)
}

// Loads the iterator at the given [index] in the reading order.
// The [locator] will be used to compute the starting [Locator] for the iterator.
func (it *PublicationContentIterator) loadIteratorAt(ctx context.Context, index int, locator manifest.Locator) *IndexedIterator {
	link := it.manifest.ReadingOrder[index]
	resource := it.fetcher.Get(ctx, link)

	for _, factory := range it.resourceContentIteratorFactories {
		res := factory(resource, locator)
		if res != nil {
			return &IndexedIterator{index, res}
		}
	}
	return nil
}

// Loads the iterator at the given [index] in the reading order.
// The [progression] will be used to build a locator and call [loadIteratorAt].
func (it *PublicationContentIterator) loadIteratorAtProgression(ctx context.Context, index int, progression float64) *IndexedIterator {
	link := it.manifest.ReadingOrder[index]
	locator := it.manifest.LocatorFromLink(link)
	if locator == nil {
		return nil
	}
	locator.Locations.Progression = &progression
	return it.loadIteratorAt(ctx, index, *locator)
}
