package epub

import (
	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpNCXNavMap   = mustCompileNS("//ncx:navMap")
	xpNCXPageList = mustCompileNS("//ncx:pageList")
)

func ParseNCX(document *xmlquery.Node, filePath url.URL) map[string]manifest.LinkList {
	toc := xmlquery.QuerySelector(document, xpNCXNavMap)
	pageList := xmlquery.QuerySelector(document, xpNCXPageList)

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
	for el := element.FirstChild; el != nil; el = el.NextSibling {
		if !matchesNavigationElement(el, NamespaceNCX, "navPoint") {
			continue
		}
		if p := parseNavPointElement(el, filePath); p != nil {
			if links == nil {
				links = make(manifest.LinkList, 0, countNavigationChildren(element, NamespaceNCX, "navPoint"))
			}
			links = append(links, *p)
		}
	}
	return links
}

func parsePageListElement(element *xmlquery.Node, filePath url.URL) manifest.LinkList {
	links := make([]manifest.Link, 0, countNavigationChildren(element, NamespaceNCX, "pageTarget"))
	for el := element.FirstChild; el != nil; el = el.NextSibling {
		if !matchesNavigationElement(el, NamespaceNCX, "pageTarget") {
			continue
		}
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
	children := parseNavMapElement(element, filePath)
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
	for label := element.FirstChild; label != nil; label = label.NextSibling {
		if matchesNavigationElement(label, NamespaceNCX, "navLabel") {
			if text := firstNavigationChild(label, NamespaceNCX, "text"); text != nil {
				return collapseWhitespace(text.InnerText())
			}
		}
	}
	return ""
}

func extractHref(element *xmlquery.Node, filePath url.URL) url.URL {
	el := firstNavigationChild(element, NamespaceNCX, "content")
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
