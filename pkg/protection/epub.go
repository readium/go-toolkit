package protection

import (
	"context"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Well-known XML namespaces used by EPUB DRM containers.
const (
	namespaceENC = "http://www.w3.org/2001/04/xmlenc#"
	namespaceSIG = "http://www.w3.org/2000/09/xmldsig#"
)

// Well-known file paths used by EPUB DRM containers.
const (
	pathLCPLicense    = "META-INF/license.lcpl"
	pathEncryption    = "META-INF/encryption.xml"
	pathAdeptRights   = "META-INF/rights.xml"
	pathFairplaySinf  = "META-INF/sinf.xml"
	pathKoboRights    = "rights.xml"
	lcpRetrievalURI   = "license.lcpl#/encryption/content_key"
	barnesAndNobleTag = "barnesandnoble"
)

var xmlNS = map[string]string{
	"enc":      namespaceENC,
	"ds":       namespaceSIG,
	"adept":    "http://ns.adobe.com/adept",
	"fairplay": "http://itunes.apple.com/ns/epub",
}

var (
	xpLCPRetrieval  = mustCompileNS(`//enc:EncryptedData/ds:KeyInfo/ds:RetrievalMethod[@URI="` + lcpRetrievalURI + `"]`)
	xpAdeptOperator = mustCompileNS("//adept:operatorURL")
	xpFairplaySinf  = mustCompileNS("//fairplay:sinf")
	xpKdrm          = xpath.MustCompile("//kdrm")
)

func mustCompileNS(expr string) *xpath.Expr {
	e, err := xpath.CompileWithNS(expr, xmlNS)
	if err != nil {
		panic("protection: invalid xpath " + expr + ": " + err.Error())
	}
	return e
}

// IdentifyEPUBProtection inspects the well-known DRM metadata files inside an
// EPUB container and returns the detected protection [Scheme]. Returns [NoDRM]
// when no protection metadata is present.
func IdentifyEPUBProtection(ctx context.Context, f fetcher.Fetcher) (Scheme, error) {
	links, err := f.Links(ctx)
	if err != nil {
		return NoDRM, err
	}

	hasLink := func(path string) (*manifest.Link, bool) {
		u, uerr := url.URLFromString(path)
		if uerr != nil {
			return nil, false
		}
		l := links.FirstWithHref(u)
		return l, l != nil
	}

	readXML := func(link *manifest.Link) (*xmlquery.Node, error) {
		doc, rerr := fetcher.ReadResourceAsXML(ctx, f.Get(ctx, *link))
		if rerr != nil {
			if rerr.Code == fetcher.CodeInternalServerError {
				return nil, nil
			}
			return nil, errors.Wrap(rerr.Cause, "unable to read "+link.Href.String())
		}
		return doc, nil
	}

	// LCP: presence of the license file is the strongest signal.
	if _, ok := hasLink(pathLCPLicense); ok {
		return LCP, nil
	}

	// Apple FairPlay: META-INF/sinf.xml containing <fairplay:sinf>.
	if link, ok := hasLink(pathFairplaySinf); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, derr
		}
		if doc != nil && xmlquery.QuerySelector(doc, xpFairplaySinf) != nil {
			return Fairplay, nil
		}
	}

	// Adobe ADEPT (and Barnes & Noble): META-INF/rights.xml with <adept:operatorURL>.
	if link, ok := hasLink(pathAdeptRights); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, derr
		}
		if doc != nil {
			if op := xmlquery.QuerySelector(doc, xpAdeptOperator); op != nil {
				if strings.Contains(strings.ToLower(op.InnerText()), barnesAndNobleTag) {
					return BarnesAndNoble, nil
				}
				return Adept, nil
			}
		}
	}

	// Kobo: rights.xml at the container root containing <kdrm>.
	if link, ok := hasLink(pathKoboRights); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, derr
		}
		if doc != nil && xmlquery.QuerySelector(doc, xpKdrm) != nil {
			return Kobo, nil
		}
	}

	// Fall back to META-INF/encryption.xml: it may reveal LCP via the
	// retrieval method, or indicate generic/unknown encryption otherwise.
	if link, ok := hasLink(pathEncryption); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, derr
		}
		if doc != nil {
			if xmlquery.QuerySelector(doc, xpLCPRetrieval) != nil {
				return LCP, nil
			}
			return Generic, nil
		}
	}

	return NoDRM, nil
}
