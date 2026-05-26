package epub

import (
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpNCXNavMap    = mustCompileNS("//ncx:navMap")
	xpNCXPageList  = mustCompileNS("//ncx:pageList")
	xpNCXNavPoint  = mustCompileNS("ncx:navPoint")
	xpNCXPageTgt   = mustCompileNS("ncx:pageTarget")
	xpNCXContent   = mustCompileNS("ncx:content")
	xpNCXNavLblTxt = mustCompileNS("ncx:navLabel/ncx:text")
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
	for _, el := range xmlquery.QuerySelectorAll(element, xpNCXNavPoint) {
		if p := parseNavPointElement(el, filePath); p != nil {
			links = append(links, *p)
		}
	}
	return links
}

func parsePageListElement(element *xmlquery.Node, filePath url.URL) manifest.LinkList {
	selectedElements := xmlquery.QuerySelectorAll(element, xpNCXPageTgt)
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
	for _, el := range xmlquery.QuerySelectorAll(element, xpNCXNavPoint) {
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
	tel := xmlquery.QuerySelector(element, xpNCXNavLblTxt)
	if tel == nil {
		return ""
	}
	return strings.TrimSpace(muchSpaceSuchWowMatcher.ReplaceAllString(tel.InnerText(), " "))
}

func extractHref(element *xmlquery.Node, filePath url.URL) url.URL {
	el := xmlquery.QuerySelector(element, xpNCXContent)
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
