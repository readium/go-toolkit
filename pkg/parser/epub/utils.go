package epub

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/xmlquery"
)

func GetRootFilePath(fetcher fetcher.Fetcher) (string, error) {
	res := fetcher.Get(manifest.Link{Href: "/META-INF/container.xml"})
	xml, err := res.ReadAsXML(map[string]string{
		"urn:oasis:names:tc:opendocument:xmlns:container": "cn",
	})
	if err != nil {
		return "", errors.Wrap(err, "failed loading container.xml")
	}
	n := xml.SelectElement("/container/rootfiles/rootfile")
	if n == nil {
		return "", errors.New("rootfile not found in container")
	}
	p := n.SelectAttr("full-path")
	if p == "" {
		return "", errors.New("no full-path in rootfile")
	}
	return p, nil
}

func NSSelect(namespace, localName string) string {
	return "*[namespace-uri()='" + namespace + "' and local-name()='" + localName + "']"
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
