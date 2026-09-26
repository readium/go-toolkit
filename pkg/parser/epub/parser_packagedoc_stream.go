package epub

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// parsePackageDocumentData avoids building XML nodes for the flat manifest and
// spine lists. The public DOM parser remains the reference and fallback for
// unusual documents, including its XML error types and model error context.
func parsePackageDocumentData(data []byte, filePath url.URL) (*PackageDocument, error) {
	if document, ok := parsePackageDocumentStream(data, filePath); ok {
		return document, nil
	}
	document, err := xmlquery.ParseWithOptions(bytes.NewReader(data), xmlquery.ParserOptions{
		Decoder: &xmlquery.DecoderOptions{Strict: true, Entity: xml.HTMLEntity},
	})
	if err != nil {
		return nil, fetcher.Other(err)
	}
	result, err := ParsePackageDocument(document, filePath)
	if err != nil {
		return nil, errors.Wrap(err, "invalid OPF file")
	}
	return result, nil
}

// xmlquery's attribute selectors use prefixes in Attr.Name.Space and retain the
// resolved URI separately. Match its URI-to-prefix bookkeeping, including the
// depth rule for a default namespace and declarations surviving an end tag.
type opfXMLPrefix struct {
	name  string
	depth int
}

func opfXMLNode(start xml.StartElement, prefixes map[string]opfXMLPrefix, attrs []xmlquery.Attr) xmlquery.Node {
	if cap(attrs) < len(start.Attr) {
		attrs = make([]xmlquery.Attr, len(start.Attr))
	} else {
		attrs = attrs[:len(start.Attr)]
	}
	for i, attr := range start.Attr {
		name := attr.Name
		if prefix, ok := prefixes[name.Space]; ok {
			name.Space = prefix.name
		}
		attrs[i] = xmlquery.Attr{Name: name, Value: attr.Value, NamespaceURI: attr.Name.Space}
	}
	return xmlquery.Node{Type: xmlquery.ElementNode, Data: start.Name.Local, NamespaceURI: start.Name.Space, Attr: attrs}
}

