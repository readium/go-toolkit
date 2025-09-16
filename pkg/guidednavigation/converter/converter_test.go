package converter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/require"
)

func TestDo(t *testing.T) {
	f := fetcher.NewBytesResource(manifest.Link{
		Href: manifest.MustNewHREFFromString("hello.xhtml", false),
	}, func() []byte {
		return []byte(`
		<!doctype html>
		<html xmlns:epub="http://www.idpf.org/2007/ops"><!-- lang="en" xml:lang="en" -->
		<body>
			<p xml:lang="fr">Paragraphe avec image: <img src="src/image.jpg" alt="A cool image" /></p>
			<p>This job requires a certain <em xml:lang="fr">savoir faire</em> that can only be acquired over time.</p>
			<p>This is a paragraph <b>with some very-<em>strong</em> bold</b> text!</p>

			<div>
			<span id="pg04" role="doc-pagebreak" epub:type="pagebreak" title="4"/>
			<p>And the next pagebreak is in the middle <span id="pg05" role="doc-pagebreak" epub:type="pagebreak" title="4"/> of a sentence.</p>
			</div>


			<section role="doc-chapter" epub:type="chapter">
				<h1>Title of the chapter</h1>
			</section>
			<ul>
				<li>First item</li>
				<li>Second item</li>
				<li>Third item</li>
			</ul>
			<p aria-hidden="true">Hidden <b>text!</b> <img src="with_image.jpg" />...</p>

			<img src="image1.avif" alt="Alternative text using the alt attribute">
			<span role="img" aria-label="Rating: 4 out of 5 stars">
				<span>★</span>
				<span>★</span>
				<span>★</span>
				<span>★</span>
				<span>☆</span>
			</span>
			<figure aria-labelledby="cat-caption"> 
				<pre>
				 /\_/\
				( o.o )
				   ^ 
				</pre>
				<figcaption id="cat-caption">
				ASCII Art of a cat face
				</figcaption>
			</figure>
		</body>
		</html>`)
	})

	nav, err := Do(context.Background(), f, manifest.Locator{
		Href: f.Link().Href.Resolve(nil, nil),
	})
	require.NoError(t, err)
	bin, _ := json.Marshal(nav)
	t.Log(string(bin))
}
