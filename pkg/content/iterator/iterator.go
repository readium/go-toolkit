package iterator

import "github.com/readium/go-toolkit/pkg/content/element"

// Iterates through a list of [Element] items asynchronously.
// [hasNext] and [hasPrevious] refer to the last element computed by a previous call to any of both methods.
// TODO: It's based on a kotlin iterator, maybe we can make this more of something for go?
type Iterator interface {
	HasNext() (bool, error)     // Returns true if the iterator has a next element
	Next() element.Element      // Retrieves the element computed by a preceding call to [hasNext]. Panics if [hasNext] was not invoked.
	HasPrevious() (bool, error) // Returns true if the iterator has a previous element
	Previous() element.Element  // Retrieves the element computed by a preceding call to [hasPrevious]. Panics if [hasNext] was not invoked.
}

// Moves to the next item and returns it, or nil if we reached the end.
func ItNextOrNil(it Iterator) (element.Element, error) {
	b, err := it.HasNext()
	if err != nil {
		return nil, err
	}
	if b {
		return it.Next(), nil
	}
	return nil, nil
}

// Moves to the previous item and returns it, or nil if we reached the beginning.
func ItPreviousOrNil(it Iterator) (element.Element, error) {
	b, err := it.HasPrevious()
	if err != nil {
		return nil, err
	}
	if b {
		return it.Previous(), nil
	}
	return nil, nil
}

// [Iterator] for a resource, associated with its [index] in the reading order.
type IndexedIterator struct {
	index    int
	iterator Iterator
}

func (it *IndexedIterator) NextContentIn(direction Direction) (element.Element, error) {
	if direction == Foward {
		return ItNextOrNil(it.iterator)
	} else {
		return ItPreviousOrNil(it.iterator)
	}
}

type Direction int8

const Foward Direction = 1
const Backward Direction = -1

// Just turn the direction into a number by casting it
func (d Direction) Delta() int {
	return int(d)
}

// [Element] loaded with [hasPrevious] or [hasNext], associated with the move direction.
type ElementInDirection struct {
	El  element.Element
	Dir Direction
}

// [Element] loaded with [hasPrevious] or [hasNext], associated with the move delta.
type ElementWithDelta struct {
	El    element.Element
	Delta int
}
