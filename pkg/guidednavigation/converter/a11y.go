package converter

import (
	"encoding/xml"
	"slices"
	"strings"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func getElementByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := getElementByID(c, id); res != nil {
			return res
		}
	}
	return nil
}

func nodeIsHidden(n *html.Node) bool {
	for _, attr := range n.Attr {
		if attr.Key == "aria-hidden" && attr.Val == "true" {
			return true
		}
		if attr.Key == "hidden" {
			return true
		}
	}
	return false
}

func nodeText(sb *strings.Builder, n *html.Node) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

// Normalized (whitespace-coalesced and trimmed) text content of a node's subtree.
func normalizedNodeText(n *html.Node) string {
	var raw strings.Builder
	nodeText(&raw, n)
	var sb strings.Builder
	appendNormalizedWhitespace(&sb, raw.String(), true)
	return strings.TrimSpace(sb.String())
}

// https://www.w3.org/TR/accname/#terminology
// Returns the node's accessibility text if existent, and whether or not the node is visible in the first place.
func ExtractNodeAria(el *html.Node) (*guidednavigation.GuidedNavigationText, bool) {
	// 2.A
	if nodeIsHidden(el) {
		return nil, false
	}

	// 2.B
	if labelledBy := strings.TrimSpace(getAttr(el, "aria-labelledby")); labelledBy != "" {
		rawIds := strings.Fields(labelledBy)
		ids := make([]string, 0, len(rawIds))
		for _, v := range rawIds {
			if v != "" && !slices.Contains(ids, v) {
				ids = append(ids, v)
			}
		}

		// Traverse up to the root of the document
		doc := el
		for doc.Parent != nil {
			doc = doc.Parent
		}

		labelNodes := make([]*html.Node, 0, len(ids))
		for _, v := range ids {
			n := getElementByID(doc, v)
			if n != nil {
				labelNodes = append(labelNodes, n)
			}
		}
		if len(labelNodes) > 0 {
			var sb strings.Builder
			for i, n := range labelNodes {
				if nodeIsHidden(n) {
					continue
				}
				if label := getAttr(n, "aria-label"); label != "" {
					sb.WriteString(label)
				} else {
					nodeText(&sb, n)
				}

				if i < len(labelNodes)-1 {
					sb.WriteRune(' ') // Add a space at the end
				}
			}
			var normalized strings.Builder
			appendNormalizedWhitespace(&normalized, sb.String(), true)
			text := strings.TrimSpace(normalized.String())
			if text != "" {
				return &guidednavigation.GuidedNavigationText{
					Plain: text,
				}, true
			}
		}
	}

	// 2.C
	if label := strings.TrimSpace(getAttr(el, "aria-label")); label != "" {
		return &guidednavigation.GuidedNavigationText{
			Plain: label,
		}, true
	}

	// 2.D
	// TODO: more support for els
	switch el.DataAtom {
	case atom.Img:
		if alt := strings.TrimSpace(getAttr(el, "alt")); alt != "" {
			return &guidednavigation.GuidedNavigationText{
				Plain: alt,
			}, true
		}
		// 2.I fallback for images: the title attribute
		if title := strings.TrimSpace(getAttr(el, "title")); title != "" {
			return &guidednavigation.GuidedNavigationText{
				Plain: title,
			}, true
		}
	case atom.Svg:
		// The accessible name of an SVG comes from its <title> child
		if title := childOfType(el, atom.Title, true); title != nil {
			if text := normalizedNodeText(title); text != "" {
				return &guidednavigation.GuidedNavigationText{
					Plain: text,
				}, true
			}
		}
	}

	return nil, true
}

// ConvertElementToSSMLTag maps an HTML element to the SSML tag its text should be wrapped in.
// https://www.w3.org/TR/speech-synthesis11/#S3.2.2
func ConvertElementToSSMLTag(a atom.Atom) (string, []xml.Attr) {
	switch a {
	case atom.Em:
		return "emphasis", nil
	case atom.B:
		return "emphasis", nil
	case atom.I:
		return "emphasis", []xml.Attr{{
			Name:  xml.Name{Local: "level"},
			Value: "reduced",
		}}
	case atom.Strong:
		return "emphasis", []xml.Attr{{
			Name:  xml.Name{Local: "level"},
			Value: "strong",
		}}
	case atom.Br:
		return "break", nil
	default:
		return "", nil
	}
}

// Elements whose entire subtree carries no user-facing content.
var skippedElements = map[atom.Atom]struct{}{
	atom.Script:   {},
	atom.Style:    {},
	atom.Template: {},
	atom.Noscript: {},
	atom.Textarea: {},
	atom.Select:   {},
	atom.Datalist: {},
	atom.Iframe:   {},
	// Ruby annotations would duplicate the base text when read aloud
	atom.Rt:  {},
	atom.Rp:  {},
	atom.Rtc: {},
}
