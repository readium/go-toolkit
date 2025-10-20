package converter

import (
	"encoding/xml"
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

type textSegment struct {
	text string

	tag        *string
	attributes []xml.Attr
}

type navigationObject struct {
	node     *html.Node
	object   guidednavigation.GuidedNavigationObject
	children []*navigationObject
	parent   *navigationObject
	noText   bool
}

func (n *navigationObject) convert(prettify bool) guidednavigation.GuidedNavigationObject {
	result := n.object

	for _, child := range n.children {
		res := child.convert(prettify)
		if !res.Empty() {
			result.Children = append(result.Children, res)
		}
	}
	// Prettify
	if len(result.Children) == 1 && result.Children[0].TextOnly() && prettify {
		result.Text = result.Children[0].Text
		result.Children = nil
	}

	return result
}

type HTMLConverter struct {
	baseLocator manifest.Locator

	segmentsAcc     []textSegment   // Segments accumulated for the current element.
	textAcc         strings.Builder // Text since the beginning of the current segment, after coalescing whitespaces.
	currentLanguage *string         // Language of the current segment.
	lastTextNode    *html.Node

	root     *navigationObject
	current  *navigationObject
	skipNode bool
}

func NewHTMLConverter(baseLocator manifest.Locator) *HTMLConverter {
	return &HTMLConverter{
		baseLocator: baseLocator,
	}
}

func (c *HTMLConverter) descend(n *html.Node) {
	newNode := &navigationObject{
		node:   n,
		parent: c.current,
		noText: c.current.noText,
	}
	if c.current == nil {
		c.root = newNode
	} else {
		c.current.children = append(c.current.children, newNode)
	}
	c.current = newNode
}

func (c *HTMLConverter) ascend() {
	if c.current != nil {
		c.current = c.current.parent
	}
}

func (c *HTMLConverter) Convert(doc *html.Node) {
	node := doc
	c.root = &navigationObject{
		node: doc,
	}
	c.current = c.root

	depth := 0

	for node != nil {
		c.head(node)
		if node.FirstChild != nil && !c.skipNode { // descend
			node = node.FirstChild
			depth++
		} else {
			for {
				if !(node.NextSibling == nil && depth > 0) {
					break
				}
				c.tail(node)
				node = node.Parent
				depth--
			}
			c.tail(node)
			if node == doc {
				break
			}
			node = node.NextSibling
		}
	}
}

func (c *HTMLConverter) Result() []guidednavigation.GuidedNavigationObject {
	if c.root == nil {
		return nil
	}
	return c.root.convert(true).Children
}

func (c *HTMLConverter) head(n *html.Node) {
	if n.Type != html.ElementNode {
		return
	}

	aria, visible := ExtractNodeAria(n)
	if !visible {
		c.skipNode = true
		return
	}

	isBlock := !isInlineTag(n)
	if isBlock {
		// Flush text
		c.flushText()
	}
	c.descend(n)

	cur := &c.current.object

	roles, level := ExtractNodeRoles(n)

	if n.DataAtom == atom.Br {
		c.flushSegment("", nil)
		breakStr := "break"
		c.segmentsAcc = append(c.segmentsAcc, textSegment{
			text: "",
			tag:  &breakStr,
		})
	} else if n.DataAtom == atom.Audio || n.DataAtom == atom.Video || slices.Contains(roles, guidednavigation.RoleImage) || slices.Contains(roles, guidednavigation.RoleFigure) {
		// These three ops are essential to ensuring the correct order of the inline elements in the guided nav tree
		c.flushText()
		c.ascend()
		c.descend(n)
		c.current.object.Role = roles
		if slices.Contains(roles, guidednavigation.RoleImage) {
			if href := srcRelativeToHref(n, c.baseLocator.Href); href != nil {
				c.current.object.ImgRef = href
			}
			if aria != nil {
				c.current.object.Description = aria.Plain
				c.current.noText = true
			}
		} else if slices.Contains(roles, guidednavigation.RoleFigure) {
			if aria != nil {
				c.current.object.Description = aria.Plain
				c.current.noText = true
			}
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
				switch n.DataAtom {
				case atom.Audio:
					c.current.object.AudioRef = href
					if aria != nil {
						c.current.object.Description = aria.Plain
						c.current.noText = true
					}
				case atom.Video:
					// TODO: videoref?
					c.current.noText = true
				}
			}
		}
	} else {
		cur.Level = level
		cur.Role = roles
		if aria != nil {
			cur.Description = aria.Plain
		}
	}
}

