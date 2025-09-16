package converter

import (
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
		if n.FirstChild != nil {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
	}
	f(n)
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
		rawIds := strings.Split(strings.TrimSpace(labelledBy), " ")
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
			text := strings.TrimSpace(sb.String())
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
	if el.DataAtom == atom.Img {
		if alt := strings.TrimSpace(getAttr(el, "alt")); alt != "" {
			return &guidednavigation.GuidedNavigationText{
				Plain: alt,
			}, true
		}
	}

	return nil, true
}
