package converter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func convertDoc(t *testing.T, doc string, mt *mediatype.MediaType) string {
	t.Helper()
	f := fetcher.NewBytesResource(manifest.Link{
		Href:      manifest.MustNewHREFFromString("hello.xhtml", false),
		MediaType: mt,
	}, func() []byte {
		return []byte(doc)
	})

	nav, err := Do(context.Background(), f, manifest.Locator{
		Href: f.Link().Href.Resolve(nil, nil),
	})
	require.NoError(t, err)
	bin, err := json.Marshal(nav)
	require.NoError(t, err)
	return string(bin)
}

func wrapXHTML(bodyAttrs, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Test</title></head>
<body` + bodyAttrs + `>` + body + `</body>
</html>`
}

// Convert an XHTML body (through the XML parsing path) to a guided navigation JSON string.
func convertBody(t *testing.T, body string) string {
	t.Helper()
	return convertDoc(t, wrapXHTML("", body), &mediatype.XHTML)
}

// The expected guided navigation document JSON for the given children of <body>.
func guidedJSON(children string) string {
	res := `{"guided":[{"role":["body"],"textref":"hello.xhtml"`
	if children != "" {
		res += `,"children":[` + children + `]`
	}
	return res + `}]}`
}

// Canonical example from the specification's read-aloud examples: sections and headers.
func TestConvertSectionAndHeading(t *testing.T) {
	res := convertBody(t, `
		<section role="doc-chapter" epub:type="chapter">
			<h1>Title of the chapter</h1>
		</section>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["section", "chapter"],
		"children": [
			{"role": ["heading1"], "text": "Title of the chapter"}
		]
	}`), res)
}

func TestConvertHeadingLevels(t *testing.T) {
	res := convertBody(t, `
		<h2>Two</h2>
		<h6>Six</h6>
		<p role="heading" aria-level="4">Four</p>
		<div role="heading">Default</div>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["heading2"], "text": "Two"},
		{"role": ["heading6"], "text": "Six"},
		{"role": ["paragraph", "heading4"], "text": "Four"},
		{"role": ["heading2"], "text": "Default"}`), res)
}

// Canonical example from the specification: a simple paragraph.
func TestConvertParagraph(t *testing.T) {
	res := convertBody(t, `<p>It is a truth universally acknowledged, that a single man in possession of a good fortune, must be in want of a wife.</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": "It is a truth universally acknowledged, that a single man in possession of a good fortune, must be in want of a wife."
	}`), res)
}

// Canonical example from the specification: multilingual text using SSML.
func TestConvertMultilingualText(t *testing.T) {
	res := convertDoc(t, `<?xml version="1.0" encoding="UTF-8"?>
		<html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="en">
		<head><title>Test</title></head>
		<body>
			<p>This job requires a certain <span xml:lang="fr">savoir faire</span> that can only be acquired over time.</p>
			<p>This job requires a certain <em xml:lang="fr">savoir faire</em> that can only be acquired over time.</p>
		</body>
		</html>`, &mediatype.XHTML)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "This job requires a certain savoir faire that can only be acquired over time.",
			"ssml": "This job requires a certain <lang xml:lang=\"fr\">savoir faire</lang> that can only be acquired over time.",
			"language": "en"
		}
	},
	{
		"role": ["paragraph"],
		"text": {
			"plain": "This job requires a certain savoir faire that can only be acquired over time.",
			"ssml": "This job requires a certain <emphasis xml:lang=\"fr\">savoir faire</emphasis> that can only be acquired over time.",
			"language": "en"
		}
	}`), res)
}

// A paragraph entirely in another language needs no SSML, just a language.
func TestConvertSingleForeignLanguage(t *testing.T) {
	res := convertBody(t, `<p xml:lang="fr">Tout à fait.</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {"plain": "Tout à fait.", "language": "fr"}
	}`), res)
}

