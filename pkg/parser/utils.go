package parser

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

func hrefCommonFirstComponent(links []manifest.Link) string {
	latest := ""
	for _, link := range links {
		normalized := strings.SplitN(strings.TrimPrefix(link.Href, "/"), "/", 2)[0]
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

func guessPublicationTitleFromFileStructure(fetcher fetcher.Fetcher) string { // TODO test for this
	links, err := fetcher.Links()
	if err != nil || len(links) == 0 {
		return ""
	}
	commonFirstComponent := hrefCommonFirstComponent(links)
	if commonFirstComponent == "" {
		return ""
	}
	if commonFirstComponent == strings.TrimPrefix("/", links[0].Href) {
		return ""
	}

	return commonFirstComponent
}

func isMediatypeReadiumWebPubProfile(mt mediatype.MediaType) bool {
	return mt.Matches(
		&mediatype.READIUM_WEBPUB, &mediatype.READIUM_WEBPUB_MANIFEST,
		&mediatype.READIUM_AUDIOBOOK, &mediatype.READIUM_AUDIOBOOK_MANIFEST, &mediatype.LCP_PROTECTED_AUDIOBOOK,
		&mediatype.DIVINA, &mediatype.DIVINA_MANIFEST, &mediatype.LCP_PROTECTED_PDF,
	)
}
