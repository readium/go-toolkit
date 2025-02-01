package serve

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

var mimeSubstitutions = map[string]string{
	"application/vnd.ms-opentype": "font/otf", // Not just because it's sane, but because CF will compress it!
}

var utfCharsetNeeded = []string{
	mediatype.ReadiumWebpubManifest.String(),
	mediatype.ReadiumDivinaManifest.String(),
	mediatype.ReadiumAudiobookManifest.String(),
	mediatype.ReadiumPositionList.String(),
	mediatype.ReadiumContentDocument.String(),
	mediatype.ReadiumGuidedNavigationDocument.String(),
}

var compressableMimes = []string{
	"application/javascript",
	"application/x-javascript",
	"image/x-icon",
	"text/css",
	"text/html",
	"application/xhtml+xml",
	mediatype.ReadiumWebpubManifest.String(),
	mediatype.ReadiumDivinaManifest.String(),
	mediatype.ReadiumPositionList.String(),
	mediatype.ReadiumContentDocument.String(),
	mediatype.ReadiumAudiobookManifest.String(),
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

func conformsToAsMimetype(conformsTo manifest.Profiles) mediatype.MediaType {
	mime := mediatype.ReadiumWebpubManifest
	for _, profile := range conformsTo {
		if profile == manifest.ProfileDivina {
			mime = mediatype.ReadiumDivinaManifest
		} else if profile == manifest.ProfileAudiobook {
			mime = mediatype.ReadiumAudiobookManifest
		} else {
			continue
		}
		break
	}
	return mime
}

func supportsEncoding(r *http.Request, encoding string) bool {
	vv := r.Header.Values("Accept-Encoding")
	for _, v := range vv {
		for _, sv := range strings.Split(v, ",") {
			coding := parseCoding(sv)
			if coding == "" {
				continue
			}
			if coding == encoding {
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

func convertURLValuesToMap(values url.Values) map[string]string {
	result := make(map[string]string)
	for key, val := range values {
		if len(val) > 0 {
			result[key] = val[0] // Take the first value for each key
		}
	}
	return result
}
