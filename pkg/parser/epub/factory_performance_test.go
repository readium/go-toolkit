package epub

import (
	"fmt"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkPublicationFactory isolates OPF-to-manifest conversion from ZIP I/O
// and XML parsing. Increasing item counts expose accidental quadratic work in
// the split between reading order and resources.
func BenchmarkPublicationFactory(b *testing.B) {
	for _, size := range []int{32, 256, 2048} {
		for _, encrypted := range []bool{false, true} {
			b.Run(fmt.Sprintf("items=%d/encrypted=%t", size, encrypted), func(b *testing.B) {
				factory := benchmarkPublicationFactory(size, encrypted)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					publication := factory.Create()
					if len(publication.ReadingOrder) != size*3/4 || len(publication.Resources) != size-size*3/4 {
						b.Fatal("incorrect reading order or resource count")
					}
				}
			})
		}
	}
}

func benchmarkPublicationFactory(size int, encrypted bool) PublicationFactory {
	factory := PublicationFactory{
		FallbackTitle: "Factory benchmark",
		PackageDocument: PackageDocument{
			EPUBVersion:       3,
			EPUBVersionString: "3.0",
			Manifest:          make([]Item, size),
			Spine:             Spine{itemrefs: make([]ItemRef, size*3/4)},
		},
	}
	if encrypted {
		factory.EncryptionData = make(map[string]manifest.Encryption)
	}
	for i := range size {
		id := fmt.Sprintf("item-%04d", i)
		href := url.MustURLFromString(fmt.Sprintf("OPS/chapter-%04d.xhtml", i))
		factory.PackageDocument.Manifest[i] = Item{ID: id, Href: href, MediaType: &mediatype.XHTML}
		if i < len(factory.PackageDocument.Spine.itemrefs) {
			factory.PackageDocument.Spine.itemrefs[i] = ItemRef{idref: id, linear: true}
		}
		if encrypted && i%16 == 0 {
			factory.EncryptionData[href.String()] = manifest.Encryption{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc"}
		}
	}
	return factory
}

func TestPublicationFactoryReadingOrderMembership(t *testing.T) {
	item := func(id, href string) Item {
		return Item{ID: id, Href: url.MustURLFromString(href)}
	}
	factory := PublicationFactory{
		PackageDocument: PackageDocument{
			Manifest: []Item{
				item("chapter", "superseded.xhtml"),
				item("nonlinear", "notes.xhtml"),
				item("chapter", "chapter.xhtml"),
				item("resource", "image.png"),
			},
			Spine: Spine{itemrefs: []ItemRef{
				{idref: "chapter", linear: true},
				{idref: "missing", linear: true},
				{idref: "nonlinear", linear: false},
				{idref: "chapter", linear: true},
			}},
		},
	}
	publication := factory.Create()
	require.Len(t, publication.ReadingOrder, 2)
	assert.Equal(t, "chapter.xhtml", publication.ReadingOrder[0].Href.String())
	assert.Equal(t, "chapter.xhtml", publication.ReadingOrder[1].Href.String())
	require.Len(t, publication.Resources, 2)
	assert.Equal(t, "notes.xhtml", publication.Resources[0].Href.String())
	assert.Equal(t, "image.png", publication.Resources[1].Href.String())
}

func TestPublicationFactoryCombinesLinkProperties(t *testing.T) {
	encryption := manifest.Encryption{Algorithm: "test-encryption", OriginalLength: 1024}
	factory := PublicationFactory{
		PackageDocument: PackageDocument{
			metadata: EPUBMetadata{global: map[string][]MetadataItem{
				VocabularyMeta + "cover": {{value: "chapter"}},
			}},
			Manifest: []Item{{
				ID:   "chapter",
				Href: url.MustURLFromString("OPS/text/../chapter.xhtml"),
				Properties: []string{
					VocabularyItem + "cover-image",
					VocabularyItem + "scripted",
					VocabularyItem + "svg",
					"https://example.org/custom-property",
				},
			}},
			Spine: Spine{itemrefs: []ItemRef{{
				idref: "chapter", linear: true,
				properties: []string{VocabularyItemref + "page-spread-left", "unknown", VocabularyRendition + "page-spread-right"},
			}}},
		},
		EncryptionData: map[string]manifest.Encryption{"OPS/chapter.xhtml": encryption},
	}
	publication := factory.Create()
	require.Len(t, publication.ReadingOrder, 1)
	link := publication.ReadingOrder[0]
	assert.Equal(t, manifest.Strings{"cover"}, link.Rels)
	assert.Equal(t, manifest.Properties{
		"contains":                            []string{"js", "svg"},
		"https://example.org/custom-property": true,
		"page":                                "right",
		"encrypted":                           encryption.ToMap(),
	}, link.Properties)
	// Encryption matching normalizes the lookup, not the published href.
	assert.Equal(t, "OPS/text/../chapter.xhtml", link.Href.String())
}

func TestPublicationFactoryNoLinkProperties(t *testing.T) {
	for _, encryption := range []map[string]manifest.Encryption{nil, {}} {
		factory := benchmarkPublicationFactory(32, false)
		factory.EncryptionData = encryption
		publication := factory.Create()
		for _, links := range []manifest.LinkList{publication.ReadingOrder, publication.Resources} {
			for _, link := range links {
				assert.Nil(t, link.Properties)
			}
		}
	}
}

func TestPublicationFactoryRepeatedMetadata(t *testing.T) {
	for _, fixture := range []string{"titles-epub3", "title-multiple-subtitles", "collections-epub3", "collections-epub2"} {
		t.Run(fixture, func(t *testing.T) {
			document, resourceErr := fetcher.ReadResourceAsXML(t.Context(), fetcher.NewFileResource(
				manifest.Link{}, "testdata/package/"+fixture+".opf"))
			require.Nil(t, resourceErr)
			packageDocument, err := ParsePackageDocument(document, url.MustURLFromString("OPS/content.opf"))
			require.NoError(t, err)
			factory := PublicationFactory{PackageDocument: *packageDocument, FallbackTitle: "Fallback title"}
			first := factory.Create()
			assert.Equal(t, first, factory.Create())
			assert.Equal(t, first, factory.Create())
		})
	}
}
