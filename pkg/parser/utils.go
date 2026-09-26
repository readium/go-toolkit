package parser

import (
	"context"
	"strings"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

func hrefCommonFirstComponent(links manifest.LinkList) string {
	latest := ""
	for _, link := range links {
		normalized, _, _ := strings.Cut(link.URL(nil, nil).Path(), "/")
		if latest != "" {
			if latest != normalized {
				latest = "" // No distinct prefix
				break
			}
		}
		latest = normalized
	}
	return latest
}

func GuessPublicationTitleFromFileStructure(ctx context.Context, fetcher fetcher.Fetcher) string { // TODO test for this
	links, err := fetcher.Links(ctx)
	if err != nil || len(links) == 0 {
		return ""
	}
	commonFirstComponent := hrefCommonFirstComponent(links)
	if commonFirstComponent == "" {
		return ""
	}
	if commonFirstComponent == links[0].Href.String() {
		return ""
	}

	return commonFirstComponent
}
