package epub

import (
	"bytes"
	"encoding/xml"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

// parsePackageDocumentDOM preserves the complete previous Parser.Parse OPF path,
// including the distinction between XML resource errors and OPF model errors.
func parsePackageDocumentDOM(data []byte, path url.URL) (*PackageDocument, error) {
	document, err := xmlquery.ParseWithOptions(bytes.NewReader(data), xmlquery.ParserOptions{
		Decoder: &xmlquery.DecoderOptions{Strict: true, Entity: xml.HTMLEntity},
	})
	if err != nil {
		return nil, fetcher.Other(err)
	}
	model, err := ParsePackageDocument(document, path)
	if err != nil {
		return nil, errors.Wrap(err, "invalid OPF file")
	}
	return model, nil
}

func assertPackageDocumentDataMatchesDOM(t *testing.T, data []byte) {
	t.Helper()
	path := url.MustURLFromString("OPS/package.opf")
	want, wantErr := parsePackageDocumentDOM(data, path)
	got, gotErr := parsePackageDocumentData(data, path)
	if wantErr == nil {
		require.NoError(t, gotErr)
		assert.Equal(t, want, got)
		return
	}
	require.Error(t, gotErr)
	assert.IsType(t, wantErr, gotErr)
	assert.EqualError(t, gotErr, wantErr.Error())
	assert.Equal(t, want, got)
	if wantResourceErr, ok := wantErr.(*fetcher.ResourceError); ok {
		gotResourceErr, ok := gotErr.(*fetcher.ResourceError)
		require.True(t, ok)
		assert.Equal(t, wantResourceErr.Code, gotResourceErr.Code)
		assert.IsType(t, wantResourceErr.Cause, gotResourceErr.Cause)
		assert.EqualError(t, gotResourceErr.Cause, wantResourceErr.Cause.Error())
	}
}

func TestPackageDocumentDataMatchesFixtures(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/package/*.opf")
	require.NoError(t, err)
	require.NotEmpty(t, fixtures)
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			require.NoError(t, err)
			assertPackageDocumentDataMatchesDOM(t, data)
		})
	}
}

const streamTestPackageStart = `<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="3.0" unique-identifier="book-id" xml:lang="en">`
const streamTestMetadata = `<metadata><dc:identifier id="book-id">urn:test:book</dc:identifier><dc:title id="title">A book</dc:title><dc:language>en</dc:language></metadata>`
const streamTestManifest = `<manifest><item id="chapter" href="text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="style" href="styles/book.css" media-type="text/css"/></manifest>`
const streamTestSpine = `<spine page-progression-direction="rtl"><itemref idref="chapter"/></spine>`

func streamTestPackage(metadata, manifest, spine string) string {
	return streamTestPackageStart + metadata + manifest + spine + `</package>`
}

func streamTestPrefixedPackage(data string) string {
	return strings.NewReplacer(
		`xmlns="http://www.idpf.org/2007/opf"`, `xmlns:opf="http://www.idpf.org/2007/opf"`,
		"<package", "<opf:package", "</package", "</opf:package",
		"<metadata", "<opf:metadata", "</metadata", "</opf:metadata",
		"<manifest", "<opf:manifest", "</manifest", "</opf:manifest",
		"<spine", "<opf:spine", "</spine", "</opf:spine",
		"<itemref", "<opf:itemref", "</itemref", "</opf:itemref",
		"<item", "<opf:item", "</item", "</opf:item",
	).Replace(data)
}

func TestPackageDocumentDataMatchesDOM(t *testing.T) {
	standard := streamTestPackage(streamTestMetadata, streamTestManifest, streamTestSpine)
	for _, test := range []struct{ name, source string }{
		{"standard", standard},
		{"prefixed", streamTestPrefixedPackage(standard)},
		{"UTF8BOM", "\ufeff" + standard},
		{"XMLDeclaration", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + standard},
		{"EPUB2", strings.Replace(standard, `version="3.0"`, `version="2.0"`, 1)},
		{"defaultVersion", strings.Replace(standard, ` version="3.0"`, "", 1)},
		{"invalidVersion", strings.Replace(standard, `version="3.0"`, `version="invalid"`, 1)},
		{"invalidVersionBeforeItemProperties", strings.Replace(strings.Replace(standard, `version="3.0"`, `version="invalid"`, 1), `id="chapter"`, `id="chapter" properties=":"`, 1)},
		{"missingMetadataBeforeItemrefProperties", streamTestPackage("", streamTestManifest, `<spine><itemref idref="chapter" properties="nav &#x3000;: svg"/></spine>`)},
		{"trailingXMLErrorBeforeProperties", strings.Replace(standard, `id="chapter"`, `id="chapter" properties=":"`, 1) + `<broken`},
		{"rootLanguage", strings.Replace(standard, `xml:lang="en"`, `xml:lang="日本語"`, 1)},
		{"metadataLanguage", streamTestPackage(`<metadata><dc:title xml:lang="fr">Français</dc:title><dc:language>日本語</dc:language></metadata>`, streamTestManifest, streamTestSpine)},
		{"languageInLaterMetadata", streamTestPackage(`<metadata><dc:title>A book</dc:title></metadata><metadata><dc:language>fr</dc:language></metadata>`, streamTestManifest, streamTestSpine)},
		{"namespacedRootAttributes", strings.Replace(standard, `version="3.0"`, `xmlns:f="urn:foreign" f:version="2.0" version="3.0"`, 1)},
		{"sameNamespacePrefixAfterDefault", strings.Replace(standard, `version="3.0"`, `xmlns:v="http://www.idpf.org/2007/opf" v:version="2.0" version="3.0"`, 1)},
		{"sameNamespacePrefixBeforeDefault", strings.Replace(standard, `<package xmlns=`, `<package xmlns:v="http://www.idpf.org/2007/opf" v:version="2.0" xmlns=`, 1)},
		{"nestedMetadataMarkup", streamTestPackage(`<metadata xmlns:h="http://www.w3.org/1999/xhtml"><dc:title>A <h:span>multilingual <h:em>日本語</h:em></h:span> book</dc:title></metadata>`, streamTestManifest, streamTestSpine)},
		{"CDATA", streamTestPackage(`<metadata><dc:title><![CDATA[Title </metadata> <manifest> & 日本語]]></dc:title></metadata>`, streamTestManifest, streamTestSpine)},
		{"comments", `<!-- before package -->` + streamTestPackage(`<!-- before metadata -->`+strings.Replace(streamTestMetadata, "A book", "A <!-- </metadata> -->book", 1)+`<!-- after metadata -->`, streamTestManifest, streamTestSpine) + `<!-- after package -->`},
		{"processingInstructions", `<?reader before?>` + streamTestPackage(`<?reader before-metadata?>`+strings.Replace(streamTestMetadata, "A book", "A <?reader inside?>book", 1), `<?reader between?>`+streamTestManifest, streamTestSpine) + `<?reader after?>`},
		{"HTMLEntities", strings.Replace(standard, "A book", `A&nbsp;book &copy; &#x65E5; &#160;`, 1)},
		{"rootPrefixDeclarations", strings.Replace(standard, `version="3.0"`, `prefix="custom: https://example.org/vocab schema: https://schema.org/" version="3.0"`, 1)},
		{"metadataRefinements", streamTestPackage(`<metadata><dc:title id="main">Main</dc:title><meta property="alternate-script" refines="#main" xml:lang="fr">Principal</meta><meta property="file-as" refines="#main">Main, The</meta><meta property="media:duration" refines="#chapter">12.5</meta></metadata>`, streamTestManifest, streamTestSpine)},
		{"metadataLinks", streamTestPackage(`<metadata><dc:title>Book</dc:title><link href="record.xml" rel="record" media-type="application/xml" properties="onix"/></metadata>`, streamTestManifest, streamTestSpine)},
		{"multipleMetadata", streamTestPackage(streamTestMetadata+`<metadata><dc:title>Second title</dc:title></metadata>`, streamTestManifest, streamTestSpine)},
		{"nestedMetadata", streamTestPackage(`<extension>`+streamTestMetadata+`</extension>`, streamTestManifest, streamTestSpine)},
		{"nestedAndDirectMetadata", streamTestPackage(`<extension><metadata><dc:title>First nested</dc:title></metadata></extension>`+streamTestMetadata, streamTestManifest, streamTestSpine)},
		{"foreignMetadata", streamTestPackage(`<f:metadata xmlns:f="urn:foreign"><dc:title>Foreign</dc:title></f:metadata>`+streamTestMetadata, streamTestManifest, streamTestSpine)},
		{"metadataNamespaceRedeclaration", streamTestPackage(`<metadata xmlns:dc="urn:foreign"><dc:title>Foreign DC namespace</dc:title></metadata>`, streamTestManifest, streamTestSpine)},
		{"metadataPrefixRebound", streamTestPackage(`<metadata xmlns:p="urn:outer"><dc:title xmlns:p="urn:inner" p:lang="fr">Français</dc:title><dc:creator p:role="aut">Author</dc:creator></metadata>`, streamTestManifest, streamTestSpine)},
		{"namespacePrefixSurvivesMetadata", streamTestPackage(strings.Replace(streamTestMetadata, `<metadata>`, `<metadata xmlns:opf="http://www.idpf.org/2007/opf">`, 1), `<manifest xmlns:p="http://www.idpf.org/2007/opf"><p:item id="chapter" p:href="namespaced.xhtml" href="chapter.xhtml"/></manifest>`, streamTestSpine)},
		{"emptyMetadata", streamTestPackage(`<metadata/>`, streamTestManifest, streamTestSpine)},
		{"emptyManifestAndSpine", streamTestPackage(streamTestMetadata, `<manifest/>`, `<spine/>`)},
		{"duplicateManifest", streamTestPackage(streamTestMetadata, streamTestManifest+`<manifest><item id="other" href="other.xhtml"/></manifest>`, streamTestSpine)},
		{"duplicateSpine", streamTestPackage(streamTestMetadata, streamTestManifest, streamTestSpine+`<spine><itemref idref="style"/></spine>`)},
		{"sectionsReordered", streamTestPackage(streamTestSpine, streamTestManifest, streamTestMetadata)},
		{"foreignManifestItems", streamTestPackage(streamTestMetadata, `<manifest xmlns:f="urn:foreign"><f:item id="foreign" href="foreign.xhtml"/><item id="chapter" href="chapter.xhtml"/><wrapper><item id="nested" href="nested.xhtml"/></wrapper></manifest>`, streamTestSpine)},
		{"foreignSpineItems", streamTestPackage(streamTestMetadata, streamTestManifest, `<spine xmlns:f="urn:foreign"><f:itemref idref="style"/><wrapper><itemref idref="style"/></wrapper><itemref idref="chapter"/></spine>`)},
		{"nestedItemMarkup", streamTestPackage(streamTestMetadata, `<manifest><item id="chapter" href="chapter.xhtml"><item id="nested" href="nested.xhtml"/><!-- item comment --></item></manifest>`, streamTestSpine)},
		{"missingItemHref", streamTestPackage(streamTestMetadata, `<manifest><item id="missing"/><item id="chapter" href="chapter.xhtml"/></manifest>`, streamTestSpine)},
		{"missingItemrefID", streamTestPackage(streamTestMetadata, streamTestManifest, `<spine><itemref/><itemref idref="chapter"/></spine>`)},
		{"itemPropertiesAndURIs", streamTestPackage(streamTestMetadata, `<manifest><item id="chapter" href="../my chapter.xhtml#p1" media-type="application/xhtml+xml" properties="nav scripted svg" fallback="style" media-overlay="overlay"/><item id="remote" href="https://example.org/a?x=1&amp;y=2"/></manifest>`, `<spine toc="ncx"><itemref idref="chapter" linear="no" properties="rendition:page-spread-center"/></spine>`)},
		{"duplicateRootAttributes", strings.Replace(standard, `version="3.0"`, `version="3.0" version="2.0"`, 1)},
		{"duplicateItemAttributes", strings.Replace(standard, `id="chapter"`, `id="chapter" id="later"`, 1)},
		{"duplicateSpineAttributes", strings.Replace(standard, `page-progression-direction="rtl"`, `page-progression-direction="rtl" page-progression-direction="ltr"`, 1)},
		{"prefixedItemAttributes", strings.Replace(standard, `id="chapter"`, `xmlns:f="urn:foreign" f:id="foreign" id="chapter"`, 1)},
		{"undeclaredAttributeNamespace", strings.Replace(standard, `id="chapter"`, `undeclared:id="foreign" id="chapter"`, 1)},
		{"undeclaredElementNamespace", streamTestPackage(streamTestMetadata, `<manifest><undeclared:item id="foreign" href="foreign.xhtml"/><item id="chapter" href="chapter.xhtml"/></manifest>`, streamTestSpine)},
		{"undeclaredRootNamespace", strings.ReplaceAll(streamTestPrefixedPackage(standard), ` xmlns:opf="http://www.idpf.org/2007/opf"`, "")},
		{"missingOPFNamespace", strings.Replace(standard, ` xmlns="http://www.idpf.org/2007/opf"`, "", 1)},
		{"rootWrapped", `<wrapper>` + standard + `</wrapper>`},
		{"missingMetadata", streamTestPackage("", streamTestManifest, streamTestSpine)},
		{"missingManifest", streamTestPackage(streamTestMetadata, "", streamTestSpine)},
		{"missingSpine", streamTestPackage(streamTestMetadata, streamTestManifest, "")},
		{"emptyInput", ""},
		{"truncatedRoot", strings.TrimSuffix(standard, `</package>`)},
		{"mismatchedClosingTag", strings.Replace(standard, `</manifest>`, `</metadata>`, 1)},
		{"unclosedAttribute", strings.Replace(standard, `href="text/chapter.xhtml"`, `href="text/chapter.xhtml`, 1)},
		{"unknownEntity", strings.Replace(standard, "A book", "A &undefined; book", 1)},
		{"invalidUTF8", strings.Replace(standard, "A book", "A \xff book", 1)},
		{"trailingText", standard + "trailing text"},
		{"trailingUnclosedTag", standard + `<broken`},
		{"trailingUnclosedComment", standard + `<!--broken`},
		{"trailingUnclosedPI", standard + `<?broken`},
		{"multipleRoots", standard + `<another/>`},
		{"multiplePackages", standard + strings.Replace(standard, "A book", "Another book", 1)},
		{"leadingForeignRoot", `<another/>` + standard},
		{"doctype", `<!DOCTYPE package SYSTEM "about:legacy-compat">` + standard},
		{"doctypeInternalSubset", `<!DOCTYPE package [<!ELEMENT package ANY>]>` + standard},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertPackageDocumentDataMatchesDOM(t, []byte(test.source))
		})
	}
}

func TestPackageDocumentDataEncodedXMLMatchesDOM(t *testing.T) {
	standard := streamTestPackage(streamTestMetadata, streamTestManifest, streamTestSpine)
	latin1 := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?>` + strings.Replace(standard, "A book", "Un caf\xe9", 1))
	utf16 := []byte(`<?xml version="1.0" encoding="UTF-16"?>` + strings.Replace(standard, "A book", "日本語", 1))
	littleEndian, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes(utf16)
	require.NoError(t, err)
	bigEndian, err := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder().Bytes(utf16)
	require.NoError(t, err)
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"ISO-8859-1", latin1},
		{"UTF-16LE", littleEndian},
		{"UTF-16BE", bigEndian},
		{"unknownEncoding", []byte(`<?xml version="1.0" encoding="unknown-charset"?>` + standard)},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertPackageDocumentDataMatchesDOM(t, test.data)
		})
	}
}

func TestPackageDocumentStreamStandardFastPath(t *testing.T) {
	standard := streamTestPackage(streamTestMetadata, streamTestManifest, streamTestSpine)
	for _, test := range []struct{ name, source string }{
		{"defaultNamespace", standard},
		{"prefixedNamespace", streamTestPrefixedPackage(standard)},
		{"EPUB2", strings.Replace(standard, `version="3.0"`, `version="2.0"`, 1)},
		{"emptySections", streamTestPackage(`<metadata/>`, `<manifest/>`, `<spine/>`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := url.MustURLFromString("OPS/package.opf")
			want, err := parsePackageDocumentDOM([]byte(test.source), path)
			require.NoError(t, err)
			got, fast := parsePackageDocumentStream([]byte(test.source), path)
			require.True(t, fast, "ordinary OPF should use the streaming path")
			assert.Equal(t, want, got)
		})
	}
}

func FuzzPackageDocumentDataMatchesDOM(f *testing.F) {
	standard := streamTestPackage(streamTestMetadata, streamTestManifest, streamTestSpine)
	for _, seed := range []string{
		standard,
		streamTestPrefixedPackage(standard),
		streamTestPackage(`<metadata><dc:title><![CDATA[A <book> & 日本語]]><span>Nested title</span></dc:title></metadata>`, streamTestManifest, streamTestSpine),
		streamTestPackage(`<metadata xmlns:opf="http://www.idpf.org/2007/opf"><dc:title xmlns="urn:foreign">A book</dc:title></metadata>`, streamTestManifest, streamTestSpine),
		strings.Replace(standard, `<package xmlns=`, `<package xmlns:v="http://www.idpf.org/2007/opf" v:version="2.0" xmlns=`, 1),
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 128<<10 {
			t.Skip("limit the work per fuzz input")
		}
		// Bound nesting and element count before entering the reference DOM
		// parser, whose recursive traversals were not designed as fuzz targets.
		decoder := xml.NewDecoder(bytes.NewReader(data))
		decoder.Entity = xml.HTMLEntity
		depth, elements := 0, 0
		for {
			token, err := decoder.Token()
			if err != nil {
				break
			}
			switch token.(type) {
			case xml.StartElement:
				depth++
				elements++
				if depth > 128 || elements > 4096 {
					t.Skip("limit DOM depth and size")
				}
			case xml.EndElement:
				depth--
			}
		}

		path := url.MustURLFromString("OPS/package.opf")
		var want *PackageDocument
		var wantErr error
		baselineReturned := false
		func() {
			// Existing DOM parser panics are outside this optimization's scope.
			// Keep recovery restricted to the baseline so new parser panics fail.
			defer func() { _ = recover() }()
			want, wantErr = parsePackageDocumentDOM(data, path)
			baselineReturned = true
		}()
		if !baselineReturned {
			t.Skip("reference parser panics on this input")
		}

		got, gotErr := parsePackageDocumentData(data, path)
		if wantErr != nil {
			require.Error(t, gotErr)
			assert.IsType(t, wantErr, gotErr)
			assert.EqualError(t, gotErr, wantErr.Error())
			assert.Equal(t, want, got)
			if wantResourceErr, ok := wantErr.(*fetcher.ResourceError); ok {
				gotResourceErr, ok := gotErr.(*fetcher.ResourceError)
				require.True(t, ok)
				assert.Equal(t, wantResourceErr.Code, gotResourceErr.Code)
				assert.IsType(t, wantResourceErr.Cause, gotResourceErr.Cause)
				assert.EqualError(t, gotResourceErr.Cause, wantResourceErr.Cause.Error())
			}
			return
		}
		require.NoError(t, gotErr)
		require.NotNil(t, got)
		// strconv.ParseFloat accepts NaN as a version in the reference parser;
		// matching NaNs are equivalent even though DeepEqual treats them unequal.
		if math.IsNaN(want.EPUBVersion) && math.IsNaN(got.EPUBVersion) {
			want.EPUBVersion, got.EPUBVersion = 0, 0
		}
		assert.Equal(t, want, got)
	})
}