func (c *HTMLConverter) tail(n *html.Node) {
	if n.Type == html.TextNode && !onlySpace(n.Data) && !c.current.noText {
		language := nodeLanguage(n)
		ssmlTag, attrs := ConvertElementToSSMLTag(n.Parent.DataAtom)
		if c.currentLanguage != language || ssmlTag != "" {
			c.flushSegment(ssmlTag, attrs)
			c.currentLanguage = language
		}

		var stripLeading bool
		if acc := c.textAcc.String(); len(acc) > 0 && acc[len(acc)-1] == ' ' {
			stripLeading = true
		}
		appendNormalizedWhitespace(&c.textAcc, n.Data, stripLeading)
		c.lastTextNode = n
	} else if n.Type == html.ElementNode {
		if !isInlineTag(n) { // Is block
			c.flushText()
		}
		if !c.skipNode {
			c.ascend()
		} else {
			c.skipNode = false
		}
	}
}

func (c *HTMLConverter) flushText() {
	if c.lastTextNode != nil {
		ssmlTag, attrs := ConvertElementToSSMLTag(c.lastTextNode.Parent.DataAtom)
		c.flushSegment(ssmlTag, attrs)
	} else {
		c.flushSegment("", nil)
	}

	if len(c.segmentsAcc) == 0 {
		return
	}

	// Trim the end of the last segment's text to get a cleaner output for the TextElement.
	// Only whitespaces between the segments are meaningful.
	c.segmentsAcc[len(c.segmentsAcc)-1].text = strings.TrimRightFunc(c.segmentsAcc[len(c.segmentsAcc)-1].text, unicode.IsSpace)

	cobj := guidednavigation.GuidedNavigationObject{}

	var ssml bool
	allLang := true
	var lastLang string
	var sb strings.Builder
	for _, v := range c.segmentsAcc {
		if v.tag != nil {
			ssml = true
			if *v.tag == "lang" {
				// Cheating here because we're in control of the attributes
				if lastLang != "" && lastLang != v.attributes[0].Value {
					allLang = false
					break
				}
				lastLang = v.attributes[0].Value
			} else {
				allLang = false
				break
			}
		} else {
			allLang = false
		}
	}
	if allLang {
		ssml = false
		cobj.Text.Language = lastLang
	}

	for i, v := range c.segmentsAcc {
		if i > 0 && len(c.segmentsAcc[i-1].text) > 0 && len(v.text) > 0 && v.tag == nil {
			sb.WriteRune(' ')
		}
		if ssml {
			if v.tag != nil {
				sb.WriteRune('<')
				sb.WriteString(*v.tag)
				for _, attr := range v.attributes {
					sb.WriteRune(' ')
					sb.WriteString(attr.Name.Local)
					sb.WriteString(`="`)
					xml.EscapeText(&sb, []byte(attr.Value))
					sb.WriteRune('"')
				}
				if len(v.text) > 0 {
					sb.WriteRune('>')
				} else {
					sb.WriteString("/>")
				}
			}
			if len(v.text) > 0 {
				xml.EscapeText(&sb, []byte(v.text))
				if v.tag != nil {
					sb.WriteString("</")
					sb.WriteString(*v.tag)
					sb.WriteRune('>')
				}
			}
		} else {
			sb.WriteString(v.text)
		}
	}
	if ssml {
		cobj.Text.SSML = sb.String()
	} else {
		cobj.Text.Plain = sb.String()
	}
	c.current.children = append(c.current.children, &navigationObject{
		object: cobj,
	})

	c.segmentsAcc = []textSegment{}
}

func (c *HTMLConverter) flushSegment(asTag string, extraAttrs []xml.Attr) {
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

		obj := textSegment{
			text: text,
		}

		if asTag != "" {
			obj.tag = &asTag
		}
		obj.attributes = append(obj.attributes, extraAttrs...)
		if c.currentLanguage != nil {
			if obj.tag == nil {
				langStr := "lang"
				obj.tag = &langStr
			}
			obj.attributes = append(obj.attributes, xml.Attr{
				Name:  xml.Name{Local: "xml:lang"},
				Value: *c.currentLanguage,
			})
		}
		c.segmentsAcc = append(c.segmentsAcc, obj)
	}

	c.textAcc.Reset()
}
