package util

import (
	"strings"
	"testing"

	"github.com/andybalholm/cascadia"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

const testDoc = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en">
	<head>
		<title>Section IV: FAIRY STORIES—MODERN FANTASTIC TALES</title>
		<link href="css/epub.css" type="text/css" rel="stylesheet" />
	</head>
	<body>
		 <section id="pgepubid00498">
			 <div class="center"><span epub:type="pagebreak" title="171" id="Page_171">171</span></div>
			 <h3>INTRODUCTORY</h3>
			 
			 <p>The difficulties of classification are very apparent here, and once more it must be noted that illustrative and practical purposes rather than logical ones are served by the arrangement adopted. The modern fanciful story is here placed next to the real folk story instead of after all the groups of folk products. The Hebrew stories at the beginning belong quite as well, perhaps even better, in Section V, while the stories at the end of Section VI shade off into the more modern types of short tales.</p>
			 <p><span>The child's natural literature.</span> The world has lost certain secrets as the price of an advancing civilization.</p>
			 <p>Without discussing the limits of the culture-epoch theory of human development as a complete guide in education, it is clear that the young child passes through a period when his mind looks out upon the world in a manner analogous to that of the folk as expressed in their literature.</p>
		</section>
	</body>
</html>`

func TestCSSSelector(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(testDoc))
	require.NoError(t, err)

	qf := func(query string) string {
		n := cascadia.Query(doc, cascadia.MustCompile(query))
		require.NotNil(t, n)
		return CSSSelector(n)
	}

	assert.Equal(t, qf("body"), "body")
	assert.Equal(t, qf("#pgepubid00498"), "#pgepubid00498")
	assert.Equal(t, qf("#Page_171"), "#Page_171")
	assert.Equal(t, qf("#pgepubid00498 > h3"), "#pgepubid00498 > h3")
	assert.Equal(t, qf("#pgepubid00498 > div.center"), "#pgepubid00498 > div.center")
	assert.Equal(t, qf("#pgepubid00498 > p:nth-child(3)"), "#pgepubid00498 > p:nth-child(3)")
	assert.Equal(t, qf("#pgepubid00498 > p:nth-child(5)"), "#pgepubid00498 > p:nth-child(5)")
	assert.Equal(t, qf("#pgepubid00498 > p:nth-child(4) > span"), "#pgepubid00498 > p:nth-child(4) > span")
}

// Hostile identifiers must never panic the selector generator, and the escaped
// selectors should round-trip: parsing them back finds exactly the source element.
func TestCSSSelectorHostileIdentifiers(t *testing.T) {
	docs := []string{
		`<html><body><div><p class="foo(bar)">x</p><p class="foo(bar)">y</p></div></body></html>`,
		`<html><body><div><p class="123">x</p></div></body></html>`,
		`<html><body><div><p class="--x">x</p></div></body></html>`,
		`<html><body><div><p class="-5x">x</p></div></body></html>`,
		`<html><body><div><p class="a.b">x</p></div></body></html>`,
		`<html><body><div><p class="a:b(c)">x</p></div></body></html>`,
		`<html><body><div><p class="&gt;,+~*">x</p></div></body></html>`,
		`<html><body><div><p class="foo.">x</p></div></body></html>`,
		`<html><body><div><p class="a\b">x</p></div></body></html>`,
		`<html><body><div><p class="émoji😀">x</p></div></body></html>`,
		`<html><body><div><p class="[foo]">x</p><p class="'quo&quot;te'">y</p></div></body></html>`,
		`<html><body><div><epub:switch><epub:case>x</epub:case></epub:switch></div></body></html>`,
	}
	for _, doc := range docs {
		root, err := html.Parse(strings.NewReader(doc))
		require.NoError(t, err)

		// The target is the first <p> (or unknown prefixed element) in the doc
		var target *html.Node
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && (n.Data == "p" || strings.Contains(n.Data, ":")) {
				target = n
				return
			}
			for c := n.FirstChild; c != nil && target == nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(root)
		require.NotNil(t, target, doc)

		sel := CSSSelector(target)
		assert.NotEmpty(t, sel, doc)

		s, err := cascadia.Parse(sel)
		if !assert.NoError(t, err, "generated selector %q should parse", sel) {
			continue
		}
		assert.Equal(t, target, cascadia.Query(root, s), "%q should round-trip", sel)
	}
}
