package epub

import (
	"bytes"
	"encoding/xml"
	"path/filepath"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
)

// BenchmarkEPUBStages separates archive opening, OPF XML decoding, OPF model
// construction, navigation loading/parsing, DRM detection, and manifest creation.
// Except ArchiveOpen, stages reuse an open archive; OPFModel and ManifestFactory
// also reuse prepared input structures. These diagnose CPU/allocation costs, not
// cold I/O latency. Use BenchmarkParser for complete fresh-archive measurements.
// OPFDecode compares full XML-to-model decoding with the retained DOM reference.
func BenchmarkEPUBStages(b *testing.B) {
	files, dir := epubBenchmarkFiles(b)
	for _, path := range files {
		name, err := filepath.Rel(dir, path)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.ToSlash(name), func(b *testing.B) {
			ctx := b.Context()
			f, err := fetcher.NewArchiveFetcherFromPath(ctx, path)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(f.Close)
			opfPath, err := GetRootFilePath(ctx, f)
			if err != nil {
				b.Fatal(err)
			}
			res := f.Get(ctx, manifest.Link{Href: manifest.NewHREF(opfPath)})
			raw, rerr := res.Read(ctx, 0, 0)
			res.Close()
			if rerr != nil {
				b.Fatal(rerr)
			}
			decode := func() (*xmlquery.Node, error) {
				return xmlquery.ParseWithOptions(bytes.NewReader(raw), xmlquery.ParserOptions{
					Decoder: &xmlquery.DecoderOptions{Strict: true, Entity: xml.HTMLEntity},
				})
			}
			doc, err := decode()
			if err != nil {
				b.Fatal(err)
			}
			pkg, err := ParsePackageDocument(doc, opfPath)
			if err != nil {
				b.Fatal(err)
			}
			scheme, encDoc, err := protection.IdentifyEPUBProtection(ctx, f)
			if err != nil {
				b.Fatal(err)
			}
			encryption, err := parseEncryptionData(ctx, f, scheme.URI(), encDoc)
			if err != nil {
				b.Fatal(err)
			}
			factory := PublicationFactory{
				FallbackTitle: filepath.Base(path), PackageDocument: *pkg,
				NavigationData: parseNavigationData(ctx, *pkg, f),
				EncryptionData: encryption, DisplayOptions: parseDisplayOptions(ctx, f),
			}
			b.Run("ArchiveOpen", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					archive, err := fetcher.NewArchiveFetcherFromPath(ctx, path)
					if err != nil {
						b.Fatal(err)
					}
					archive.Close()
				}
			})
			b.Run("OPFXML", func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(raw)))
				for b.Loop() {
					if _, err := decode(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("OPFModel", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := ParsePackageDocument(doc, opfPath); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("OPFDecode", func(b *testing.B) {
				for _, implementation := range []string{"DOM", "Streaming"} {
					b.Run(implementation, func(b *testing.B) {
						_, streamed := parsePackageDocumentStream(raw, opfPath)
						b.ReportAllocs()
						b.SetBytes(int64(len(raw)))
						for b.Loop() {
							var parsed *PackageDocument
							var err error
							if implementation == "DOM" {
								var node *xmlquery.Node
								node, err = decode()
								if err == nil {
									parsed, err = ParsePackageDocument(node, opfPath)
								}
							} else {
								parsed, err = parsePackageDocumentData(raw, opfPath)
							}
							if err != nil {
								b.Fatal(err)
							}
							if parsed == nil || len(parsed.Manifest) != len(pkg.Manifest) || len(parsed.Spine.itemrefs) != len(pkg.Spine.itemrefs) {
								b.Fatal("OPF decoder returned different item counts")
							}
						}
						if implementation == "Streaming" && streamed {
							b.ReportMetric(1, "streamed/op")
						}
					})
				}
			})
			b.Run("Navigation", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					parseNavigationData(ctx, *pkg, f)
				}
			})
			b.Run("Protection", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					scheme, doc, err := protection.IdentifyEPUBProtection(ctx, f)
					if err != nil {
						b.Fatal(err)
					}
					if _, err := parseEncryptionData(ctx, f, scheme.URI(), doc); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("ManifestFactory", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					factory.Create()
				}
			})
		})
	}
}
