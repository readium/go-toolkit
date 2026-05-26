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
	xpNavOL   = mustCompileNS("html:ol")
	xpNavLI   = mustCompileNS("html:li")
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

	links := parseOlElement(xmlquery.QuerySelector(nav, xpNavOL), filePath)
	if len(links) > 0 && len(types) > 0 {
		return types, links
	}
	return nil, nil
}

func parseOlElement(ol *xmlquery.Node, filePath url.URL) manifest.LinkList {
	if ol == nil {
		return nil
	}
	lis := xmlquery.QuerySelectorAll(ol, xpNavLI)
	links := make(manifest.LinkList, 0, len(lis))
	for _, li := range lis {
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
	first := li.SelectElement("*") // should be <a>, <span>, or <ol>
	if first == nil {
		return nil
	}
	var title string
	if first.Data != "ol" {
		title = strings.TrimSpace(muchSpaceSuchWowMatcher.ReplaceAllString(first.InnerText(), " "))
	}
	rawHref := first.SelectAttr("href")
	href := url.MustURLFromString("#")
	if first.Data == "a" && rawHref != "" {
		s, err := url.FromEPUBHref(rawHref)
		if err == nil {
			href = filePath.Resolve(s)
		}
	}

	children := parseOlElement(xmlquery.QuerySelector(li, xpNavOL), filePath)
	if len(children) == 0 && (href.String() == "" || title == "") {
		return nil
	}
	return &manifest.Link{
		Title:    title,
		Href:     manifest.NewHREF(href),
		Children: children,
	}
}
