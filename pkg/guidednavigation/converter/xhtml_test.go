package converter

import (
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const xhtmlSample = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en">
<head><title>Test</title></head>
<body>
<p>Caf&eacute;&nbsp;<span epub:type="pagebreak" title="4"/>after</p>
<p xml:lang="fr">Bonjour</p>
</body>
</html>`

func TestParseXHTMLSelfClosingSpan(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(xhtmlSample))
	require.NoError(t, err)

	body := childOfType(doc, atom.Body, true)
	require.NotNil(t, body)

	span := childOfType(doc, atom.Span, true)
	require.NotNil(t, span)
	// The self-closing span must be empty, not swallow the text after it
	assert.Nil(t, span.FirstChild, "self-closing span should have no children")
	require.NotNil(t, span.NextSibling)
	assert.Equal(t, "after", span.NextSibling.Data)
}

// Attributes must use the same representation html.Parse produces:
// the whole qualified name in Key, and an empty Namespace.
func TestParseXHTMLAttributeCompatibility(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(xhtmlSample))
	require.NoError(t, err)

	span := childOfType(doc, atom.Span, true)
	require.NotNil(t, span)
	assert.Equal(t, "pagebreak", getAttr(span, "epub:type"))
	assert.Equal(t, "4", getAttr(span, "title"))

	htmlNode := childOfType(doc, atom.Html, true)
	require.NotNil(t, htmlNode)
	assert.Equal(t, "en", getAttr(htmlNode, "xml:lang"))
	assert.Equal(t, "http://www.idpf.org/2007/ops", getAttr(htmlNode, "xmlns:epub"))

	for _, attr := range span.Attr {
		assert.Empty(t, attr.Namespace, "attribute namespaces must be empty for html.Parse compatibility")
	}
}

func TestParseXHTMLEntities(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(xhtmlSample))
	require.NoError(t, err)

	p := childOfType(doc, atom.P, true)
	require.NotNil(t, p)
	require.NotNil(t, p.FirstChild)
	assert.Equal(t, "Café ", p.FirstChild.Data)
}

func TestParseXHTMLForeignContent(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(`<?xml version="1.0"?>
		<html xmlns="http://www.w3.org/1999/xhtml"><body>
		<svg xmlns="http://www.w3.org/2000/svg"><title>Icon</title></svg>
		<math xmlns="http://www.w3.org/1998/Math/MathML" alttext="x"><mi>x</mi></math>
		</body></html>`))
	require.NoError(t, err)

	svg := childOfType(doc, atom.Svg, true)
	require.NotNil(t, svg)
	assert.Equal(t, "svg", svg.Namespace)

	math := childOfType(doc, atom.Math, true)
	require.NotNil(t, math)
	assert.Equal(t, "math", math.Namespace)
	assert.Equal(t, "x", getAttr(math, "alttext"))
}

func TestParseXHTMLCDATAAndComments(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(`<?xml version="1.0"?>
		<html xmlns="http://www.w3.org/1999/xhtml"><body>
		<p><!-- a comment -->before<![CDATA[ & cdata]]></p>
		</body></html>`))
	require.NoError(t, err)

	p := childOfType(doc, atom.P, true)
	require.NotNil(t, p)
	require.NotNil(t, p.FirstChild)
	// Comments are dropped; CDATA merges with adjacent text
	assert.Equal(t, "before & cdata", p.FirstChild.Data)
	assert.Equal(t, html.TextNode, p.FirstChild.Type)
	assert.Nil(t, p.FirstChild.NextSibling)
}

// Namespace prefixes keep their case consistently between the xmlns declaration
// and reconstructed attribute names.
func TestParseXHTMLUppercasePrefix(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(`<?xml version="1.0"?>
		<html xmlns="http://www.w3.org/1999/xhtml" xmlns:EPUB="http://www.idpf.org/2007/ops">
		<body><span EPUB:type="pagebreak" title="4"/></body></html>`))
	require.NoError(t, err)

	span := childOfType(doc, atom.Span, true)
	require.NotNil(t, span)
	assert.Equal(t, "pagebreak", getAttr(span, "EPUB:type"))

	htmlNode := childOfType(doc, atom.Html, true)
	require.NotNil(t, htmlNode)
	assert.Equal(t, "http://www.idpf.org/2007/ops", getAttr(htmlNode, "xmlns:EPUB"))

	// The role extraction resolves the prefix through the declaration
	roles := ExtractNodeRoles(span)
	assert.Contains(t, roles, guidednavigation.RolePagebreak)
}

// The input is already UTF-8, whatever encoding the XML declaration announces.
func TestParseXHTMLForeignEncodingDeclaration(t *testing.T) {
	doc, err := ParseXHTML(strings.NewReader(`<?xml version="1.0" encoding="iso-8859-1"?>
		<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Café</p></body></html>`))
	require.NoError(t, err)
	p := childOfType(doc, atom.P, true)
	require.NotNil(t, p)
	assert.Equal(t, "Café", p.FirstChild.Data)
}

func TestParseXHTMLMalformed(t *testing.T) {
	// An unclosed element at EOF is a hard error, triggering the html.Parse fallback in Do
	_, err := ParseXHTML(strings.NewReader(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Unclosed`))
	require.Error(t, err)
}
