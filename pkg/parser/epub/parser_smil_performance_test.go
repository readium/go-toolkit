package epub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSMILMixedNamespaceSelectionAndOrder(t *testing.T) {
	document, err := xmlquery.Parse(strings.NewReader(`<smil xmlns="http://www.w3.org/ns/SMIL"
        xmlns:old="http://www.w3.org/2001/SMIL20/" xmlns:epub="http://www.idpf.org/2007/ops"
        xmlns:foreign="urn:foreign">
      <old:body><old:par><old:text src="ignored-body.xhtml"/></old:par></old:body>
      <body>
        <foreign:par><text src="ignored-foreign-par.xhtml"/></foreign:par>
        <foreign:wrapper><par><text src="ignored-nested-par.xhtml"/></par></foreign:wrapper>
        <old:seq epub:textref="old-sequence.xhtml"><old:par><old:text src="old-child.xhtml"/></old:par></old:seq>
        <seq epub:textref="sequence.xhtml"/>
        <old:par><old:text src="ignored-old-text.xhtml"/><text src="old-first.xhtml"/><text src="ignored-later-text.xhtml"/>
          <old:audio src="ignored-old-audio.mp3"/><audio src="first.mp3"/><audio src="ignored-later-audio.mp3"/>
        </old:par>
        <par><foreign:text src="ignored-foreign-text.xhtml"/><foreign:wrapper><text src="ignored-nested-text.xhtml"/></foreign:wrapper>
          <text src="first.xhtml"/><foreign:audio src="ignored-foreign-audio.mp3"/>
        </par>
        <old:par><old:text src="old-second.xhtml"/><old:audio src="second.mp3"/></old:par>
        <!-- A comment and character data are not SMIL elements. -->
        par
        <par><text src="second.xhtml"/></par>
      </body>
      <body><par><text src="ignored-later-body.xhtml"/></par></body>
    </smil>`))
	require.NoError(t, err)
	result, err := ParseSMILDocument(document, url.MustURLFromString("OPS/overlay.smil"))
	require.NoError(t, err)
	require.Len(t, result.Guided, 6)

	// The pinned XPath union selected each namespace/name branch in order,
	// keeping sibling order within a branch. Preserve that existing behavior.
	var refs []string
	for _, object := range result.Guided {
		refs = append(refs, object.TextRef.String())
	}
	assert.Equal(t, []string{
		"OPS/first.xhtml", "OPS/second.xhtml", "OPS/sequence.xhtml",
		"OPS/old-first.xhtml", "OPS/old-second.xhtml", "OPS/old-sequence.xhtml",
	}, refs)
	assert.Nil(t, result.Guided[0].AudioRef)
	assert.NotNil(t, result.Guided[2].Children)
	assert.Empty(t, result.Guided[2].Children)
	assert.Equal(t, "OPS/first.mp3", result.Guided[3].AudioRef.String())
	assert.Equal(t, "OPS/second.mp3", result.Guided[4].AudioRef.String())
	require.Len(t, result.Guided[5].Children, 1)
	assert.Equal(t, "OPS/old-child.xhtml", result.Guided[5].Children[0].TextRef.String())
}