// Regression test: emphasis must wrap the emphasized segment, not the text before it,
// and nested emphasis elements with the same SSML mapping merge into one.
func TestConvertEmphasis(t *testing.T) {
	res := convertBody(t, `
		<p>This is a paragraph <b>with some very-<em>strong</em> bold</b> text!</p>
		<p><i>Reduced paragraph</i></p>
		<p>Very <strong>strong</strong> words</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "This is a paragraph with some very-strong bold text!",
			"ssml": "This is a paragraph <emphasis>with some very-strong bold</emphasis> text!"
		}
	},
	{
		"role": ["paragraph"],
		"text": {
			"plain": "Reduced paragraph",
			"ssml": "<emphasis level=\"reduced\">Reduced paragraph</emphasis>"
		}
	},
	{
		"role": ["paragraph"],
		"text": {
			"plain": "Very strong words",
			"ssml": "Very <emphasis level=\"strong\">strong</emphasis> words"
		}
	}`), res)
}

// Canonical example from the specification: images.
func TestConvertImages(t *testing.T) {
	res := convertBody(t, `
		<img src="image1.avif" alt="Alternative text using the alt attribute"/>
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
		</figure>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["image"], "imgref": "image1.avif", "description": "Alternative text using the alt attribute"},
		{"role": ["image"], "description": "Rating: 4 out of 5 stars"},
		{"role": ["figure"], "description": "ASCII Art of a cat face"}`), res)
}

// An image in the middle of a sentence becomes a child object referenced
// from the text with a custom SSML tag.
func TestConvertInlineImage(t *testing.T) {
	res := convertBody(t, `<p xml:lang="fr">Paragraphe avec image: <img src="src/image.jpg" alt="A cool image"/></p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Paragraphe avec image:",
			"ssml": "Paragraphe avec image: <readium:image id=\"image1\"/>",
			"language": "fr"
		},
		"children": [
			{"role": ["image"], "id": "image1", "imgref": "src/image.jpg", "description": "A cool image"}
		]
	}`), res)
}

func TestConvertMultipleInlineImages(t *testing.T) {
	res := convertBody(t, `
		<p xml:lang="fr">Paragraphe avec image #1 <img src="src/image.jpg" alt="A cool image"/> et #2 <img src="src/image.jpg" alt="A second cool image"/>!</p>
		<p xml:lang="fr"><img src="src/image.jpg" alt="The coolest image"/> et <img src="src/image.jpg" alt="The boring image"/></p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Paragraphe avec image #1 et #2!",
			"ssml": "Paragraphe avec image #1 <readium:image id=\"image1\"/> et #2 <readium:image id=\"image2\"/>!",
			"language": "fr"
		},
		"children": [
			{"role": ["image"], "id": "image1", "imgref": "src/image.jpg", "description": "A cool image"},
			{"role": ["image"], "id": "image2", "imgref": "src/image.jpg", "description": "A second cool image"}
		]
	},
	{
		"role": ["paragraph"],
		"text": {
			"plain": "et",
			"ssml": "<readium:image id=\"image3\"/> et <readium:image id=\"image4\"/>",
			"language": "fr"
		},
		"children": [
			{"role": ["image"], "id": "image3", "imgref": "src/image.jpg", "description": "The coolest image"},
			{"role": ["image"], "id": "image4", "imgref": "src/image.jpg", "description": "The boring image"}
		]
	}`), res)
}

// Decorative images don't produce any output.
func TestConvertDecorativeImages(t *testing.T) {
	res := convertBody(t, `
		<p>Before <img src="deco.png" alt=""/> after</p>
		<img src="deco2.png" role="presentation" alt="ignored"/>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": "Before after"
	}`), res)
}

// A figure without an accessible name keeps its content, including the caption.
func TestConvertUnlabelledFigure(t *testing.T) {
	res := convertBody(t, `
		<figure>
			<img src="chart.png" alt="A chart"/>
			<figcaption>Sales over time</figcaption>
		</figure>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["figure"],
		"children": [
			{"role": ["image"], "imgref": "chart.png", "description": "A chart"},
			{"role": ["caption"], "text": "Sales over time"}
		]
	}`), res)
}

