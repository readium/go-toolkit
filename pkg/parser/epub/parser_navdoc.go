package epub

import (
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpNavBody = mustCompileNS("//html:body")
	xpNavNav  = mustCompileNS("//html:nav")
)

func ParseNavDoc(document *xmlquery.Node, filePath url.URL) map[string]manifest.LinkList {
	ret := make(map[string]manifest.LinkList)
	docPrefixes := parsePrefixes(SelectNodeAttrNs(document, NamespaceOPS, "prefix"))
	for k, v := range ContentReservedPrefixes {
		if _, ok := docPrefixes[k]; !ok { // prefix element overrides reserved prefixes
			docPrefixes[k] = v
		}
	}

	body := xmlquery.QuerySelector(document, xpNavBody)
	if body == nil {
		return ret
	}

	for _, nav := range xmlquery.QuerySelectorAll(body, xpNavNav) {
		types, links := parseNavElement(nav, filePath, docPrefixes)
		if types == nil && links == nil {
			continue
		}

		for _, t := range types {
			suffix := strings.TrimPrefix(t, VocabularyType)
			if suffix == "toc" || suffix == "page-list" || suffix == "landmarks" || suffix == "lot" || suffix == "loi" || suffix == "loa" || suffix == "lov" {
				ret[suffix] = links
			} else {
				ret[t] = links
			}
		}
	}

	return ret
}

func parseNavElement(nav *xmlquery.Node, filePath url.URL, prefixMap map[string]string) ([]string, manifest.LinkList) {
	typeAttr := SelectNodeAttrNs(nav, NamespaceOPS, "type")
	if typeAttr == "" {
		return nil, nil
	}

	parsedProps := parseProperties(typeAttr)
	types := make([]string, 0, len(parsedProps))
	for _, prop := range parsedProps {
		types = append(types, resolveProperty(prop, prefixMap, DefaultVocabType))
	}

	links := parseOlElement(firstNavigationChild(nav, NamespaceXHTML, "ol"), filePath)
	if len(links) > 0 && len(types) > 0 {
		return types, links
	}
	return nil, nil
}

func parseOlElement(ol *xmlquery.Node, filePath url.URL) manifest.LinkList {
	if ol == nil {
		return nil
	}
	links := make(manifest.LinkList, 0, countNavigationChildren(ol, NamespaceXHTML, "li"))
	for li := ol.FirstChild; li != nil; li = li.NextSibling {
		if !matchesNavigationElement(li, NamespaceXHTML, "li") {
			continue
		}
		l := parseLiElement(li, filePath)
		if l != nil {
			links = append(links, *l)
		}
	}
	return links
}

func parseLiElement(li *xmlquery.Node, filePath url.URL) (link *manifest.Link) {
	if li == nil {
		return nil
	}
	first := li.FirstChild // should be <a>, <span>, or <ol>, in any namespace
	for first != nil && first.Type != xmlquery.ElementNode && first.Type != xmlquery.ProcessingInstruction {
		first = first.NextSibling
	}
	// xmlquery's XPath navigator treats processing instructions as elements;
	// retain that behavior when replacing its previous "*" selection.
	if first == nil {
		return nil
	}
	var title string
	if first.Data != "ol" {
		title = collapseWhitespace(first.InnerText())
	}
	rawHref := first.SelectAttr("href")
	var href url.URL
	if first.Data == "a" && rawHref != "" {
		if s, err := url.FromEPUBHref(rawHref); err == nil {
			href = filePath.Resolve(s)
		}
	}

	children := parseOlElement(firstNavigationChild(li, NamespaceXHTML, "ol"), filePath)
	// A nil href here is the lazy stand-in for the old "#" default, whose
	// String() is "" — so a missing or empty-resolved href drops the entry
	// (unless it has children), exactly as before.
	if len(children) == 0 && (href == nil || href.String() == "" || title == "") {
		return nil
	}
	if href == nil {
		href = url.MustURLFromString("#")
	}
	return &manifest.Link{
		Title:    title,
		Href:     manifest.NewHREF(href),
		Children: children,
	}
}

// These helpers implement the direct-child steps used repeatedly by navigation
// parsing, without constructing an XPath iterator for each short selection.
func matchesNavigationElement(node *xmlquery.Node, namespace, name string) bool {
	return (node.Type == xmlquery.ElementNode || node.Type == xmlquery.ProcessingInstruction) &&
		node.NamespaceURI == namespace && node.Data == name
}

func firstNavigationChild(parent *xmlquery.Node, namespace, name string) *xmlquery.Node {
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if matchesNavigationElement(child, namespace, name) {
			return child
		}
	}
	return nil
}

func countNavigationChildren(parent *xmlquery.Node, namespace, name string) int {
	count := 0
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if matchesNavigationElement(child, namespace, name) {
			count++
		}
	}
	return count
}
