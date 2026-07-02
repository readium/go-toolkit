package converter

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Do(ctx context.Context, resource fetcher.Resource, locator manifest.Locator) (*guidednavigation.GuidedNavigationDocument, error) {
	raw, rerr := fetcher.ReadResourceAsString(ctx, resource)
	if rerr != nil {
		return nil, errors.Wrap(rerr, "failed reading HTML string of "+resource.Link().Href.String())
	}

	var document *html.Node
	xmlParsed := false
	if mt := resource.Link().MediaType; mt != nil && mt.Matches(&mediatype.XHTML) {
		// XHTML is XML: parse it as such, so that e.g. self-closing elements are
		// handled correctly. Ill-formed documents fall back to the HTML parser.
		if doc, err := ParseXHTML(strings.NewReader(raw)); err == nil && childOfType(doc, atom.Body, true) != nil {
			document = doc
			xmlParsed = true
		}
	}
	if document == nil {
		var err error
		document, err = html.ParseWithOptions(
			strings.NewReader(raw),
			html.ParseOptionEnableScripting(false),
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing HTML of "+resource.Link().Href.String())
		}
	}

	body := childOfType(document, atom.Body, true)
	if body == nil {
		return nil, errors.New("HTML of " + resource.Link().Href.String() + " doesn't have a <body>")
	}

	contentConverter := NewHTMLConverter(locator)
	contentConverter.xmlParsed = xmlParsed

	// Traverse the document's HTML
	contentConverter.Convert(body)

	return &guidednavigation.GuidedNavigationDocument{
		Guided: contentConverter.Result(),
	}, nil
}