// A labelled figure keeps media children, but its text content is
// replaced by the description.
func TestConvertLabelledFigureKeepsImage(t *testing.T) {
	res := convertBody(t, `
		<figure aria-label="A described figure">
			<img src="chart.png" alt="A chart"/>
			<figcaption>Dropped caption</figcaption>
		</figure>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["figure"],
		"description": "A described figure",
		"children": [
			{"role": ["image"], "imgref": "chart.png", "description": "A chart"}
		]
	}`), res)
}

// Canonical example from the specification's read-aloud examples: pagebreaks,
// standalone and in the middle of a sentence. Uses self-closing <span/>, which
// exercises the XML parsing path.
func TestConvertPagebreaks(t *testing.T) {
	res := convertBody(t, `
		<span id="pg04" role="doc-pagebreak" epub:type="pagebreak" title="4"/>
		<p>And the next pagebreak is in the middle <span id="pg05" role="doc-pagebreak" epub:type="pagebreak" title="5"/> of a sentence.</p>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["pagebreak"], "text": "4", "textref": "hello.xhtml#pg04"},
		{
			"role": ["paragraph"],
			"text": {
				"plain": "And the next pagebreak is in the middle of a sentence.",
				"ssml": "And the next pagebreak is in the middle <readium:pagebreak id=\"pg05\"/> of a sentence."
			},
			"children": [
				{"role": ["pagebreak"], "id": "pg05", "text": "5", "textref": "hello.xhtml#pg05"}
			]
		}`), res)
}

