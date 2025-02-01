package epub

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/readium/xmlquery"
)

func ParseNCX(document *xmlquery.Node, filePath url.URL) map[string]manifest.LinkList {
	toc := document.SelectElement("//" + NSSelect(NamespaceNCX, "navMap"))
	pageList := document.SelectElement("//" + NSSelect(NamespaceNCX, "pageList"))

	ret := make(map[string]manifest.LinkList)
	if toc != nil {
		p := parseNavMapElement(toc, filePath)
		if len(p) > 0 {
			ret["toc"] = p
		}
	}
	if pageList != nil {
		p := parsePageListElement(pageList, filePath)
		if len(p) > 0 {
			ret["page-list"] = p
		}
	}

	return ret
}

func parseNavMapElement(element *xmlquery.Node, filePath url.URL) manifest.LinkList {
	var links manifest.LinkList
	for _, el := range element.SelectElements(NSSelect(NamespaceNCX, "navPoint")) {
		if p := parseNavPointElement(el, filePath); p != nil {
			links = append(links, *p)
		}
	}
	return links
}

func parsePageListElement(element *xmlquery.Node, filePath url.URL) manifest.LinkList {
	selectedElements := element.SelectElements(NSSelect(NamespaceNCX, "pageTarget"))
	links := make([]manifest.Link, 0, len(selectedElements))
	for _, el := range selectedElements {
		href := extractHref(el, filePath)
		title := extractTitle(el)
		if href == nil || title == "" {
			continue
		}
		links = append(links, manifest.Link{
			Title: title,
			Href:  manifest.NewHREF(href),
		})
	}
	return links
}

func parseNavPointElement(element *xmlquery.Node, filePath url.URL) *manifest.Link {
	title := extractTitle(element)
	href := extractHref(element, filePath)
	var children manifest.LinkList
	for _, el := range element.SelectElements(NSSelect(NamespaceNCX, "navPoint")) {
		if p := parseNavPointElement(el, filePath); p != nil {
			children = append(children, *p)
		}
	}
	if len(children) == 0 && (href == nil || title == "") {
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

func extractTitle(element *xmlquery.Node) string {
	tel := element.SelectElement(NSSelect(NamespaceNCX, "navLabel") + "/" + NSSelect(NamespaceNCX, "text"))
	if tel == nil {
		return ""
	}
	return strings.TrimSpace(muchSpaceSuchWowMatcher.ReplaceAllString(tel.InnerText(), " "))
}

func extractHref(element *xmlquery.Node, filePath url.URL) url.URL {
	el := element.SelectElement(NSSelect(NamespaceNCX, "content"))
	if el == nil {
		return nil
	}
	src := el.SelectAttr("src")
	if src == "" {
		return nil
	}

	if s, err := url.FromEPUBHref(src); err == nil {
		return filePath.Resolve(s)
	}
	return nil
}
