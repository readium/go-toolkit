package epub

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
	"github.com/readium/xmlquery"
)

func ParseNCX(document *xmlquery.Node, filePath string) map[string]manifest.LinkList {
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

func parseNavMapElement(element *xmlquery.Node, filePath string) manifest.LinkList {
	var links manifest.LinkList
	for _, el := range element.SelectElements(NSSelect(NamespaceNCX, "navPoint")) {
		if p := parseNavPointElement(el, filePath); p != nil {
			links = append(links, *p)
		}
	}
	return links
}

func parsePageListElement(element *xmlquery.Node, filePath string) manifest.LinkList {
	selectedElements := element.SelectElements(NSSelect(NamespaceNCX, "pageTarget"))
	links := make([]manifest.Link, 0, len(selectedElements))
	for _, el := range selectedElements {
		href := extractHref(el, filePath)
		title := extractTitle(el)
		if href == "" || title == "" {
			continue
		}
		links = append(links, manifest.Link{
			Title: title,
			Href:  href,
		})
	}
	return links
}

func parseNavPointElement(element *xmlquery.Node, filePath string) *manifest.Link {
	title := extractTitle(element)
	href := extractHref(element, filePath)
	var children manifest.LinkList
	for _, el := range element.SelectElements(NSSelect(NamespaceNCX, "navPoint")) {
		if p := parseNavPointElement(el, filePath); p != nil {
			children = append(children, *p)
		}
	}
	if len(children) == 0 && (href == "" || title == "") {
		return nil
	}
	if href == "" {
		href = "#"
	}
	return &manifest.Link{
		Title:    title,
		Href:     href,
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

func extractHref(element *xmlquery.Node, filePath string) string {
	el := element.SelectElement(NSSelect(NamespaceNCX, "content"))
	if el == nil {
		return ""
	}
	src := el.SelectAttr("src")
	if src == "" {
		return ""
	}
	s, _ := util.NewHREF(src, filePath).String()
	return s
}
