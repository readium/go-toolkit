package converter

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

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

/*func getFirstAttr(n *html.Node, keys []string) string {
	for _, attr := range n.Attr {
		if slices.Contains(keys, attr.Key) {
			return attr.Val
		}
	}
	return ""
}*/

func srcRelativeToHref(n *html.Node, base url.URL) url.URL {
	if n == nil {
		return nil
	}

	if v := getAttr(n, "src"); v != "" {
		if u, _ := url.URLFromString(v); u != nil {
			return base.Resolve(u)
		}
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

// Everything from this list except "device":
// https://github.com/jhy/jsoup/blob/0b10d516ed8f907f8fb4acb9a0806137a8988d45/src/main/java/org/jsoup/parser/Tag.java#L243
var inlineTags map[atom.Atom]struct{} = map[atom.Atom]struct{}{
	atom.Object:   {},
	atom.Base:     {},
	atom.Font:     {},
	atom.Tt:       {},
	atom.I:        {},
	atom.B:        {},
	atom.U:        {},
	atom.Big:      {},
	atom.Small:    {},
	atom.Em:       {},
	atom.Strong:   {},
	atom.Dfn:      {},
	atom.Code:     {},
	atom.Samp:     {},
	atom.Kbd:      {},
	atom.Var:      {},
	atom.Cite:     {},
	atom.Abbr:     {},
	atom.Time:     {},
	atom.Acronym:  {},
	atom.Mark:     {},
	atom.Ruby:     {},
	atom.Rt:       {},
	atom.Rp:       {},
	atom.Rtc:      {},
	atom.A:        {},
	atom.Img:      {},
	atom.Br:       {},
	atom.Wbr:      {},
	atom.Map:      {},
	atom.Q:        {},
	atom.Sub:      {},
	atom.Sup:      {},
	atom.Bdo:      {},
	atom.Iframe:   {},
	atom.Embed:    {},
	atom.Span:     {},
	atom.Input:    {},
	atom.Select:   {},
	atom.Textarea: {},
	atom.Label:    {},
	atom.Button:   {},
	atom.Optgroup: {},
	atom.Option:   {},
	atom.Legend:   {},
	atom.Datalist: {},
	atom.Keygen:   {},
	atom.Output:   {},
	atom.Progress: {},
	atom.Meter:    {},
	atom.Area:     {},
	atom.Param:    {},
	atom.Source:   {},
	atom.Track:    {},
	atom.Summary:  {},
	atom.Command:  {},
	atom.Basefont: {},
	atom.Bgsound:  {},
	atom.Menuitem: {},
	atom.Data:     {},
	atom.Bdi:      {},
	atom.S:        {},
	atom.Strike:   {},
	atom.Nobr:     {},
	atom.Rb:       {},
}

// Not inline = is block
func isInlineTag(n *html.Node) bool {
	if n == nil {
		return false
	}
	_, ok := inlineTags[n.DataAtom]
	return ok
}

// This isn't cheap to run
func nodeLanguage(n *html.Node) *string {
	// xml:lang takes priority over lang

	var lang string
	for _, attr := range n.Attr {
		if attr.Key == "xml:lang" && attr.Val != "" {
			return &attr.Val
		} else if attr.Key == "lang" {
			lang = attr.Val
		}
	}
	if lang != "" {
		return &lang
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

type breadcrumbData struct {
	node   *html.Node
	object *guidednavigation.GuidedNavigationObject
	skip   bool // If true, this block and its children should be skipped
}

// Note that this whole thing is based off of JSoup's NodeVisitor and NodeTraverser classes
// https://jsoup.org/apidocs/org/jsoup/select/NodeVisitor.html
// https://jsoup.org/apidocs/org/jsoup/select/NodeTraversor.html
type HTMLConverter struct {
	baseLocator manifest.Locator

	root    *guidednavigation.GuidedNavigationObject
	current *guidednavigation.GuidedNavigationObject

	skipCurrent     bool
	segmentsAcc     []guidednavigation.GuidedNavigationObject // Segments accumulated for the current element.
	textAcc         strings.Builder                           // Text since the beginning of the current segment, after coalescing whitespaces.
	currentLanguage *string                                   // Language of the current segment.

	breadcrumbs []breadcrumbData // LIFO stack of the current element's block ancestors.
}

func NewHTMLConverter(baseLocator manifest.Locator) *HTMLConverter {
	doc := &guidednavigation.GuidedNavigationObject{}
	return &HTMLConverter{
		baseLocator: baseLocator,
		root:        doc,
		current:     doc,
	}
}

func (c *HTMLConverter) Result() []guidednavigation.GuidedNavigationObject {
	return c.root.Children
}

// Implements NodeTraversor
func (c *HTMLConverter) Head(n *html.Node, depth int) {
	if n.Type == html.ElementNode {
		aria, visible := ExtractNodeAria(n)

		isBlock := !isInlineTag(n)
		if isBlock {
			// Flush text
			c.flushText()

			// Go down in the GN tree
			c.current.Children = append(c.current.Children, guidednavigation.GuidedNavigationObject{
				// Role: []guidednavigation.GuidedNavigationRole{guidednavigation.GuidedNavigationRole(n.Data)},
			})
			c.current = &c.current.Children[len(c.current.Children)-1]

			// Add blocks to breadcrumbs
			c.breadcrumbs = append(c.breadcrumbs, breadcrumbData{
				node:   n,
				object: c.current,
				skip:   !visible,
			})
		}

		roles, level := ExtractNodeRoles(n)

		if n.DataAtom == atom.Br {
			c.flushText()
		} else if n.DataAtom == atom.Audio || n.DataAtom == atom.Video || slices.Contains(roles, guidednavigation.RoleImage) {
			c.flushText()

			if slices.Contains(roles, guidednavigation.RoleImage) {
				obj := guidednavigation.GuidedNavigationObject{
					Role: roles,
				}
				if href := srcRelativeToHref(n, c.baseLocator.Href); href != nil {
					obj.ImgRef = href
				}
				if aria != nil {
					obj.Description = aria.Plain
					c.skipCurrent = true
				}
				c.current.Children = append(c.current.Children, obj)
			} else { // Audio or Video
				href := srcRelativeToHref(n, c.baseLocator.Href)
				if href == nil {
					sourceNodes := childrenOfType(n, atom.Source, 1)
					for _, source := range sourceNodes {
						if src := srcRelativeToHref(source, c.baseLocator.Href); src != nil {
							href = src
							// TODO: we're losing the alts
							break
						}
					}
				}

				if href != nil {
					if n.DataAtom == atom.Audio {
						obj := guidednavigation.GuidedNavigationObject{
							AudioRef: href,
							// elementLocator
						}
						if aria != nil {
							obj.Description = aria.Plain
							c.skipCurrent = true
						}
						c.current.Children = append(c.current.Children, obj)
					} else if n.DataAtom == atom.Video {
						// TODO: videoref?
						panic("videoref not implemented!")
					}
				}
			}
		} else {
			if isBlock {
				if level > 0 {
					c.current.Level = level
				}
				if len(roles) > 0 {
					for _, role := range roles {
						if !slices.Contains(c.current.Role, role) {
							c.current.Role = append(c.current.Role, role)
						}
					}
				}
			}

			if aria != nil {
				c.current.Description = aria.Plain
				c.breadcrumbs[len(c.breadcrumbs)-1].skip = true
				c.skipCurrent = true
			}
		}

		if isBlock {
			c.flushText()
		}
	}
}

// Implements NodeTraversor
func (c *HTMLConverter) Tail(n *html.Node, depth int) {
	if n.Type == html.TextNode && !onlySpace(n.Data) && !c.skippable() {
		language := nodeLanguage(n)
		if c.currentLanguage != language {
			c.flushSegment()
			c.currentLanguage = language
		}

		var stripLeading bool
		if acc := c.textAcc.String(); len(acc) > 0 && acc[len(acc)-1] == ' ' {
			stripLeading = true
		}
		appendNormalizedWhitespace(&c.textAcc, n.Data, stripLeading)
	} else if n.Type == html.ElementNode {
		if !isInlineTag(n) { // Is block
			c.skipCurrent = false
			c.flushText()

			cleanChildren := make([]guidednavigation.GuidedNavigationObject, 0, len(c.current.Children))
			for _, child := range c.current.Children {
				if !child.Empty() {
					cleanChildren = append(cleanChildren, child)
				}
			}
			c.current.Children = cleanChildren

			if len(c.breadcrumbs) > 0 {
				if c.breadcrumbs[len(c.breadcrumbs)-1].node != n {
					panic("HTMLConverter: breadcrumbs mismatch")
				}

				c.breadcrumbs = c.breadcrumbs[:len(c.breadcrumbs)-1]
				if len(c.breadcrumbs) > 0 {
					// Go up in the GN tree
					c.current = c.breadcrumbs[len(c.breadcrumbs)-1].object
				} else {
					c.current = c.root
				}
			} else {
				c.current = c.root
			}
		}
	}
}

func (c *HTMLConverter) skippable() bool {
	if c.skipCurrent {
		return true
	}
	if len(c.breadcrumbs) == 0 {
		return false
	}
	for i := len(c.breadcrumbs) - 1; i >= 0; i-- {
		if c.breadcrumbs[i].skip {
			return true
		}
	}
	return false
}

func (c *HTMLConverter) flushText() {
	c.flushSegment()

	if len(c.segmentsAcc) == 0 {
		return
	}

	// Trim the end of the last segment's text to get a cleaner output for the TextElement.
	// Only whitespaces between the segments are meaningful.
	lastSegment := c.segmentsAcc[len(c.segmentsAcc)-1]
	lastSegment.Text.Plain = strings.TrimRightFunc(lastSegment.Text.Plain, unicode.IsSpace)

	c.current.Children = append(c.current.Children, c.segmentsAcc...)

	if len(c.breadcrumbs) > 0 {
		el := c.breadcrumbs[len(c.breadcrumbs)-1].node
		roles, level := ExtractNodeRoles(el)
		if level > 0 {
			c.current.Level = level
		}
		if len(roles) > 0 {
			for _, role := range roles {
				if !slices.Contains(c.current.Role, role) {
					c.current.Role = append(c.current.Role, role)
				}
			}
		}
	}

	c.segmentsAcc = []guidednavigation.GuidedNavigationObject{}
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

		obj := guidednavigation.GuidedNavigationObject{}
		obj.Text.Plain = text
		if c.currentLanguage != nil {
			obj.Text.Language = *c.currentLanguage
		}
		c.segmentsAcc = append(c.segmentsAcc, obj)
	}

	c.textAcc.Reset()
}
