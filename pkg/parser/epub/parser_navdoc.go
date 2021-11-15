package epub

import (
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
)

func ParseNavDoc(document *xmlquery.Node, filePath string) map[string][]manifest.Link {
	ret := make(map[string][]manifest.Link)
	docPrefixes := parsePrefixes(document.SelectAttr(NAMESPACE_OPS + ":prefix"))
	for k, v := range CONTENT_RESERVED_PREFIXES {
		if _, ok := docPrefixes[k]; !ok { // prefix element overrides reserved prefixes
			docPrefixes[k] = v
		}
	}

	body := document.SelectElement("//body[namespace-uri()='" + NAMESPACE_XHTML + "']")
	if body == nil {
		return ret
	}

	for _, nav := range body.SelectElements("nav[namespace-uri()='" + NAMESPACE_XHTML + "']") {
		types, links := parseNavElement(nav, filePath, docPrefixes)
		if types == nil && links == nil {
			continue
		}

		for _, t := range types {
			suffix := strings.TrimPrefix(t, VOCABULARY_TYPE)
			if suffix == "toc" || suffix == "page-list" || suffix == "landmarks" || suffix == "lot" || suffix == "loi" || suffix == "loa" || suffix == "lov" {
				ret[suffix] = links
			} else {
				ret[t] = links
			}
		}
	}

	return ret
}

func parseNavElement(nav *xmlquery.Node, filePath string, prefixMap map[string]string) ([]string, []manifest.Link) {
	typeAttr := ""
	for _, na := range nav.Attr {
		if na.NamespaceURI == NAMESPACE_OPS && na.Name.Local == "type" {
			typeAttr = na.Value
			break
		}
	}
	if typeAttr == "" {
		return nil, nil
	}

	var types []string
	for _, prop := range parseProperties(typeAttr) {
		types = append(types, resolveProperty(prop, prefixMap, TYPE))
	}

	links := parseOlElement(nav.SelectElement("ol[namespace-uri()='"+NAMESPACE_XHTML+"']"), filePath)
	if len(links) > 0 && len(types) > 0 {
		return types, links
	}
	return nil, nil
}

func parseOlElement(ol *xmlquery.Node, filePath string) (links []manifest.Link) {
	if ol == nil {
		return nil
	}
	for _, li := range ol.SelectElements("li[namespace-uri()='" + NAMESPACE_XHTML + "']") {
		l := parseLiElement(li, filePath)
		if l != nil {
			links = append(links, *l)
		}
	}
	return
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

	children := parseOlElement(li.SelectElement("ol[namespace-uri()='"+NAMESPACE_XHTML+"']"), filePath)
	if len(children) == 0 && (href == "#" || title == "") {
		return nil
	}
	return &manifest.Link{
		Title:    title,
		Href:     href,
		Children: children,
	}
}
