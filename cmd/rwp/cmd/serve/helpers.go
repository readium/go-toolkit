package serve

import (
	"net/http"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
)

var mimeSubstitutions = map[string]string{
	"application/vnd.ms-opentype":          "font/otf",                                            // Not just because it's sane, but because CF will compress it!
	"application/vnd.readium.content+json": "application/vnd.readium.content+json; charset=utf-8", // Need utf-8 encoding
}

var compressableMimes = []string{
	"application/javascript",
	"application/x-javascript",
	"image/x-icon",
	"text/css",
	"text/html",
	"application/xhtml+xml",
	"application/webpub+json",
	"application/divina+json",
	"application/vnd.readium.position-list+json",
	"application/vnd.readium.content+json",
	"application/audiobook+json",
	"font/ttf",
	"application/ttf",
	"application/x-ttf",
	"application/x-font-ttf",
	"font/otf",
	"application/otf",
	"application/x-otf",
	"application/vnd.ms-opentype",
	"font/opentype",
	"application/opentype",
	"application/x-opentype",
	"application/truetype",
	"application/font-woff",
	"font/x-woff",
	"application/vnd.ms-fontobject",
}

func makeRelative(link manifest.Link) manifest.Link {
	link.Href = strings.TrimPrefix(link.Href, "/")
	for i, alt := range link.Alternates {
		link.Alternates[i].Href = strings.TrimPrefix(alt.Href, "/")
	}
	return link
}

func conformsToAsMimetype(conformsTo manifest.Profiles) string {
	mime := "application/webpub+json"
	for _, profile := range conformsTo {
		if profile == manifest.ProfileDivina {
			mime = "application/divina+json"
		} else if profile == manifest.ProfileAudiobook {
			mime = "application/audiobook+json"
		} else {
			continue
		}
		break
	}
	return mime
}

func supportsDeflate(r *http.Request) bool {
	vv := r.Header.Values("Accept-Encoding")
	for _, v := range vv {
		for _, sv := range strings.Split(v, ",") {
			coding := parseCoding(sv)
			if coding == "" {
				continue
			}
			if coding == "deflate" {
				return true
			}
		}
	}
	return false
}

func parseCoding(s string) (coding string) {
	p := strings.IndexRune(s, ';')
	if p == -1 {
		p = len(s)
	}
	coding = strings.ToLower(strings.TrimSpace(s[:p]))
	return
}
