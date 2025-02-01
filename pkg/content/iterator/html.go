package iterator

import (
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/content/element"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type HTMLContentIterator struct {
	resource        fetcher.Resource
	locator         manifest.Locator
	BeforeMaxLength int // Locators will contain a `before` context of up to this amount of characters.

	currentElement *ElementWithDelta
	currentIndex   *int
	parsedElements *ParsedElements
}

// Iterates an HTML [resource], starting from the given [locator].
// If you want to start mid-resource, the [locator] must contain a `cssSelector` key in its [Locator.Locations] object.
// If you want to start from the end of the resource, the [locator] must have a `progression` of 1.0.
func NewHTML(resource fetcher.Resource, locator manifest.Locator) *HTMLContentIterator {
	return &HTMLContentIterator{
		resource:        resource,
		locator:         locator,
		BeforeMaxLength: 50,
	}
}

func HTMLFactory() ResourceContentIteratorFactory {
	return func(resource fetcher.Resource, locator manifest.Locator) Iterator {
		if resource.Link().MediaType.Matches(&mediatype.HTML, &mediatype.XHTML) {
			return NewHTML(resource, locator)
		}
		return nil
	}
}

func (it *HTMLContentIterator) HasPrevious() (bool, error) {
	if it.currentElement != nil && it.currentElement.Delta == -1 {
		return true, nil
	}

	elements, err := it.elements()
	if err != nil {
		return false, err
	}
	index := elements.StartIndex
	if it.currentIndex != nil {
		index = *it.currentIndex
	}
	index--

	if index < 0 || index >= len(elements.Elements) {
		return false, nil
	}

	it.currentIndex = &index
	it.currentElement = &ElementWithDelta{
		El:    elements.Elements[index],
		Delta: -1,
	}
	return true, nil
}

func (it *HTMLContentIterator) Previous() element.Element {
	if it.currentElement == nil || it.currentElement.Delta != -1 {
		panic("Previous() in HTMLContentIterator called without a previous call to HasPrevious()")
	}
	el := it.currentElement.El
	it.currentElement = nil
	return el
}

func (it *HTMLContentIterator) HasNext() (bool, error) {
	if it.currentElement != nil && it.currentElement.Delta == 1 {
		return true, nil
	}

	elements, err := it.elements()
	if err != nil {
		return false, err
	}
	index := elements.StartIndex - 1
	if it.currentIndex != nil {
		index = *it.currentIndex
	}
	index++

	if index < 0 || index >= len(elements.Elements) {
		return false, nil
	}

	it.currentIndex = &index
	it.currentElement = &ElementWithDelta{
		El:    elements.Elements[index],
		Delta: 1,
	}
	return true, nil
}

func (it *HTMLContentIterator) Next() element.Element {
	if it.currentElement == nil || it.currentElement.Delta != 1 {
		panic("Next() in HTMLContentIterator called without a previous call to HasNext()")
	}
	el := it.currentElement.El
	it.currentElement = nil
	return el
}

func (it *HTMLContentIterator) elements() (*ParsedElements, error) {
	if it.parsedElements == nil {
		elements, err := it.parseElements()
		if err != nil {
			return nil, err
		}
		it.parsedElements = elements
	}
	return it.parsedElements, nil
}

func (it *HTMLContentIterator) parseElements() (*ParsedElements, error) {
	raw, rerr := it.resource.ReadAsString()
	if rerr != nil {
		return nil, errors.Wrap(rerr, "failed reading HTML string of "+it.resource.Link().Href.String())
	}

	document, err := html.ParseWithOptions(
		strings.NewReader(raw),
		html.ParseOptionEnableScripting(false),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing HTML of "+it.resource.Link().Href.String())
	}

	body := childOfType(document, atom.Body, true)
	if body == nil {
		return nil, errors.New("HTML of " + it.resource.Link().Href.String() + " doesn't have a <body>")
	}

	contentConverter := HTMLConverter{
		baseLocator:     it.locator,
		beforeMaxLength: it.BeforeMaxLength,
	}
	if sel := it.locator.Locations.CSSSelector(); sel != "" {
		c, err := cascadia.Parse(sel)
		if err != nil {
			return nil, errors.Wrapf(err, "failed parsing CSS selector \"%s\" of locator for %s", sel, it.locator.Href)
		}
		if find := cascadia.Query(body, c); find != nil {
			contentConverter.startElement = find
		}
	}

	// Traverse the document's HTML
	TraverseNode(&contentConverter, body)

	res := contentConverter.Result()
	return &res, nil
}
