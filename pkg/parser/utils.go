package parser

import (
	"strings"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

func hrefCommonFirstComponent(links manifest.LinkList) string {
	latest := ""
	for _, link := range links {
		normalized := strings.SplitN(link.URL(nil, nil).Path(), "/", 2)[0]
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
	if commonFirstComponent == links[0].Href.String() {
		return ""
	}

	return commonFirstComponent
}

func isMediatypeReadiumWebPubProfile(mt mediatype.MediaType) bool {
	return mt.Matches(
		&mediatype.ReadiumWebpub, &mediatype.ReadiumWebpubManifest,
		&mediatype.ReadiumAudiobook, &mediatype.ReadiumAudiobookManifest, &mediatype.LCPProtectedAudiobook,
		&mediatype.ReadiumDivina, &mediatype.ReadiumDivinaManifest, &mediatype.LCPProtectedPDF,
	)
}