func parsePackageDocumentStream(data []byte, filePath url.URL) (*PackageDocument, bool) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	decoder.Entity = xml.HTMLEntity
	// Keep CharsetReader nil, as in ReadResourceAsXML's DecoderOptions.

	document := &xmlquery.Node{Type: xmlquery.DocumentNode}
	var root, metadataParent *xmlquery.Node
	var section string
	var seenMetadata, seenManifest, seenSpine bool
	var depth int
	prefixes := map[string]opfXMLPrefix{
		"http://www.w3.org/XML/1998/namespace": {name: "xml"},
	}
	var vocabulary map[string]string
	// Collect pointers while the counts are unknown, then allocate the final
	// value slices once. Growing slices of the much larger model structs would
	// otherwise copy the manifest repeatedly on books with thousands of items.
	var items []*Item
	var itemrefs []*ItemRef
	var attrBuffer [8]xmlquery.Attr

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false
		}
		switch token := token.(type) {
		case xml.StartElement:
			depth++
			for _, attr := range token.Attr {
				// resolveProperty cannot handle a lone colon. Defer this input
				// to the reference parser, which checks XML, package version and
				// required sections before evaluating item properties.
				if attr.Name.Local == "properties" && strings.Contains(attr.Value, ":") {
					for property := range strings.FieldsSeq(attr.Value) {
						if property == ":" {
							return nil, false
						}
					}
				}
				if attr.Name.Local == "xmlns" {
					if previous, ok := prefixes[attr.Value]; !ok || previous.depth >= depth {
						prefixes[attr.Value] = opfXMLPrefix{depth: depth}
					}
				} else if attr.Name.Space == "xmlns" {
					prefixes[attr.Value] = opfXMLPrefix{name: attr.Name.Local, depth: depth}
				}
			}
			// encoding/xml permits undeclared element prefixes; xmlquery does not.
			if token.Name.Space != "" {
				if _, declared := prefixes[token.Name.Space]; !declared {
					return nil, false
				}
			}
			if depth == 1 {
				if root != nil || token.Name.Space != NamespaceOPF || token.Name.Local != "package" {
					return nil, false
				}
				node := opfXMLNode(token, prefixes, nil)
				root = &node
				xmlquery.AddChild(document, root)
				vocabulary = make(map[string]string, len(PackageReservedPrefixes))
				for key, value := range PackageReservedPrefixes {
					vocabulary[key] = value
				}
				for key, value := range parsePrefixes(root.SelectAttr("prefix")) {
					vocabulary[key] = value
				}
				continue
			}
			if token.Name.Space == NamespaceOPF && token.Name.Local == "metadata" {
				// Metadata uses descendant queries, including its language lookup.
				// Keep the full parser for metadata outside the one direct section.
				if depth != 2 || seenMetadata {
					return nil, false
				}
				seenMetadata = true
			}
			if depth == 2 {
				section = ""
				if token.Name.Space == NamespaceOPF {
					switch token.Name.Local {
					case "metadata":
						section = "metadata"
					case "manifest":
						if seenManifest {
							return nil, false
						}
						seenManifest, section = true, "manifest"
					case "spine":
						if seenSpine {
							return nil, false
						}
						seenSpine, section = true, "spine"
					}
				}
				if section != "" {
					node := opfXMLNode(token, prefixes, nil)
					xmlquery.AddChild(root, &node)
					if section == "metadata" {
						metadataParent = &node
					}
				}
				continue
			}
			if metadataParent != nil {
				node := opfXMLNode(token, prefixes, nil)
				xmlquery.AddChild(metadataParent, &node)
				metadataParent = &node
			} else if depth == 3 && token.Name.Space == NamespaceOPF {
				// ParseItem and ParseItemRef retain values, not the temporary node
				// or attributes, so the small attribute buffer is safe to reuse.
				switch {
				case section == "manifest" && token.Name.Local == "item":
					node := opfXMLNode(token, prefixes, attrBuffer[:0])
					if item := ParseItem(&node, filePath, vocabulary); item != nil {
						items = append(items, item)
					}
				case section == "spine" && token.Name.Local == "itemref":
					node := opfXMLNode(token, prefixes, attrBuffer[:0])
					if itemref := ParseItemRef(&node, vocabulary); itemref != nil {
						itemrefs = append(itemrefs, itemref)
					}
				}
			}
		case xml.EndElement:
			depth--
			if metadataParent != nil {
				if depth == 1 {
					metadataParent = nil
				} else {
					metadataParent = metadataParent.Parent
				}
			}
		case xml.CharData:
			if metadataParent != nil {
				// Metadata consumes InnerText, for which text and CDATA are
				// equivalent. Retain all whitespace and nested text in order.
				xmlquery.AddChild(metadataParent, &xmlquery.Node{Type: xmlquery.TextNode, Data: string(token)})
			}
		case xml.ProcInst:
			if token.Target != "xml" || depth != 0 || root != nil {
				return nil, false
			}
		case xml.Directive:
			if depth != 0 || root != nil {
				return nil, false
			}
			// A leading DOCTYPE is ignored by the reference parser as well;
			// custom entities are not expanded by either decoder.
		}
	}
	if root == nil || !seenMetadata || !seenManifest || !seenSpine {
		return nil, false
	}
	// Reuse metadata refinements, localization, package attributes and spine
	// settings from the reference parser, using only the small retained tree.
	result, err := ParsePackageDocument(document, filePath)
	if err != nil {
		return nil, false
	}
	if len(items) > 0 {
		result.Manifest = make([]Item, len(items))
		for i, item := range items {
			result.Manifest[i] = *item
		}
	}
	if len(itemrefs) > 0 {
		result.Spine.itemrefs = make([]ItemRef, len(itemrefs))
		for i, itemref := range itemrefs {
			result.Spine.itemrefs[i] = *itemref
		}
	}
	return result, true
}