func TestSMILRequiredElements(t *testing.T) {
	for _, namespace := range []string{NamespaceSMIL, NamespaceSMIL2} {
		t.Run(namespace, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				xml  string
				want string
			}{
				{"foreign root", `<foreign:smil><body/></foreign:smil>`, "SMIL root element not found"},
				{"nested root", `<wrapper><smil><body/></smil></wrapper>`, "SMIL root element not found"},
				{"missing body", `<smil><foreign:body/><wrapper><body/></wrapper></smil>`, "SMIL body not found"},
				{"empty body", `<smil><body><foreign:par/><wrapper><par/></wrapper></body></smil>`, "failed parsing SMIL body: SMIL body is empty"},
				{"missing text", `<smil><body><par><foreign:text src="wrong.xhtml"/><wrapper><text src="nested.xhtml"/></wrapper></par></body></smil>`, "failed parsing SMIL body: failed parsing SMIL par: SMIL par has no text element"},
				{"empty first text", `<smil><body><par><text/><text src="later.xhtml"/></par></body></smil>`, "failed parsing SMIL body: failed parsing SMIL par: SMIL par text element has empty src attribute"},
				{"empty first audio", `<smil><body><par><text src="page.xhtml"/><audio/><audio src="later.mp3"/></par></body></smil>`, "failed parsing SMIL body: failed parsing SMIL par: SMIL par audio element has empty src attribute"},
				{"foreign textref", `<smil><body><seq foreign:textref="wrong.xhtml"/></body></smil>`, "failed parsing SMIL body: SMIL seq has no textref"},
				{"invalid seq textref", `<smil><body><seq epub:textref="%"/></body></smil>`, `failed parsing SMIL body: failed parsing SMIL seq textref: parse "%": invalid URL escape "%"`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					// Place namespace declarations on a document wrapper, then pass
					// that wrapper as the query root, as supported by xmlquery.
					xml := `<root xmlns="` + namespace + `" xmlns:foreign="urn:foreign" xmlns:epub="` + NamespaceOPS + `">` + tc.xml + `</root>`
					document, err := xmlquery.Parse(strings.NewReader(xml))
					require.NoError(t, err)
					root := document.FirstChild
					for root != nil && root.Type != xmlquery.ElementNode {
						root = root.NextSibling
					}
					require.NotNil(t, root)
					result, err := ParseSMILDocument(root, url.MustURLFromString("OPS/overlay.smil"))
					require.EqualError(t, err, tc.want)
					assert.Nil(t, result)
				})
			}
		})
	}
}

func TestSMILNamespacePrecedenceDoesNotSkipInvalidElement(t *testing.T) {
	for _, tc := range []struct {
		name string
		xml  string
		want string
	}{
		{"body", `<old:body><old:par><old:text src="fallback.xhtml"/></old:par></old:body><body/>`, "failed parsing SMIL body: SMIL body is empty"},
		{"text", `<body><par><old:text src="fallback.xhtml"/><text/></par></body>`, "failed parsing SMIL body: failed parsing SMIL par: SMIL par text element has empty src attribute"},
		{"audio", `<body><par><text src="page.xhtml"/><old:audio src="fallback.mp3"/><audio/></par></body>`, "failed parsing SMIL body: failed parsing SMIL par: SMIL par audio element has empty src attribute"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document, err := xmlquery.Parse(strings.NewReader(`<smil xmlns="` + NamespaceSMIL + `" xmlns:old="` + NamespaceSMIL2 + `">` + tc.xml + `</smil>`))
			require.NoError(t, err)
			result, err := ParseSMILDocument(document, url.MustURLFromString("OPS/overlay.smil"))
			require.EqualError(t, err, tc.want)
			assert.Nil(t, result)
		})
	}
}

// Keep DOM parsing outside the timed loop so sibling counts expose conversion
// cost rather than XML tokenization or archive I/O.
func BenchmarkSMILPreparedDOM(b *testing.B) {
	for _, size := range []int{32, 256, 2048} {
		b.Run(fmt.Sprintf("pars=%d", size), func(b *testing.B) {
			var xml strings.Builder
			xml.WriteString(`<smil xmlns="` + NamespaceSMIL + `"><body>`)
			for i := range size {
				fmt.Fprintf(&xml, "\n<par><text src=\"chapter.xhtml#word%d\"/><audio src=\"audio.mp3\" clipBegin=\"%ds\" clipEnd=\"%ds\"/></par>", i, i, i+1)
			}
			xml.WriteString("\n</body></smil>")
			document, err := xmlquery.Parse(strings.NewReader(xml.String()))
			if err != nil {
				b.Fatal(err)
			}
			path := url.MustURLFromString("OPS/overlay.smil")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := ParseSMILDocument(document, path)
				if err != nil {
					b.Fatal(err)
				}
				if len(result.Guided) != size {
					b.Fatalf("expected %d objects, got %d", size, len(result.Guided))
				}
			}
		})
	}
}
