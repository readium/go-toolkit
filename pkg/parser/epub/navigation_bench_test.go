package epub

import (
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// BenchmarkNavigationDOM measures extraction from an already parsed DOM so the
// cost of navigation traversal is visible independently of XML decoding and I/O.
func BenchmarkNavigationDOM(b *testing.B) {
	for _, format := range []struct {
		name, directory, extension string
		parse                      func(*xmlquery.Node, url.URL) map[string]manifest.LinkList
	}{
		{"NCX", "ncx/ncx-", ".ncx", ParseNCX},
		{"NavDoc", "navdoc/nav-", ".xhtml", ParseNavDoc},
	} {
		for _, fixture := range []string{"complex", "children", "titles"} {
			b.Run(format.name+"/"+fixture, func(b *testing.B) {
				document, err := fetcher.ReadResourceAsXML(b.Context(), fetcher.NewFileResource(
					manifest.Link{}, "testdata/"+format.directory+fixture+format.extension))
				if err != nil {
					b.Fatal(err)
				}
				base := url.MustURLFromString("OPS/navigation" + format.extension)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if len(format.parse(document, base)["toc"]) == 0 {
						b.Fatal("fixture produced no navigation")
					}
				}
			})
		}
	}
}
