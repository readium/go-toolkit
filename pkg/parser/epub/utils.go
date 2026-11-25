package epub

import (
	"context"
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// xmlNS maps the XML namespace prefixes used by precompiled XPath expressions
// throughout this package to their canonical namespace URIs. Update this map
// when introducing queries against a new namespace.
var xmlNS = map[string]string{
	"opf":       NamespaceOPF,
	"dc":        NamespaceDC,
	"rendition": "http://www.idpf.org/2013/rendition",
	"enc":       NamespaceENC,
	"ds":        NamespaceSIG,
	"comp":      NamespaceCOMP,
	"ncx":       NamespaceNCX,
	"html":      NamespaceXHTML,
	"epub":      NamespaceOPS,
	"smil":      NamespaceSMIL,
	"smil2":     NamespaceSMIL2,
	"ocf":       NamespaceOPC,
}

var xpRootfile = mustCompileNS("/ocf:container/ocf:rootfiles/ocf:rootfile")

func mustCompileNS(expr string) *xpath.Expr {
	e, err := xpath.CompileWithNS(expr, xmlNS)
	if err != nil {
		panic("epub: invalid xpath " + expr + ": " + err.Error())
	}
	return e
}

func GetRootFilePath(ctx context.Context, fetcher fetcher.Fetcher) (url.URL, error) {
	xml, err := readXML(ctx, fetcher, manifest.MustNewHREFFromString("META-INF/container.xml", false))
	if err != nil {
		return nil, errors.Wrap(err, "failed loading container.xml")
	}
	n := xmlquery.QuerySelector(xml, xpRootfile)
	if n == nil {
		return nil, errors.New("rootfile not found in container")
	}
	p := n.SelectAttr("full-path")
	if p == "" {
		return nil, errors.New("no full-path in rootfile")
	}
	u, merr := url.FromEPUBHref(p)
	if merr != nil {
		return nil, errors.Wrap(merr, "failed parsing rootfile full-path")
	}

	return u, nil
}

// GetPackageDocumentPaths returns the paths of the package documents of the
// container, one per rendition: the rootfiles of META-INF/container.xml whose
// media type is the one of package documents, in their order.
// [GetRootFilePath] returns the first rootfile, the default rendition's.
func GetPackageDocumentPaths(ctx context.Context, fetcher fetcher.Fetcher) ([]url.URL, error) {
	xml, rerr := readXML(ctx, fetcher, manifest.MustNewHREFFromString("META-INF/container.xml", false))
	if rerr != nil {
		return nil, errors.Wrap(rerr, "failed loading container.xml")
	}
	var paths []url.URL
	for _, n := range xmlquery.QuerySelectorAll(xml, xpRootfile) {
		if mt := mediatype.MaybeNewOfString(n.SelectAttr("media-type")); mt == nil || !mt.Equal(&mediatype.OPF) {
			continue
		}
		u, err := url.FromEPUBHref(n.SelectAttr("full-path"))
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing rootfile full-path")
		}
		paths = append(paths, u)
	}
	return paths, nil
}

func SelectNodeAttrNs(n *xmlquery.Node, ns, name string) string {
	for _, a := range n.Attr {
		if a.NamespaceURI == ns && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func floatOrNil(raw string) *float64 {
	if raw == "" {
		return nil
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &f
}

func intOrNil(raw string) *int {
	if raw == "" {
		return nil
	}
	i, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &i
}

func nilIntOrZero(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
