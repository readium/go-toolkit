package util

import (
	"strings"
	"testing"

	"github.com/andybalholm/cascadia"
	"github.com/stretchr/testify/assert"
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
	if !assert.NoError(t, err) {
		return
	}

	qf := func(query string) string {
		n := cascadia.Query(doc, cascadia.MustCompile(query))
		if !assert.NotNil(t, n) {
			t.FailNow()
		}
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
