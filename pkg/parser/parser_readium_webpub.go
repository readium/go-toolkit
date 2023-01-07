package parser

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

type WebPubParser struct {
	client *http.Client
	// pdfFactory may never be needed
}

func NewWebPubParser(client *http.Client) WebPubParser {
	return WebPubParser{
		client: client,
	}
}

// Parse implements PublicationParser
func (p WebPubParser) Parse(asset asset.PublicationAsset, fetcher fetcher.Fetcher) (*pub.Builder, error) {
	lFetcher := fetcher
	mediaType := asset.MediaType()

	if !isMediatypeReadiumWebPubProfile(mediaType) {
		return nil, nil
	}

	isPackage := !mediaType.IsRwpm()

	var manifestJSON map[string]interface{}
	if isPackage {
		res := lFetcher.Get(manifest.Link{Href: "/manifest.json"})
		mjr, err := res.ReadAsJSON()
		if err != nil {
			return nil, err
		}
		manifestJSON = mjr
	} else {
		// For a single manifest file, reads the first (and only) file in the fetcher.
		links, err := lFetcher.Links()
		if err != nil {
			return nil, err
		}
		if len(links) == 0 {
			return nil, errors.New("links is empty")
		}
		manifestJSON, err = lFetcher.Get(links[0]).ReadAsJSON()
		if err != nil {
			return nil, err
		}
	}

	manifest, err := manifest.ManifestFromJSON(manifestJSON, isPackage)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing RWPM Manifest")
	}

	// For a manifest, we discard the [fetcher] provided by the Streamer, because it was only
	// used to read the manifest file. We use an [HttpFetcher] instead to serve the remote resources.
	if !isPackage {
		/*baseURL := ""
		if link := manifest.LinkWithRel("self"); link != nil {
			baseURL = path.Base(link.Href)
		}*/

		panic("remote HttpFetcher not implemented!")

		lFetcher = fetcher // TODO HttpFetcher using p.client
	}

	// Checks the requirements from the LCPDF specification.
	// https://readium.org/lcp-specs/notes/lcp-for-pdf.html
	readingOrder := manifest.ReadingOrder
	if mediaType.Equal(&mediatype.LCPProtectedPDF) && (len(readingOrder) == 0 || !readingOrder.AllMatchMediaType(&mediatype.PDF)) {
		return nil, errors.New("invalid LCP protected PDF")
	}

	return pub.NewBuilder(*manifest, lFetcher, nil), nil // TODO services!
}
