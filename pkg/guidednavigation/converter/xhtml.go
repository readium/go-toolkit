package converter

import (
	"encoding/xml"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	xmlNamespace    = "http://www.w3.org/XML/1998/namespace"
	xlinkNamespace  = "http://www.w3.org/1999/xlink"
	svgNamespace    = "http://www.w3.org/2000/svg"
	mathmlNamespace = "http://www.w3.org/1998/Math/MathML"
)

// Namespace prefixes in scope at a point of the document, for reconstructing
// prefixed attribute names (encoding/xml only exposes the resolved namespace URI).
type nsScope struct {
	parent      *nsScope
	uriToPrefix map[string]string
}

func (s *nsScope) lookup(uri string) (string, bool) {
	for scope := s; scope != nil; scope = scope.parent {
		if prefix, ok := scope.uriToPrefix[uri]; ok {
			return prefix, true
		}
	}
	return "", false
}

// ParseXHTML parses an XHTML document into the tree shape html.Parse produces, so
// consumers written against html.Parse output work unchanged. Unlike html.Parse,
// self-closing elements like <span epub:type="pagebreak"/> are handled per XML rules
// instead of being treated as unclosed tags that swallow the content following them.
func ParseXHTML(r io.Reader) (*html.Node, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false                // Invent end tags for mismatched elements, keep unknown entities as text
	dec.AutoClose = xml.HTMLAutoClose // Tolerate HTML-style void tags like <br>
	dec.Entity = xml.HTMLEntity       // &nbsp; etc. (the HTML 4 set, matching the XHTML DTDs)
	// The input has already been decoded to UTF-8, whatever encoding the XML
	// declaration announces
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	doc := &html.Node{Type: html.DocumentNode}
	cur := doc
	scope := &nsScope{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// Track xmlns declarations, which encoding/xml leaves untranslated
			child := &nsScope{parent: scope}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					if child.uriToPrefix == nil {
						child.uriToPrefix = make(map[string]string)
					}
					child.uriToPrefix[a.Value] = a.Name.Local
				}
			}
			scope = child

			local := strings.ToLower(t.Name.Local)
			n := &html.Node{
				Type:      html.ElementNode,
				Data:      local,
				DataAtom:  atom.Lookup([]byte(local)),
				Namespace: elementNamespace(t.Name.Space),
			}
			for _, a := range t.Attr {
				n.Attr = append(n.Attr, attrFromXML(a, scope))
			}
			cur.AppendChild(n)
			cur = n
		case xml.EndElement:
			// Self-closing elements arrive as Start+End pairs, keeping this balanced
			if scope.parent != nil {
				scope = scope.parent
			}
			if cur.Parent != nil {
				cur = cur.Parent
			}
		case xml.CharData:
			if cur == doc {
				continue // Whitespace around the root element
			}
			if last := cur.LastChild; last != nil && last.Type == html.TextNode {
				last.Data += string(t)
			} else {
				cur.AppendChild(&html.Node{Type: html.TextNode, Data: string(t)})
			}
		case xml.Directive:
			if d := string(t); len(d) > 8 && strings.EqualFold(d[:8], "DOCTYPE ") {
				cur.AppendChild(&html.Node{Type: html.DoctypeNode, Data: strings.TrimSpace(d[8:])})
			}
		}
		// Comments and processing instructions are skipped
	}
	return doc, nil
}

func elementNamespace(space string) string {
	switch space {
	case svgNamespace:
		return "svg"
	case mathmlNamespace:
		return "math"
	default:
		// XHTML and unknown namespaces are treated as HTML
		return ""
	}
}

// Converts an XML attribute to the representation html.Parse would have produced:
// the whole qualified name (prefix included) in Key, and an empty Namespace.
func attrFromXML(a xml.Attr, scope *nsScope) html.Attribute {
	local := strings.ToLower(a.Name.Local)
	switch a.Name.Space {
	case "":
		return html.Attribute{Key: local, Val: a.Value}
	case "xmlns":
		// Keep the prefix's case, so the declaration matches the reconstructed
		// qualified names below
		return html.Attribute{Key: "xmlns:" + a.Name.Local, Val: a.Value}
	case xmlNamespace:
		return html.Attribute{Key: "xml:" + local, Val: a.Value}
	case xlinkNamespace:
		return html.Attribute{Key: "xlink:" + local, Val: a.Value}
	}
	if prefix, ok := scope.lookup(a.Name.Space); ok && prefix != "" {
		return html.Attribute{Key: prefix + ":" + local, Val: a.Value}
	}
	if !strings.ContainsAny(a.Name.Space, ":/") {
		// encoding/xml passes undeclared prefixes through verbatim
		return html.Attribute{Key: a.Name.Space + ":" + local, Val: a.Value}
	}
	// A namespace URI without an in-scope prefix: degrade to the local name
	return html.Attribute{Key: local, Val: a.Value}
}
