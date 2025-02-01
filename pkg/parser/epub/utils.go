package epub

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/readium/xmlquery"
)

func GetRootFilePath(fetcher fetcher.Fetcher) (url.URL, error) {
	res := fetcher.Get(manifest.Link{Href: manifest.MustNewHREFFromString("META-INF/container.xml", false)})
	xml, err := res.ReadAsXML(map[string]string{
		"urn:oasis:names:tc:opendocument:xmlns:container": "cn",
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed loading container.xml")
	}
	n := xml.SelectElement("/container/rootfiles/rootfile")
	if n == nil {
		return nil, errors.New("rootfile not found in container")
	}
	p := n.SelectAttr("full-path")
	if p == "" {
		return nil, errors.New("no full-path in rootfile")
	}
	u, merr := url.FromEPUBHref(p)
	if merr != nil {
		return nil, errors.Wrap(err, "failed parsing rootfile full-path")
	}

	return u, nil
}

// TODO: Use updated xpath/xmlquery functions
func NSSelect(namespace, localName string) string {
	return "*[namespace-uri()='" + namespace + "' and local-name()='" + localName + "']"
}

// TODO: Use updated xpath/xmlquery functions
func DualNSSelect(namespace1, namespace2, localName string) string {
	return "*[(namespace-uri()='" + namespace1 + "' or namespace-uri()='" + namespace2 + "') and local-name()='" + localName + "']"
}

// TODO: Use updated xpath/xmlquery functions
func ManyNSSelectMany(namespaces []string, localNames []string) string {
	if len(namespaces) == 0 || len(localNames) == 0 {
		panic("namespaces and localNames must not be empty")
	}

	var sb strings.Builder
	sb.WriteString("*[(")
	for i, ns := range namespaces {
		if i > 0 {
			sb.WriteString(" or ")
		}
		sb.WriteString("namespace-uri()='")
		sb.WriteString(ns)
		sb.WriteString("'")
	}
	sb.WriteString(") and (")
	for i, ln := range localNames {
		if i > 0 {
			sb.WriteString(" or ")
		}
		sb.WriteString("local-name()='")
		sb.WriteString(ln)
		sb.WriteString("'")
	}
	sb.WriteString(")]")

	return sb.String()
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
