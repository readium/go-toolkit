package iterator

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/readium/go-toolkit/pkg/content/element"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Holds the result of parsing the HTML resource into a list of [element.Element].
// The [startIndex] will be calculated from the element matched by the base [locator], if possible. Defaults to 0.
type ParsedElements struct {
	Elements   []element.Element
	StartIndex int
}

func trimText(text string, before *string) manifest.Text {
	var b string
	if before != nil {
		b = *before
	}
	// Get all the space from the beginning of the string and add it to the before
	var bsb strings.Builder
	for _, v := range text {
		if unicode.IsSpace(v) {
			bsb.WriteRune(v)
		} else {
			break
		}
	}
	b += bsb.String()

	// Get all the space from the end of the string and add it to the after
	var asb strings.Builder
	for i := len(text) - 1; i >= 0; i-- {
		if unicode.IsSpace(rune(text[i])) {
			asb.WriteRune(rune(text[i]))
		} else {
			break
		}
	}

	return manifest.Text{
		Before:    b + bsb.String(),
		Highlight: text[bsb.Len() : len(text)-asb.Len()],
		After:     asb.String(),
	}
}

func onlySpace(s string) bool {
	for _, runeValue := range s {
		if !unicode.IsSpace(runeValue) {
			return false
		}
	}
	return true
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func srcRelativeToHref(n *html.Node, base string) *string {
	if n == nil {
		return nil
	}

	if v := getAttr(n, "src"); v != "" {
		h, _ := util.NewHREF(v, base).String()
		return &h
	}
	return nil
}

// Get child elements of a certain type, with a maximum depth.
func childrenOfType(doc *html.Node, typ atom.Atom, depth uint) (children []*html.Node) {
	var f func(*html.Node, uint)
	f = func(n *html.Node, d uint) {
		if n.Type == html.ElementNode && n.DataAtom == typ {
			children = append(children, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if d > 0 {
				f(c, d-1)
			}
		}
	}
	f(doc, depth)
	return
}

// Get the first or last element of a certain type
func childOfType(doc *html.Node, typ atom.Atom, first bool) *html.Node {
	var b *html.Node
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == typ {
			b = n
			if first {
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return b
}

// https://github.com/jhy/jsoup/blob/0b10d516ed8f907f8fb4acb9a0806137a8988d45/src/main/java/org/jsoup/parser/Tag.java#L243
// TODO convert into atoms
var inlineTags map[string]struct{} = map[string]struct{}{
	"object":   {},
	"base":     {},
	"font":     {},
	"tt":       {},
	"i":        {},
	"b":        {},
	"u":        {},
	"big":      {},
	"small":    {},
	"em":       {},
	"strong":   {},
	"dfn":      {},
	"code":     {},
	"samp":     {},
	"kbd":      {},
	"var":      {},
	"cite":     {},
	"abbr":     {},
	"time":     {},
	"acronym":  {},
	"mark":     {},
	"ruby":     {},
	"rt":       {},
	"rp":       {},
	"rtc":      {},
	"a":        {},
	"img":      {},
	"br":       {},
	"wbr":      {},
	"map":      {},
	"q":        {},
	"sub":      {},
	"sup":      {},
	"bdo":      {},
	"iframe":   {},
	"embed":    {},
	"span":     {},
	"input":    {},
	"select":   {},
	"textarea": {},
	"label":    {},
	"button":   {},
	"optgroup": {},
	"option":   {},
	"legend":   {},
	"datalist": {},
	"keygen":   {},
	"output":   {},
	"progress": {},
	"meter":    {},
	"area":     {},
	"param":    {},
	"source":   {},
	"track":    {},
	"summary":  {},
	"command":  {},
	"device":   {},
	"basefont": {},
	"bgsound":  {},
	"menuitem": {},
	"data":     {},
	"bdi":      {},
	"s":        {},
	"strike":   {},
	"nobr":     {},
	"rb":       {},
}

// Not inline = is block
func isInlineTag(n *html.Node) bool {
	if n == nil {
		return false
	}
	_, ok := inlineTags[n.Data]
	return ok
}

func nodeLanguage(n *html.Node) *string {
	if l := getAttr(n, "lang"); l != "" { // Includes lang and xml:lang
		return &l
	}
	if n.Parent != nil {
		return nodeLanguage(n.Parent)
	}
	return nil
}

// From JSoup: https://github.com/jhy/jsoup/blob/1762412a28fa7b08ccf71d93fc4c98dc73086e03/src/main/java/org/jsoup/internal/StringUtil.java#L233
// Slight differing definition of what a whitespace characacter is
func appendNormalizedWhitespace(accum *strings.Builder, text string, stripLeading bool) {
	var lastWasWhite, reachedNonWhite bool
	for _, t := range text {
		if unicode.IsSpace(t) {
			if (stripLeading && !reachedNonWhite) || lastWasWhite {
				continue
			}
			accum.WriteRune(' ')
			lastWasWhite = true
		} else if t != 8203 && t != 173 { // zero width sp, soft hyphen
			accum.WriteRune(t)
			lastWasWhite = false
			reachedNonWhite = true
		}
	}
}

type NodeVisitor interface {
	Head(n *html.Node, depth int) // Callback for when a node is first visited.
	Tail(n *html.Node, depth int) // Callback for when a node is last visited, after all of its descendants have been visited.
}

// Start a depth-first traverse of the root and all of its descendants.
// This implementation does not use recursion, so a deep DOM does not risk blowing the stack.
// From JSoup: https://github.com/jhy/jsoup/blob/1762412a28fa7b08ccf71d93fc4c98dc73086e03/src/main/java/org/jsoup/select/NodeTraversor.java#L20
// NOTE: Unlike the JSoup implementation, we expect any implementor of NodeVisitor to be read-only, because it simplifies implementation
func TraverseNode(visitor NodeVisitor, root *html.Node) {
	node := root
	depth := 0

	for node != nil {
		visitor.Head(node, depth) // visit current node

		// DON'T check if removed or replaced

		if node.FirstChild != nil { // descend
			node = node.FirstChild
			depth++
		} else {
			for {
				if !(node.NextSibling == nil && depth > 0) {
					break
				}
				visitor.Tail(node, depth) // when no more siblings, ascend
				node = node.Parent
				depth--
			}
			visitor.Tail(node, depth)
			if node == root {
				break
			}
			node = node.NextSibling
		}
	}
}

// Note that this whole thing is based off of JSoup's NodeVisitor and NodeTraverser classes
// https://jsoup.org/apidocs/org/jsoup/select/NodeVisitor.html
// https://jsoup.org/apidocs/org/jsoup/select/NodeTraversor.html
type HTMLConverter struct {
	baseLocator     manifest.Locator
	startElement    *html.Node
	beforeMaxLength int

	elements   []element.Element
	startIndex int

	segmentsAcc        []element.TextSegement // Segments accumulated for the current element.
	textAcc            strings.Builder        // Text since the beginning of the current segment, after coalescing whitespaces.
	wholeRawTextAcc    *string                // Text content since the beginning of the resource, including whitespaces.
	elementRawTextAcc  string                 // Text content since the beginning of the current element, including whitespaces.
	rawTextAcc         string                 // Text content since the beginning of the current element, including whitespaces.
	currentLanguage    *string                // Language of the current segment.
	currentCSSSelector *string                // CSS selector of the current element.

	breadcrumbs []*html.Node // LIFO stack of the current element's block ancestors.
}

func (c *HTMLConverter) Result() ParsedElements {
	p := ParsedElements{
		Elements: c.elements,
	}
	one := 1.0
	if c.baseLocator.Locations.Progression == &one {
		p.StartIndex = len(c.elements)
	} else {
		p.StartIndex = c.startIndex
	}
	return p
}

// Implements NodeTraversor
func (c *HTMLConverter) Head(n *html.Node, depth int) {
	if n.Type == html.ElementNode {
		isBlock := !isInlineTag(n)
		var cssSelector *string
		if isBlock {
			// Add blocks to breadcrumbs
			c.breadcrumbs = append(c.breadcrumbs, n)

			// Calculate CSS selector now because we'll definitely need it
			cs := util.CSSSelector(n)
			cssSelector = &cs
		}

		if n.DataAtom == atom.Br {
			c.flushText()
		} else if n.DataAtom == atom.Img || n.DataAtom == atom.Audio || n.DataAtom == atom.Video {
			c.flushText()

			if cssSelector == nil {
				cs := util.CSSSelector(n)
				cssSelector = &cs
			}
			elementLocator := manifest.Locator{
				Href:  c.baseLocator.Href,
				Type:  c.baseLocator.Type,
				Title: c.baseLocator.Title,
				Text:  c.baseLocator.Text,
				Locations: manifest.Locations{
					OtherLocations: map[string]interface{}{
						"cssSelector": cssSelector,
					},
				},
			}

			if n.DataAtom == atom.Img {
				if href := srcRelativeToHref(n, c.baseLocator.Href); href != nil {
					atlist := []element.Attribute[any]{}
					alt := getAttr(n, "alt")
					if alt == "" {
						// Try fallback to title if no alt
						alt = getAttr(n, "title")
					}
					if alt != "" {
						atlist = append(atlist, element.NewAttribute(element.AcessibilityLabelAttributeKey, alt))
					}
					c.elements = append(c.elements, element.NewImageElement(
						elementLocator,
						manifest.Link{
							Href: *href,
						},
						"", // FIXME: Get the caption from figcaption
						atlist,
					))
				}
			} else { // Audio or Video
				href := srcRelativeToHref(n, c.baseLocator.Href)
				var link *manifest.Link
				if href != nil {
					link = &manifest.Link{
						Href: *href,
					}
				} else {
					sourceNodes := childrenOfType(n, atom.Source, 1)
					sources := make([]manifest.Link, len(sourceNodes))
					for _, source := range sourceNodes {
						if src := srcRelativeToHref(source, c.baseLocator.Href); src != nil {
							l := manifest.Link{
								Href: *src,
							}
							if typ := getAttr(source, "type"); typ != "" {
								l.Type = typ
							}
							sources = append(sources, l)
						}
					}
					if len(sources) > 0 {
						link = &sources[0]
						if len(sources) > 1 {
							link.Alternates = sources[1:]
						}
					}
				}

				if link != nil {
					if n.DataAtom == atom.Audio {
						c.elements = append(c.elements, element.NewAudioElement(
							elementLocator,
							*link,
							[]element.Attribute[any]{},
						))
					} else if n.DataAtom == atom.Video {
						c.elements = append(c.elements, element.NewVideoElement(
							elementLocator,
							*link,
							[]element.Attribute[any]{},
						))
					}
				}
			}
		}

		if isBlock {
			c.flushText()
			c.currentCSSSelector = cssSelector
		}
	}
}

// Implements NodeTraversor
func (c *HTMLConverter) Tail(n *html.Node, depth int) {
	if n.Type == html.TextNode && !onlySpace(n.Data) {
		language := nodeLanguage(n)
		if c.currentLanguage != language {
			c.flushSegment()
			c.currentLanguage = language
		}

		c.rawTextAcc += n.Data

		var stripLeading bool
		if acc := c.textAcc.String(); len(acc) > 0 && acc[len(acc)-1] == ' ' {
			stripLeading = true
		}
		appendNormalizedWhitespace(&c.textAcc, n.Data, stripLeading)
	} else if n.Type == html.ElementNode {
		if !isInlineTag(n) { // Is block
			if c.breadcrumbs[len(c.breadcrumbs)-1] != n {
				// TODO, should we panic? Kotlin does assert(breadcrumbs.last() == node) which throws
				panic("HTMLConverter: breadcrumbs mismatch")
			}
			c.flushText()
			c.breadcrumbs = c.breadcrumbs[:len(c.breadcrumbs)-1]
		}
	}
}

func (c *HTMLConverter) flushText() {
	c.flushSegment()

	if c.startIndex == 0 && c.startElement != nil &&
		((len(c.breadcrumbs) == 0 && c.startElement == nil) ||
			(c.startElement != nil && c.breadcrumbs[len(c.breadcrumbs)-1] == c.startElement)) {
		c.startIndex = len(c.elements)
	}

	if len(c.segmentsAcc) == 0 {
		return
	}

	// Trim the end of the last segment's text to get a cleaner output for the TextElement.
	// Only whitespaces between the segments are meaningful.
	c.segmentsAcc[len(c.segmentsAcc)-1].Text = strings.TrimRightFunc(c.segmentsAcc[len(c.segmentsAcc)-1].Text, unicode.IsSpace)

	var before *string
	if len(c.segmentsAcc) > 0 {
		before = &c.segmentsAcc[0].Locator.Text.Before
	}
	el := element.NewTextElement(
		manifest.Locator{
			Href:  c.baseLocator.Href,
			Type:  c.baseLocator.Type,
			Title: c.baseLocator.Title,
			Locations: manifest.Locations{
				OtherLocations: map[string]interface{}{},
			},
			Text: trimText(c.elementRawTextAcc, before),
		},
		element.Body{},
		c.segmentsAcc,
		nil,
	)
	if c.currentCSSSelector != nil {
		el.Locator().Locations.OtherLocations["cssSelector"] = *c.currentCSSSelector
	}
	c.elements = append(c.elements, el)
	c.elementRawTextAcc = ""
	c.segmentsAcc = []element.TextSegement{}
}

func (c *HTMLConverter) flushSegment() {
	text := c.textAcc.String()
	trimmedText := strings.TrimSpace(text)

	if len(text) > 0 {
		if len(c.segmentsAcc) == 0 {
			text = strings.TrimLeftFunc(text, unicode.IsSpace)

			var whitespaceSuffix string
			r, _ := utf8.DecodeLastRuneInString(text)
			if unicode.IsSpace(r) {
				whitespaceSuffix = string(r)
			}

			text = trimmedText + whitespaceSuffix
		}

		var before *string
		if c.wholeRawTextAcc != nil {
			var last string
			if c.beforeMaxLength > len(*c.wholeRawTextAcc) {
				last = (*c.wholeRawTextAcc)[:]
			} else {
				last = (*c.wholeRawTextAcc)[len(*c.wholeRawTextAcc)-c.beforeMaxLength:]
			}
			before = &last
		}
		seg := element.TextSegement{
			Locator: manifest.Locator{
				Href:  c.baseLocator.Href,
				Type:  c.baseLocator.Type,
				Title: c.baseLocator.Title,
				Locations: manifest.Locations{
					OtherLocations: map[string]interface{}{},
				},
				Text: trimText(c.rawTextAcc, before),
			},
			Text: text,
		}
		if c.currentCSSSelector != nil {
			seg.Locator.Locations.OtherLocations["cssSelector"] = *c.currentCSSSelector
		}
		if c.currentLanguage != nil {
			seg.AttributesHolder = element.NewAttributesHolder([]element.Attribute[any]{
				element.NewAttribute(element.LanguageAttributeKey, c.currentLanguage),
			})
		}
		c.segmentsAcc = append(c.segmentsAcc, seg)
	}

	if c.rawTextAcc != "" {
		if c.wholeRawTextAcc != nil {
			(*c.wholeRawTextAcc) += c.rawTextAcc
		} else {
			ns := strings.Clone(c.rawTextAcc)
			c.wholeRawTextAcc = &ns
		}
	}
	c.rawTextAcc = ""
	c.textAcc.Reset()
}
