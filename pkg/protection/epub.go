package protection

import (
	"context"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
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

func IdentifyEPUBProtection(ctx context.Context, f fetcher.Fetcher) (Scheme, *xmlquery.Node, error) {
	hasLink := func(path string) (*manifest.Link, bool) {
		href, err := manifest.NewHREFFromString(path, false)
		if err != nil {
			return nil, false
		}
		link := manifest.Link{Href: href}
		res := f.Get(ctx, link)
		defer res.Close()
		if _, lerr := res.Length(ctx); lerr != nil {
			return nil, false
		}
		return &link, true
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
		return LCP, nil, nil
	}

	// Apple FairPlay: META-INF/sinf.xml containing <fairplay:sinf>.
	if link, ok := hasLink(pathFairplaySinf); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, nil, derr
		}
		if doc != nil && xmlquery.QuerySelector(doc, xpFairplaySinf) != nil {
			return Fairplay, nil, nil
		}
	}

	// Adobe ADEPT (and Barnes & Noble): META-INF/rights.xml with <adept:operatorURL>.
	if link, ok := hasLink(pathAdeptRights); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, nil, derr
		}
		if doc != nil {
			if op := xmlquery.QuerySelector(doc, xpAdeptOperator); op != nil {
				if strings.Contains(strings.ToLower(op.InnerText()), barnesAndNobleTag) {
					return BarnesAndNoble, nil, nil
				}
				return Adept, nil, nil
			}
		}
	}

	// Kobo: rights.xml at the container root containing <kdrm>.
	if link, ok := hasLink(pathKoboRights); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, nil, derr
		}
		if doc != nil && xmlquery.QuerySelector(doc, xpKdrm) != nil {
			return Kobo, nil, nil
		}
	}

	// Fall back to META-INF/encryption.xml: it may reveal LCP via the
	// retrieval method, or indicate generic/unknown encryption otherwise. The
	// parsed document is handed back so the caller can avoid re-parsing it.
	if link, ok := hasLink(pathEncryption); ok {
		doc, derr := readXML(link)
		if derr != nil {
			return NoDRM, nil, derr
		}
		if doc != nil {
			if xmlquery.QuerySelector(doc, xpLCPRetrieval) != nil {
				return LCP, doc, nil
			}
			return Generic, doc, nil
		}
	}

	return NoDRM, nil, nil
}
