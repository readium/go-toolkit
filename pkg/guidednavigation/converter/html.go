package converter

import (
	"encoding/xml"
	"fmt"
	nurl "net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

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

func hasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func hasElementChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return true
		}
	}
	return false
}

func srcRelativeToHref(n *html.Node, base url.URL) url.URL {
	if n == nil {
		return nil
	}

	if v := getAttr(n, "src"); v != "" {
		if u, _ := url.URLFromString(v); u != nil {
			if base == nil {
				return u
			}
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
		if b != nil && first {
			return
		}
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

// The language in effect for a node, from the nearest lang / xml:lang attribute.
// xml:lang takes priority over lang.
func nodeLanguage(n *html.Node) string {
	for ; n != nil; n = n.Parent {
		var lang string
		for _, attr := range n.Attr {
			if attr.Key == "xml:lang" && attr.Val != "" {
				return attr.Val
			} else if attr.Key == "lang" {
				lang = attr.Val
			}
		}
		if lang != "" {
			return lang
		}
	}
	return ""
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

var ssmlTextEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
var ssmlAttrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// The SSML context a run of text lives in: its language and the SSML tag
// (with attributes) derived from its closest mapped inline ancestor.
type ssmlContext struct {
	lang  string
	tag   string
	attrs []xml.Attr
}

func (a ssmlContext) equal(b ssmlContext) bool {
	if a.lang != b.lang || a.tag != b.tag || len(a.attrs) != len(b.attrs) {
		return false
	}
	for i := range a.attrs {
		if a.attrs[i].Name.Local != b.attrs[i].Name.Local || a.attrs[i].Value != b.attrs[i].Value {
			return false
		}
	}
	return true
}

type segmentKind int

const (
	segmentText segmentKind = iota
	segmentBreak
	segmentPlaceholder
)

type textSegment struct {
	kind segmentKind

	text string      // segmentText: the (whitespace-normalized) text
	ctx  ssmlContext // segmentText: the SSML context of the text

	tag         string            // segmentPlaceholder: SSML tag name without the readium: prefix
	child       *navigationObject // segmentPlaceholder: the object the placeholder references
	candidateID string            // segmentPlaceholder: preferred id for the referenced object
}

type navigationObject struct {
	node     *html.Node
	object   guidednavigation.GuidedNavigationObject
	children []*navigationObject
	parent   *navigationObject
	noText   bool
}

func (n *navigationObject) convert() guidednavigation.GuidedNavigationObject {
	result := n.object

	for _, child := range n.children {
		res := child.convert()
		if res.Empty() {
			continue
		}
		if res.ChildrenOnly() {
			// Splice transparent wrappers (e.g. plain <div>s) into the parent
			result.Children = append(result.Children, res.Children...)
			continue
		}
		result.Children = append(result.Children, res)
	}

	// Hoist a lone anonymous text child into the object itself, together with the
	// objects referenced from its SSML (they carry an id), e.g. a paragraph with an
	// inline image becomes {role, text, children: [image]} like the spec examples.
	if result.Text.Empty() && len(result.Children) == 1 {
		child := result.Children[0]
		if !child.Text.Empty() && child.ID == "" && len(child.Role) == 0 &&
			child.TextRef == nil && child.ImgRef == nil && child.AudioRef == nil && child.VideoRef == nil &&
			child.Description.Empty() {
			result.Text = child.Text
			result.Children = child.Children
		}
	}

	return result
}

// Allocates document-unique identifiers for objects referenced from SSML placeholders.
type idAllocator struct {
	reserved map[string]struct{} // Element ids of the document, avoided when generating ids
	claimed  map[string]struct{} // Ids already assigned to objects
	counters map[string]int
}

// Try to assign a preferred id (e.g. the element's own id) to an object.
func (a *idAllocator) claim(id string) bool {
	if _, ok := a.claimed[id]; ok {
		return false
	}
	a.claimed[id] = struct{}{}
	return true
}

func (a *idAllocator) allocate(prefix string) string {
	for {
		a.counters[prefix]++
		id := fmt.Sprintf("%s%d", prefix, a.counters[prefix])
		if _, ok := a.reserved[id]; ok {
			continue
		}
		if _, ok := a.claimed[id]; ok {
			continue
		}
		a.claimed[id] = struct{}{}
		return id
	}
}

// Maximum depth of note embedding for noterefs referencing notes that themselves
// contain noterefs. Beyond it, notes are referenced by textref instead.
const maxNoterefDepth = 3

type HTMLConverter struct {
	baseLocator manifest.Locator
	xmlParsed   bool // Whether the tree comes from an XML parser (self-closing tags handled correctly).

	segments          []textSegment       // Closed segments of the text flow accumulated for the current block.
	textAcc           strings.Builder     // Text of the currently open segment, with coalesced whitespace.
	currentCtx        ssmlContext         // SSML context of the currently open segment.
	flowEndsWithSpace bool                // Whether the accumulated text flow currently ends in whitespace.
	pendingChildren   []*navigationObject // Objects for placeholders of the current flow, attached at flush time.

	root     *navigationObject
	current  *navigationObject
	skipNode bool

	ids          map[string]*html.Node   // All element ids of the document.
	suppressed   map[*html.Node]struct{} // Elements excluded from the normal flow (e.g. footnotes embedded in a noteref).
	transparent  map[*html.Node]struct{} // Elements whose children flow into the parent without opening an object.
	idAlloc      *idAllocator
	noterefDepth int
	allowNode    *html.Node // Node exempt from suppression/visibility checks (target of a noteref sub-conversion).
}

func NewHTMLConverter(baseLocator manifest.Locator) *HTMLConverter {
	return &HTMLConverter{
		baseLocator: baseLocator,
	}
}

// Whether an element opens (and closes) a navigation object during the traversal.
func (c *HTMLConverter) isBlockNode(n *html.Node) bool {
	if n == c.allowNode {
		// A noteref target is converted as a block even when inline,
		// so that its text is flushed into its own object
		return true
	}
	if _, ok := c.transparent[n]; ok {
		return false
	}
	return !isInlineTag(n)
}

// Builds a reference to a fragment of the converted resource, e.g. "chapter.xhtml#par1".
func (c *HTMLConverter) fragmentRef(id string) url.URL {
	if id == "" || c.baseLocator.Href == nil {
		return nil
	}
	frag, err := url.URLFromGo(&nurl.URL{Fragment: id})
	if err != nil {
		return nil
	}
	return c.baseLocator.Href.Resolve(frag)
}

func (c *HTMLConverter) resolveHref(href string) url.URL {
	u, err := url.URLFromString(href)
	if err != nil || u == nil {
		return nil
	}
	if c.baseLocator.Href == nil {
		return u
	}
	return c.baseLocator.Href.Resolve(u)
}

// If href points to a fragment of the document being converted, returns the fragment.
func (c *HTMLConverter) sameDocumentFragment(href string) (string, bool) {
	if href == "" {
		return "", false
	}
	u, err := url.URLFromString(href)
	if err != nil || u == nil {
		return "", false
	}
	if strings.HasPrefix(href, "#") {
		return u.Fragment(), true
	}
	if c.baseLocator.Href == nil {
		return "", false
	}
	resolved := c.baseLocator.Href.Resolve(u)
	if resolved.Fragment() == "" {
		return "", false
	}
	if strings.TrimPrefix(resolved.Path(), "/") != strings.TrimPrefix(c.baseLocator.Href.Path(), "/") {
		return "", false
	}
	return resolved.Fragment(), true
}

// Collect all element ids and the targets of same-document noterefs, which are
// embedded in their referencing object and excluded from the normal flow.
func (c *HTMLConverter) prescan(root *html.Node) {
	c.ids = make(map[string]*html.Node)
	c.suppressed = make(map[*html.Node]struct{})
	c.idAlloc = &idAllocator{
		reserved: map[string]struct{}{},
		claimed:  map[string]struct{}{},
		counters: map[string]int{},
	}

	type noterefTarget struct {
		id  string
		ref *html.Node
	}
	var noterefTargets []noterefTarget
	var f func(*html.Node, bool)
	f = func(n *html.Node, hidden bool) {
		if n.Type == html.ElementNode {
			if id := getAttr(n, "id"); id != "" {
				if _, ok := c.ids[id]; !ok {
					c.ids[id] = n
				}
				c.idAlloc.reserved[id] = struct{}{}
			}
			hidden = hidden || nodeIsHidden(n)
			// Hidden noterefs are never emitted, so they can't suppress their target
			if !hidden && n.DataAtom == atom.A {
				if roles := ExtractNodeRoles(n); slices.Contains(roles, guidednavigation.RoleNoteref) {
					if target, ok := c.sameDocumentFragment(getAttr(n, "href")); ok {
						noterefTargets = append(noterefTargets, noterefTarget{id: target, ref: n})
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			f(child, hidden)
		}
	}
	f(root, false)

	for _, target := range noterefTargets {
		n, ok := c.ids[target.id]
		if !ok {
			continue
		}
		// A note containing its own noteref (e.g. a reference to an enclosing
		// section) must stay in the normal flow, or its content would be lost
		if isAncestorOf(n, target.ref) {
			continue
		}
		c.suppressed[n] = struct{}{}
	}
}

// Whether anc is n itself or one of its ancestors.
func isAncestorOf(anc, n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == anc {
			return true
		}
	}
	return false
}

func (c *HTMLConverter) descend(n *html.Node) {
	newNode := &navigationObject{
		node:   n,
		parent: c.current,
		noText: c.current.noText,
	}
	c.current.children = append(c.current.children, newNode)
	c.current = newNode
}

func (c *HTMLConverter) ascend() {
	if c.current.parent != nil {
		c.current = c.current.parent
	}
}

func (c *HTMLConverter) appendChild(child *navigationObject) {
	child.parent = c.current
	child.noText = c.current.noText
	c.current.children = append(c.current.children, child)
}

func (c *HTMLConverter) Convert(doc *html.Node) {
	if c.ids == nil {
		c.prescan(doc)
	}

	c.root = &navigationObject{node: doc}
	c.current = c.root
	c.resetFlow()

	node := doc
	depth := 0

	for node != nil {
		c.head(node)
		if node.FirstChild != nil && !c.skipNode { // descend
			node = node.FirstChild
			depth++
		} else {
			for node.NextSibling == nil && depth > 0 {
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
	res := c.root.convert()
	if len(res.Children) == 0 {
		if res.Empty() {
			return nil
		}
		// The root wrapper merged its only child (e.g. the sub-conversion of an
		// inline noteref target): the merged object is the result itself
		return []guidednavigation.GuidedNavigationObject{res}
	}
	return res.Children
}

func (c *HTMLConverter) head(n *html.Node) {
	if n.Type != html.ElementNode {
		return
	}

	if _, ok := skippedElements[n.DataAtom]; ok {
		c.skipNode = true
		return
	}
	if _, ok := c.suppressed[n]; ok && n != c.allowNode {
		c.skipNode = true
		return
	}

	aria, visible := ExtractNodeAria(n)
	if !visible && n != c.allowNode {
		c.skipNode = true
		return
	}

	roles := ExtractNodeRoles(n)

	// Decorative images (role="presentation"/"none", or an explicitly empty alt
	// with no other accessible name) are skipped entirely.
	if n.DataAtom == atom.Img || n.DataAtom == atom.Svg {
		if slices.Contains(roles, guidednavigation.RolePresentation) ||
			(aria == nil && hasAttr(n, "alt") && strings.TrimSpace(getAttr(n, "alt")) == "") {
			c.skipNode = true
			return
		}
	}

	if n.DataAtom == atom.Br {
		if !c.current.noText {
			c.closeSegment()
			c.segments = append(c.segments, textSegment{kind: segmentBreak})
			c.flowEndsWithSpace = true
		}
		return
	}

	// Elements handled wholesale, without descending into them. They become child
	// objects of the enclosing block, referenced from its text with a readium:
	// SSML placeholder tag when the block has text.
	switch {
	case slices.Contains(roles, guidednavigation.RolePagebreak):
		if c.pagebreak(n, aria, roles) {
			// Descend, with the children flowing into the parent context
			if c.transparent == nil {
				c.transparent = make(map[*html.Node]struct{})
			}
			c.transparent[n] = struct{}{}
		} else {
			c.skipNode = true
		}
		return
	case n.DataAtom == atom.A && slices.Contains(roles, guidednavigation.RoleNoteref) && getAttr(n, "href") != "":
		c.noteref(n, roles)
		c.skipNode = true
		return
	case n.DataAtom == atom.Img:
		obj := guidednavigation.GuidedNavigationObject{Role: roles}
		if href := srcRelativeToHref(n, c.baseLocator.Href); href != nil {
			obj.ImgRef = href
		}
		if aria != nil {
			obj.Description = guidednavigation.GuidedNavigationDescription{Text: *aria}
		}
		c.placeholder(n, "image", obj)
		c.skipNode = true
		return
	case n.DataAtom == atom.Audio || n.DataAtom == atom.Video:
		obj := guidednavigation.GuidedNavigationObject{Role: roles}
		href := srcRelativeToHref(n, c.baseLocator.Href)
		if href == nil {
			// The media may be provided through <source> children instead
			for _, source := range childrenOfType(n, atom.Source, 1) {
				if src := srcRelativeToHref(source, c.baseLocator.Href); src != nil {
					href = src
					break
				}
			}
		}
		tag := "audio"
		if n.DataAtom == atom.Audio {
			obj.AudioRef = href
		} else {
			obj.VideoRef = href
			tag = "video"
		}
		if aria != nil {
			obj.Description = guidednavigation.GuidedNavigationDescription{Text: *aria}
		}
		c.placeholder(n, tag, obj)
		c.skipNode = true
		return
	case slices.Contains(roles, guidednavigation.RoleImage):
		// Elements acting as images without being one, e.g. <span role="img"> or <svg>:
		// their content is replaced by their accessible name.
		obj := guidednavigation.GuidedNavigationObject{Role: roles}
		if aria != nil {
			obj.Description = guidednavigation.GuidedNavigationDescription{Text: *aria}
		}
		c.placeholder(n, "image", obj)
		c.skipNode = true
		return
	case n.DataAtom == atom.Math && strings.TrimSpace(getAttr(n, "alttext")) != "":
		// MathML with alternative text: the markup itself is meaningless when read aloud
		obj := guidednavigation.GuidedNavigationObject{
			Role: roles,
			Text: guidednavigation.GuidedNavigationText{Plain: strings.TrimSpace(getAttr(n, "alttext"))},
		}
		c.placeholder(n, "math", obj)
		c.skipNode = true
		return
	}

	if !c.isBlockNode(n) {
		// Text flows through inline elements; their SSML context is computed per text node
		return
	}

	// A block element: open a new navigation object
	c.flushText()
	c.descend(n)

	cur := &c.current.object
	cur.Role = roles
	if aria != nil {
		cur.Description = guidednavigation.GuidedNavigationDescription{Text: *aria}
		if slices.Contains(roles, guidednavigation.RoleFigure) {
			// The accessible name replaces the figure's text content
			// (but media children are still collected)
			c.current.noText = true
		}
	}
	if n.DataAtom == atom.Body {
		// Contextualize the top-level object per the specification
		cur.TextRef = c.baseLocator.Href
	} else if len(roles) > 0 && !c.current.noText {
		if id := getAttr(n, "id"); id != "" {
			cur.TextRef = c.fragmentRef(id)
		}
	}
}

func (c *HTMLConverter) tail(n *html.Node) {
	if n.Type == html.TextNode {
		c.text(n)
		return
	}
	if n.Type != html.ElementNode {
		return
	}
	if c.skipNode {
		c.skipNode = false
		return
	}
	if c.isBlockNode(n) {
		c.flushText()
		c.ascend()
	}
}

func (c *HTMLConverter) text(n *html.Node) {
	if c.current.noText {
		return
	}

	if onlySpace(n.Data) {
		// Whitespace is context-neutral: it separates words without opening a new segment
		if c.textAcc.Len() > 0 || len(c.segments) > 0 {
			appendNormalizedWhitespace(&c.textAcc, n.Data, c.flowEndsWithSpace)
			c.updateFlowSpace()
		}
		return
	}

	ctx := c.textContext(n)
	if !ctx.equal(c.currentCtx) {
		c.closeSegment()
		c.currentCtx = ctx
	}
	appendNormalizedWhitespace(&c.textAcc, n.Data, c.flowEndsWithSpace)
	c.updateFlowSpace()
}

// The SSML context of a text node: its effective language, and the SSML tag mapped
// from its closest inline ancestor within the current block.
func (c *HTMLConverter) textContext(n *html.Node) (ctx ssmlContext) {
	ctx.lang = nodeLanguage(n.Parent)
	for p := n.Parent; p != nil && p != c.current.node; p = p.Parent {
		if p.Type != html.ElementNode {
			continue
		}
		if tag, attrs := ConvertElementToSSMLTag(p.DataAtom); tag != "" && tag != "break" {
			ctx.tag = tag
			ctx.attrs = attrs
			break
		}
	}
	return
}

func (c *HTMLConverter) updateFlowSpace() {
	if c.textAcc.Len() > 0 {
		s := c.textAcc.String()
		c.flowEndsWithSpace = s[len(s)-1] == ' '
	}
}

func (c *HTMLConverter) closeSegment() {
	if c.textAcc.Len() == 0 {
		return
	}
	c.segments = append(c.segments, textSegment{
		kind: segmentText,
		text: c.textAcc.String(),
		ctx:  c.currentCtx,
	})
	c.textAcc.Reset()
}

func (c *HTMLConverter) resetFlow() {
	c.segments = nil
	c.textAcc.Reset()
	c.currentCtx = ssmlContext{}
	c.flowEndsWithSpace = true
	c.pendingChildren = nil
}

// Register an object for an element embedded in the text flow (image, audio, video,
// pagebreak, noteref...). If the enclosing block turns out to have text, the object
// is linked from that text with a <readium:tag id="..."/> SSML placeholder.
func (c *HTMLConverter) placeholder(n *html.Node, tag string, object guidednavigation.GuidedNavigationObject) {
	c.placeholderWithID(n, tag, object, getAttr(n, "id"))
}

func (c *HTMLConverter) placeholderWithID(n *html.Node, tag string, object guidednavigation.GuidedNavigationObject, candidateID string) {
	if object.Empty() {
		// Nothing worth referencing, e.g. an <img> without src or description.
		// Registering it anyway would leave a dangling id in the SSML.
		return
	}
	child := &navigationObject{node: n, object: object}
	if c.current.noText {
		// The surrounding text is suppressed: keep the object, without a placeholder
		c.appendChild(child)
		return
	}
	c.closeSegment()
	c.pendingChildren = append(c.pendingChildren, child)
	c.segments = append(c.segments, textSegment{
		kind:        segmentPlaceholder,
		tag:         tag,
		child:       child,
		candidateID: candidateID,
	})
	c.flowEndsWithSpace = false
}

// Emits a pagebreak object. Returns whether the traversal should still descend into
// the node: a pagebreak marker is normally empty, but when the HTML parser mistakes
// a self-closing <span/> for an open tag, the content following the marker ends up
// inside of it — that content must flow transparently instead of being dropped.
func (c *HTMLConverter) pagebreak(n *html.Node, aria *guidednavigation.GuidedNavigationText, roles []guidednavigation.GuidedNavigationRole) (descend bool) {
	obj := guidednavigation.GuidedNavigationObject{Role: roles}
	// The page number lives in the title attribute, the accessible name, or the content
	if title := strings.TrimSpace(getAttr(n, "title")); title != "" {
		obj.Text = guidednavigation.GuidedNavigationText{Plain: title}
	} else if aria != nil {
		obj.Text = *aria
	}
	labelled := !obj.Text.Empty()
	// Swallowed content only exists on the HTML parse path (XML handles self-closing
	// tags correctly). There, a labelled pagebreak has no content of its own —
	// anything inside is swallowed document content. An unlabelled one might carry
	// the page number as content.
	descend = !c.xmlParsed && (hasElementChild(n) || (labelled && n.FirstChild != nil))
	if !labelled && !descend {
		if text := normalizedNodeText(n); text != "" {
			obj.Text = guidednavigation.GuidedNavigationText{Plain: text}
		}
	}
	if id := getAttr(n, "id"); id != "" {
		obj.TextRef = c.fragmentRef(id)
	}
	c.placeholder(n, "pagebreak", obj)
	return descend
}

func (c *HTMLConverter) noteref(n *html.Node, roles []guidednavigation.GuidedNavigationRole) {
	obj := guidednavigation.GuidedNavigationObject{Role: roles}
	if text := normalizedNodeText(n); text != "" {
		obj.Text = guidednavigation.GuidedNavigationText{Plain: text}
	}

	href := getAttr(n, "href")
	resolved := c.resolveHref(href)
	candidateID := getAttr(n, "id")
	if candidateID == "" && resolved != nil {
		// Identify the noteref object by the note it references, like the
		// examples of the specification
		candidateID = resolved.Fragment()
	}
	if fragment, ok := c.sameDocumentFragment(href); ok {
		// The note lives in this document: embed it in the noteref object.
		// Notes containing their own reference (e.g. a link to the enclosing
		// section) are only referenced, never embedded.
		if target := c.ids[fragment]; target != nil && !isAncestorOf(target, n) && c.noterefDepth < maxNoterefDepth {
			sub := &HTMLConverter{
				baseLocator:  c.baseLocator,
				xmlParsed:    c.xmlParsed,
				ids:          c.ids,
				suppressed:   c.suppressed,
				idAlloc:      c.idAlloc,
				noterefDepth: c.noterefDepth + 1,
				allowNode:    target,
			}
			sub.Convert(target)
			obj.Children = sub.Result()
		}
	}
	if len(obj.Children) == 0 && resolved != nil {
		// The note lives in another resource (or couldn't be embedded): reference it
		obj.Children = []guidednavigation.GuidedNavigationObject{{TextRef: resolved}}
	}

	c.placeholderWithID(n, "noteref", obj, candidateID)
}

func (c *HTMLConverter) flushText() {
	c.closeSegment()
	segments := c.segments
	pending := c.pendingChildren
	c.resetFlow()

	if len(segments) == 0 {
		return
	}

	// Trim the edges of the flow: leading/trailing whitespace and breaks are
	// meaningless, only whitespace between segments matters.
	for len(segments) > 0 {
		seg := &segments[0]
		if seg.kind == segmentBreak {
			segments = segments[1:]
			continue
		}
		if seg.kind == segmentText {
			seg.text = strings.TrimLeftFunc(seg.text, unicode.IsSpace)
			if seg.text == "" {
				segments = segments[1:]
				continue
			}
		}
		break
	}
	for len(segments) > 0 {
		seg := &segments[len(segments)-1]
		if seg.kind == segmentBreak {
			segments = segments[:len(segments)-1]
			continue
		}
		if seg.kind == segmentText {
			seg.text = strings.TrimRightFunc(seg.text, unicode.IsSpace)
			if seg.text == "" {
				segments = segments[:len(segments)-1]
				continue
			}
		}
		break
	}

	hasText := false
	for _, seg := range segments {
		if seg.kind == segmentText && strings.TrimSpace(seg.text) != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		// No meaningful text: any placeholder objects simply stand on their own
		for _, child := range pending {
			c.appendChild(child)
		}
		return
	}

	// The text's base language: the shared language of all its segments,
	// or the block element's own language.
	var textLangs []string
	for _, seg := range segments {
		if seg.kind == segmentText && strings.TrimSpace(seg.text) != "" {
			if !slices.Contains(textLangs, seg.ctx.lang) {
				textLangs = append(textLangs, seg.ctx.lang)
			}
		}
	}
	baseLang := nodeLanguage(c.current.node)
	if len(textLangs) == 1 && textLangs[0] != "" {
		baseLang = textLangs[0]
	}

	needSSML := false
	for _, seg := range segments {
		if seg.kind != segmentText || seg.ctx.tag != "" || seg.ctx.lang != baseLang {
			needSSML = true
			break
		}
	}

	// Now that we know the text object will exist, link the placeholder objects to it
	if needSSML {
		for _, seg := range segments {
			if seg.kind != segmentPlaceholder {
				continue
			}
			id := seg.candidateID
			if id == "" || !c.idAlloc.claim(id) {
				id = c.idAlloc.allocate(seg.tag)
			}
			seg.child.object.ID = id
		}
	}

	// The plain text drops the placeholders. Joints between written chunks get a
	// space depending on what stood between them: a break always separates; a
	// dropped placeholder keeps a surrounding space unless the following text binds
	// to the left, like punctuation ("endnote <ph/>." reads "endnote.", while
	// "resource <ph/> and" reads "resource and").
	var plainB strings.Builder
	prevTrail := false      // The last written chunk ended with a space
	sawBreak := false       // A break stands between the last written chunk and here
	sawPlaceholder := false // A placeholder stands between the last written chunk and here
	midSpace := false       // A whitespace-only segment stands between the last written chunk and here
	for _, seg := range segments {
		switch seg.kind {
		case segmentText:
			lead := strings.HasPrefix(seg.text, " ")
			t := strings.Trim(seg.text, " ")
			if t == "" {
				midSpace = true
				continue
			}
			join := false
			if plainB.Len() > 0 {
				spaced := prevTrail || midSpace || lead
				if sawBreak {
					join = true
				} else if sawPlaceholder {
					join = spaced && !startsWithBindingPunct(t)
				} else {
					join = spaced
				}
			}
			if join {
				plainB.WriteByte(' ')
			}
			plainB.WriteString(t)
			prevTrail = strings.HasSuffix(seg.text, " ")
			sawBreak = false
			sawPlaceholder = false
			midSpace = false
		case segmentBreak:
			sawBreak = true
		case segmentPlaceholder:
			sawPlaceholder = true
		}
	}

	text := guidednavigation.GuidedNavigationText{
		Plain:    strings.TrimSpace(plainB.String()),
		Language: baseLang,
	}

	if needSSML {
		var sb strings.Builder
		for _, seg := range segments {
			switch seg.kind {
			case segmentText:
				tag := seg.ctx.tag
				attrs := seg.ctx.attrs
				if tag == "" && seg.ctx.lang != baseLang && seg.ctx.lang != "" {
					tag = "lang"
				}
				if tag != "" {
					sb.WriteByte('<')
					sb.WriteString(tag)
					for _, attr := range attrs {
						sb.WriteByte(' ')
						sb.WriteString(attr.Name.Local)
						sb.WriteString(`="`)
						sb.WriteString(ssmlAttrEscaper.Replace(attr.Value))
						sb.WriteByte('"')
					}
					if seg.ctx.lang != baseLang && seg.ctx.lang != "" {
						sb.WriteString(` xml:lang="`)
						sb.WriteString(ssmlAttrEscaper.Replace(seg.ctx.lang))
						sb.WriteByte('"')
					}
					sb.WriteByte('>')
					sb.WriteString(ssmlTextEscaper.Replace(seg.text))
					sb.WriteString("</")
					sb.WriteString(tag)
					sb.WriteByte('>')
				} else {
					sb.WriteString(ssmlTextEscaper.Replace(seg.text))
				}
			case segmentBreak:
				sb.WriteString("<break/>")
			case segmentPlaceholder:
				sb.WriteString("<readium:")
				sb.WriteString(seg.tag)
				sb.WriteString(` id="`)
				sb.WriteString(ssmlAttrEscaper.Replace(seg.child.object.ID))
				sb.WriteString(`"/>`)
			}
		}
		text.SSML = sb.String()
	}

	// The objects referenced from the text's SSML become children of the text
	// object, so they stay attached to it even when the enclosing block hosts
	// several text flows.
	textObj := &navigationObject{
		object: guidednavigation.GuidedNavigationObject{Text: text},
	}
	for _, child := range pending {
		child.parent = textObj
		textObj.children = append(textObj.children, child)
	}
	c.appendChild(textObj)
}

// Punctuation that binds to the preceding word, suppressing the space a dropped
// placeholder would otherwise leave behind.
func startsWithBindingPunct(s string) bool {
	if s == "" {
		return false
	}
	switch []rune(s)[0] {
	case '.', ',', ';', ':', '!', '?', ')', ']', '}':
		return true
	}
	return false
}
