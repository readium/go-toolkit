package epub

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
	"github.com/readium/xmlquery"
)

func ParseNavDoc(document *xmlquery.Node, filePath string) map[string]manifest.LinkList {
	ret := make(map[string]manifest.LinkList)
	docPrefixes := parsePrefixes(SelectNodeAttrNs(document, NamespaceOPS, "prefix"))
	for k, v := range ContentReservedPrefixes {
		if _, ok := docPrefixes[k]; !ok { // prefix element overrides reserved prefixes
			docPrefixes[k] = v
		}
	}

	body := document.SelectElement("//" + NSSelect(NamespaceXHTML, "body"))
	if body == nil {
		return ret
	}

	for _, nav := range body.SelectElements("//" + NSSelect(NamespaceXHTML, "nav")) {
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

func parseNavElement(nav *xmlquery.Node, filePath string, prefixMap map[string]string) ([]string, manifest.LinkList) {
	typeAttr := ""
	for _, na := range nav.Attr {
		if na.NamespaceURI == NamespaceOPS && na.Name.Local == "type" {
			typeAttr = na.Value
			break
		}
	}
	if typeAttr == "" {
		return nil, nil
	}

	parsedProps := parseProperties(typeAttr)
	types := make([]string, 0, len(parsedProps))
	for _, prop := range parsedProps {
		types = append(types, resolveProperty(prop, prefixMap, DefaultVocabType))
	}

	links := parseOlElement(nav.SelectElement(NSSelect(NamespaceXHTML, "ol")), filePath)
	if len(links) > 0 && len(types) > 0 {
		return types, links
	}
	return nil, nil
}

func parseOlElement(ol *xmlquery.Node, filePath string) manifest.LinkList {
	if ol == nil {
		return nil
	}
	ols := ol.SelectElements(NSSelect(NamespaceXHTML, "li"))
	links := make(manifest.LinkList, 0, len(ols))
	for _, li := range ol.SelectElements(NSSelect(NamespaceXHTML, "li")) {
		l := parseLiElement(li, filePath)
		if l != nil {
			links = append(links, *l)
		}
	}
	return links
}

func parseLiElement(li *xmlquery.Node, filePath string) (link *manifest.Link) {
	if li == nil {
		return nil
	}
	first := li.SelectElement("*") // should be <a>, <span>, or <ol>
	if first == nil {
		return nil
	}
	var title string
	if first.Data != "ol" {
		title = strings.TrimSpace(muchSpaceSuchWowMatcher.ReplaceAllString(first.InnerText(), " "))
	}
	rawHref := first.SelectAttr("href")
	href := "#"
	if first.Data == "a" && rawHref != "" {
		s, err := util.NewHREF(rawHref, filePath).String()
		if err == nil {
			href = s
		}
	}

	children := parseOlElement(li.SelectElement(NSSelect(NamespaceXHTML, "ol")), filePath)
	if len(children) == 0 && (href == "#" || title == "") {
		return nil
	}
	return &manifest.Link{
		Title:    title,
		Href:     href,
		Children: children,
	}
}
