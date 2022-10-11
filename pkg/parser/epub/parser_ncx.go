package epub

import (
	"strings"

	"github.com/chocolatkey/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
)

func ParseNCX(document *xmlquery.Node, filePath string) map[string][]manifest.Link {
	toc := document.SelectElement("//*[namespace-uri()='" + NamespaceNCX + "' and local-name()='navMap']")
	pageList := document.SelectElement("//*[namespace-uri()='" + NamespaceNCX + "' and local-name()='pageList']")

	ret := make(map[string][]manifest.Link)
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

func parseNavMapElement(element *xmlquery.Node, filePath string) []manifest.Link {
	var links []manifest.Link
	for _, el := range element.SelectElements("*[namespace-uri()='" + NamespaceNCX + "' and local-name()='navPoint']") {
		if p := parseNavPointElement(el, filePath); p != nil {
			links = append(links, *p)
		}
	}
	return links
}

func parsePageListElement(element *xmlquery.Node, filePath string) []manifest.Link {
	var links []manifest.Link
	for _, el := range element.SelectElements("*[namespace-uri()='" + NamespaceNCX + "' and local-name()='pageTarget']") {
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
	var children []manifest.Link
	for _, el := range element.SelectElements("*[namespace-uri()='" + NamespaceNCX + "' and local-name()='navPoint']") {
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
	tel := element.SelectElement("*[namespace-uri()='" + NamespaceNCX + "' and local-name()='navLabel']/*[namespace-uri()='" + NamespaceNCX + "' and local-name()='text']")
	if tel == nil {
		return ""
	}
	return strings.TrimSpace(muchSpaceSuchWowMatcher.ReplaceAllString(tel.InnerText(), " "))
}

func extractHref(element *xmlquery.Node, filePath string) string {
	el := element.SelectElement("*[namespace-uri()='" + NamespaceNCX + "' and local-name()='content']")
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