// The same self-closing pagebreak <span/> parsed as HTML swallows the following
// content into the span: the pagebreak object must still be emitted and the
// swallowed content must not be lost.
func TestConvertPagebreaksHTMLParser(t *testing.T) {
	res := convertDoc(t, `<!DOCTYPE html>
		<html><head><title>Test</title></head>
		<body>
			<div>
			<span id="pg04" role="doc-pagebreak" title="4"/>
			<p>And the next pagebreak is in the middle <span id="pg05" role="doc-pagebreak" title="5"/> of a sentence.</p>
			</div>
		</body>
		</html>`, &mediatype.HTML)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["pagebreak"], "text": "4", "textref": "hello.xhtml#pg04"},
		{
			"role": ["paragraph"],
			"text": {
				"plain": "And the next pagebreak is in the middle of a sentence.",
				"ssml": "And the next pagebreak is in the middle <readium:pagebreak id=\"pg05\"/> of a sentence."
			},
			"children": [
				{"role": ["pagebreak"], "id": "pg05", "text": "5", "textref": "hello.xhtml#pg05"}
			]
		}`), res)
}

// Canonical example from the specification's read-aloud examples: footnotes and endnotes.
// The same-resource footnote is embedded in the noteref and removed from the normal flow;
// the endnote in another resource is referenced by textref.
func TestConvertNoterefs(t *testing.T) {
	res := convertBody(t, `
		<p>This text has a footnote in the same resource <a href="#note1" epub:type="noteref" role="doc-noteref">[1]</a> and an endnote <a href="endnote.xhtml#note2" epub:type="noteref" role="doc-noteref">[2]</a>.</p>
		<p>This is a paragraph without a footnote.</p>
		<aside id="note1" epub:type="footnote" role="doc-footnote">Text of the footnote</aside>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "This text has a footnote in the same resource and an endnote.",
			"ssml": "This text has a footnote in the same resource <readium:noteref id=\"note1\"/> and an endnote <readium:noteref id=\"note2\"/>."
		},
		"children": [
			{
				"role": ["noteref"],
				"id": "note1",
				"text": "[1]",
				"children": [
					{"role": ["aside", "footnote"], "text": "Text of the footnote", "textref": "hello.xhtml#note1"}
				]
			},
			{
				"role": ["noteref"],
				"id": "note2",
				"text": "[2]",
				"children": [
					{"textref": "endnote.xhtml#note2"}
				]
			}
		]
	},
	{
		"role": ["paragraph"],
		"text": "This is a paragraph without a footnote."
	}`), res)
}

// EPUB 3 footnotes are commonly hidden (they're meant to be shown in popups):
// they must still be embedded in their noteref.
func TestConvertHiddenFootnote(t *testing.T) {
	res := convertBody(t, `
		<p>Some text<a href="#note1" epub:type="noteref">1</a>.</p>
		<aside id="note1" epub:type="footnote" hidden="hidden"><p>Hidden footnote</p></aside>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Some text.",
			"ssml": "Some text<readium:noteref id=\"note1\"/>."
		},
		"children": [
			{
				"role": ["noteref"],
				"id": "note1",
				"text": "1",
				"children": [
					{
						"role": ["aside", "footnote"],
						"textref": "hello.xhtml#note1",
						"children": [{"role": ["paragraph"], "text": "Hidden footnote"}]
					}
				]
			}
		]
	}`), res)
}

// Canonical example from the specification's read-aloud examples: lists.
func TestConvertList(t *testing.T) {
	res := convertBody(t, `
		<ul>
			<li>First item</li>
			<li>Second item</li>
			<li>Third item</li>
		</ul>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["list"],
		"children": [
			{"role": ["listItem"], "text": "First item"},
			{"role": ["listItem"], "text": "Second item"},
			{"role": ["listItem"], "text": "Third item"}
		]
	}`), res)
}

func TestConvertLineBreaks(t *testing.T) {
	res := convertBody(t, `<p>Just<br/>testing<br/>some<br/> breaks! And useless <span>elements</span>...</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Just testing some breaks! And useless elements...",
			"ssml": "Just<break/>testing<break/>some<break/>breaks! And useless elements..."
		}
	}`), res)
}

func TestConvertHiddenContent(t *testing.T) {
	res := convertBody(t, `
		<p aria-hidden="true">Hidden <b>text!</b> <img src="with_image.jpg"/>...</p>
		<p hidden="hidden">Also hidden</p>
		<p>Visible <span aria-hidden="true">invisible</span> text</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": "Visible text"
	}`), res)
}

// Text inside <script>, <style> and ruby annotations doesn't leak into the output.
func TestConvertSkippedElements(t *testing.T) {
	res := convertBody(t, `
		<p>Before <script>var x = 1;</script>after</p>
		<style>p { color: red; }</style>
		<p><ruby>漢<rt>kan</rt><rp>(</rp></ruby>字</p>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["paragraph"], "text": "Before after"},
		{"role": ["paragraph"], "text": "漢字"}`), res)
}

// Wrappers without any semantics (<div>, <span>...) are dissolved into their parent.
func TestConvertWrapperSplicing(t *testing.T) {
	res := convertBody(t, `
		<div>
			<div>
				<p>Deep paragraph</p>
			</div>
			<p>Another one</p>
		</div>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["paragraph"], "text": "Deep paragraph"},
		{"role": ["paragraph"], "text": "Another one"}`), res)
}

// A wrapper with plain text can't be dissolved: its text is kept as an anonymous object.
func TestConvertDivWithMixedContent(t *testing.T) {
	res := convertBody(t, `<div>Intro text <p>Paragraph</p> outro text</div>`)
	assert.JSONEq(t, guidedJSON(`
		{"text": "Intro text"},
		{"role": ["paragraph"], "text": "Paragraph"},
		{"text": "outro text"}`), res)
}

func TestConvertTable(t *testing.T) {
	res := convertBody(t, `
		<table>
			<tr><th scope="col">Name</th><th scope="col">Age</th></tr>
			<tr><td>Alice</td><td>42</td></tr>
		</table>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["table"],
		"children": [
			{"role": ["row"], "children": [
				{"role": ["columnheader"], "text": "Name"},
				{"role": ["columnheader"], "text": "Age"}
			]},
			{"role": ["row"], "children": [
				{"role": ["cell"], "text": "Alice"},
				{"role": ["cell"], "text": "42"}
			]}
		]
	}`), res)
}

func TestConvertAudioVideo(t *testing.T) {
	res := convertBody(t, `
		<p>Listen: <audio src="audio.mp3"></audio> now</p>
		<video><source src="movie.mp4" type="video/mp4"/></video>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Listen: now",
			"ssml": "Listen: <readium:audio id=\"audio1\"/> now"
		},
		"children": [
			{"role": ["audio"], "id": "audio1", "audioref": "audio.mp3"}
		]
	},
	{"role": ["video"], "videoref": "movie.mp4"}`), res)
}

// Multiple values in a single epub:type attribute.
func TestConvertMultiValueEpubType(t *testing.T) {
	res := convertBody(t, `<section epub:type="bodymatter chapter"><p>Content</p></section>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["section", "chapter"],
		"children": [{"role": ["paragraph"], "text": "Content"}]
	}`), res)
}

// HTML entities in XHTML content are resolved, and non-breaking spaces normalized.
func TestConvertEntities(t *testing.T) {
	res := convertBody(t, `<p>Caf&eacute;&nbsp;&amp; lait &lt;3</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": "Café & lait <3"
	}`), res)
}

// Special characters must be escaped in SSML but not in plain text.
func TestConvertSSMLEscaping(t *testing.T) {
	res := convertBody(t, `<p>A&amp;B <em>&lt;tag&gt;</em></p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "A&B <tag>",
			"ssml": "A&amp;B <emphasis>&lt;tag&gt;</emphasis>"
		}
	}`), res)
}

// Elements with roles and an id get a textref pointing into the resource.
func TestConvertTextRefFragments(t *testing.T) {
	res := convertBody(t, `<section id="ch1" epub:type="chapter"><p id="par1">Text</p><div id="noref">More</div></section>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["section", "chapter"],
		"textref": "hello.xhtml#ch1",
		"children": [
			{"role": ["paragraph"], "textref": "hello.xhtml#par1", "text": "Text"},
			{"text": "More"}
		]
	}`), res)
}

// MathML with alternative text.
func TestConvertMath(t *testing.T) {
	res := convertBody(t, `<p>Consider <math xmlns="http://www.w3.org/1998/Math/MathML" alttext="x squared"><msup><mi>x</mi><mn>2</mn></msup></math> here.</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Consider here.",
			"ssml": "Consider <readium:math id=\"math1\"/> here."
		},
		"children": [
			{"role": ["math"], "id": "math1", "text": "x squared"}
		]
	}`), res)
}

// Inline SVG with a <title> is exposed as an image described by its title.
func TestConvertSVG(t *testing.T) {
	res := convertBody(t, `<p>Diagram: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><title>A blue square</title><rect width="10" height="10"/></svg></p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Diagram:",
			"ssml": "Diagram: <readium:image id=\"image1\"/>"
		},
		"children": [
			{"role": ["image"], "id": "image1", "description": "A blue square"}
		]
	}`), res)
}

// Definition lists and blockquotes.
func TestConvertMiscRoles(t *testing.T) {
	res := convertBody(t, `
		<dl>
			<dt>Term</dt>
			<dd>Definition</dd>
		</dl>
		<blockquote>Quoted text</blockquote>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["term"], "text": "Term"},
		{"role": ["definition"], "text": "Definition"},
		{"role": ["blockquote"], "text": "Quoted text"}`), res)
}

// The document language propagates to text objects.
func TestConvertDocumentLanguage(t *testing.T) {
	res := convertDoc(t, `<!DOCTYPE html>
		<html lang="en"><head><title>Test</title></head>
		<body><p>English text</p></body>
		</html>`, &mediatype.HTML)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {"plain": "English text", "language": "en"}
	}`), res)
}

// A block-level pagebreak with content must not unbalance the tree:
// all content stays nested in the body object.
func TestConvertBlockPagebreakWithContent(t *testing.T) {
	// XML path: content becomes the page label
	res := convertBody(t, `<div epub:type="pagebreak"><span>4</span></div><p>after</p>`)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["pagebreak"], "text": "4"},
		{"role": ["paragraph"], "text": "after"}`), res)

	// HTML path: a labelled block pagebreak's content flows transparently
	res = convertDoc(t, `<!DOCTYPE html>
		<html><head><title>Test</title></head>
		<body><div role="doc-pagebreak" title="4"><p>swallowed</p></div><p>after</p></body>
		</html>`, &mediatype.HTML)
	assert.JSONEq(t, guidedJSON(`
		{"role": ["pagebreak"], "text": "4"},
		{"role": ["paragraph"], "text": "swallowed"},
		{"role": ["paragraph"], "text": "after"}`), res)
}

// The text of a note in an inline element must survive its embedding.
func TestConvertInlineNoterefTarget(t *testing.T) {
	res := convertBody(t, `
		<p>Text with note<a href="#fn1" epub:type="noteref">1</a>.</p>
		<p>Note is <span id="fn1">right here</span> ok</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "Text with note.",
			"ssml": "Text with note<readium:noteref id=\"fn1\"/>."
		},
		"children": [
			{
				"role": ["noteref"],
				"id": "fn1",
				"text": "1",
				"children": [{"text": "right here"}]
			}
		]
	},
	{
		"role": ["paragraph"],
		"text": "Note is ok"
	}`), res)
}

// An image without src and description must not leave a dangling id in the SSML.
func TestConvertEmptyImageNoDanglingID(t *testing.T) {
	res := convertBody(t, `<p>Before <img src=""/> after</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": "Before after"
	}`), res)
}

// Words around a dropped noteref keep their separation, and punctuation binds left.
func TestConvertPlainTextJoins(t *testing.T) {
	res := convertBody(t, `
		<p>word<a href="other.xhtml#n1" epub:type="noteref">[1]</a> next</p>
		<p>resource <a href="other.xhtml#n2" epub:type="noteref">[2]</a> <em>and</em> more</p>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["paragraph"],
		"text": {
			"plain": "word next",
			"ssml": "word<readium:noteref id=\"n1\"/> next"
		},
		"children": [
			{"role": ["noteref"], "id": "n1", "text": "[1]", "children": [{"textref": "other.xhtml#n1"}]}
		]
	},
	{
		"role": ["paragraph"],
		"text": {
			"plain": "resource and more",
			"ssml": "resource <readium:noteref id=\"n2\"/> <emphasis>and</emphasis> more"
		},
		"children": [
			{"role": ["noteref"], "id": "n2", "text": "[2]", "children": [{"textref": "other.xhtml#n2"}]}
		]
	}`), res)
}

// A noteref inside hidden content is never emitted, so its target must stay in the flow.
func TestConvertHiddenNoterefKeepsTarget(t *testing.T) {
	res := convertBody(t, `
		<p aria-hidden="true">Hidden ref <a href="#note1" epub:type="noteref">1</a></p>
		<aside id="note1" epub:type="footnote">Visible note</aside>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["aside", "footnote"],
		"textref": "hello.xhtml#note1",
		"text": "Visible note"
	}`), res)
}

// A noteref pointing at its own ancestor must not suppress the enclosing content.
func TestConvertNoterefToAncestor(t *testing.T) {
	res := convertBody(t, `
		<section id="sec1"><p>Content <a href="#sec1" epub:type="noteref">[s]</a> here</p></section>`)
	assert.JSONEq(t, guidedJSON(`{
		"role": ["section"],
		"textref": "hello.xhtml#sec1",
		"children": [{
			"role": ["paragraph"],
			"text": {
				"plain": "Content here",
				"ssml": "Content <readium:noteref id=\"sec1\"/> here"
			},
			"children": [{
				"role": ["noteref"],
				"id": "sec1",
				"text": "[s]",
				"children": [{"textref": "hello.xhtml#sec1"}]
			}]
		}]
	}`), res)
}

// An empty body still yields the top-level object.
func TestConvertEmptyBody(t *testing.T) {
	res := convertBody(t, ``)
	assert.JSONEq(t, guidedJSON(``), res)
}

func TestDoErrorsWithoutBody(t *testing.T) {
	f := fetcher.NewBytesResource(manifest.Link{
		Href: manifest.MustNewHREFFromString("broken.xhtml", false),
	}, func() []byte {
		return []byte(``)
	})
	_, err := Do(context.Background(), f, manifest.Locator{
		Href: f.Link().Href.Resolve(nil, nil),
	})
	require.Error(t, err)
}
