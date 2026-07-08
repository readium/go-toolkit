package mediatype

import (
	"archive/zip"
	"bytes"
	"io"
	"mime"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnifferIgnoresExtensionCase(t *testing.T) {
	assert.Equal(t, &EPUB, OfExtension("EPUB"), "Sniffer should ignore \"EPUB\" case")
}

func TestSnifferIgnoresMediaTypeCase(t *testing.T) {
	assert.Equal(t, &EPUB, OfString("APPLICATION/EPUB+ZIP"), "Sniffer should ignore \"APPLICATION/EPUB+ZIP\" case")
}

func TestSnifferIgnoresMediaTypeExtraParams(t *testing.T) {
	assert.Equal(t, &EPUB, OfString("application/epub+zip;param=value"), "Sniffer should ignore extra dummy parameter when comparing MediaTypes")
}

func TestSnifferFromMetadata(t *testing.T) {
	assert.Nil(t, OfExtension(""))
	assert.Equal(t, &ReadiumAudiobook, OfExtension("audiobook"), "\"audiobook\" should be a Readium audiobook")
	assert.Nil(t, OfString(""))
	assert.Equal(t, &ReadiumAudiobook, OfString("application/audiobook+zip"), "\"application/audiobook+zip\" should be a Readium audiobook")
	assert.Equal(t, &ReadiumAudiobook, OfStringAndExtension("application/audiobook+zip", "audiobook"), "\"audiobook\" + \"application/audiobook+zip\" should be a Readium audiobook")
	assert.Equal(t, &ReadiumAudiobook, Of([]string{"application/audiobook+zip"}, []string{"audiobook"}, Sniffers), "\"audiobook\" in a slice + \"application/audiobook+zip\" in a slice should be a Readium audiobook")
}

// Opens a file in testdata and sniffs its media type from the content alone.
func sniffFixture(t *testing.T, filename string) *MediaType {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", filename))
	require.NoError(t, err)
	defer f.Close()
	return OfFileOnly(t.Context(), f)
}

func TestSnifferFromFile(t *testing.T) {
	assert.Equal(t, &ReadiumAudiobookManifest, sniffFixture(t, "audiobook.json"))
}

func TestSnifferFromBytes(t *testing.T) {
	testAudiobook, err := os.Open(filepath.Join("testdata", "audiobook.json"))
	require.NoError(t, err)
	testAudiobookBytes, err := io.ReadAll(testAudiobook)
	testAudiobook.Close()
	require.NoError(t, err)
	assert.Equal(t, &ReadiumAudiobookManifest, OfBytesOnly(t.Context(), testAudiobookBytes))
}

// JSON which manifest.ManifestFromJSON would reject must not sniff as a RWPM.
func TestSnifferRejectsInvalidRWPM(t *testing.T) {
	// No metadata title.
	assert.Nil(t, OfBytesOnly(t.Context(), []byte(`{"metadata":{"whatever":1},"readingOrder":[{"href":"a.mp3","type":"audio/mpeg"}]}`)))
	// A reading order link without href.
	assert.Nil(t, OfBytesOnly(t.Context(), []byte(`{"metadata":{"title":"T"},"readingOrder":[{"type":"audio/mpeg"}]}`)))
	// A reading order link with an unparsable type.
	assert.Nil(t, OfBytesOnly(t.Context(), []byte(`{"metadata":{"title":"T"},"readingOrder":[{"href":"a.mp3","type":"!!!"}]}`)))
	// A localized title with a non-string translation, which ManifestFromJSON rejects.
	assert.Nil(t, OfBytesOnly(t.Context(), []byte(`{"metadata":{"title":{"en":5}},"readingOrder":[{"href":"a.mp3","type":"audio/mpeg"}]}`)))
}

// The OPDS 2 heavy sniffing only classifies JSON that decodes as a RWPM, so arbitrary
// JSON carrying an OPDS-vocabulary link is not misread as an OPDS 2 document.
func TestSnifferOPDS2RequiresManifestShape(t *testing.T) {
	// An acquisition link but no metadata: not an OPDS 2 publication.
	assert.Nil(t, OfBytesOnly(t.Context(), []byte(`{"links":[{"rel":"http://opds-spec.org/acquisition/buy","href":"/buy"}]}`)))
	// A self link to an OPDS feed but no metadata: not an OPDS 2 feed.
	assert.Nil(t, OfBytesOnly(t.Context(), []byte(`{"links":[{"rel":"self","href":"/feed","type":"application/opds+json"}]}`)))
}

// The archive opened during heavy sniffing is memoized (opened once, not per sniffer
// or per entry read) and released when the sniffing context is closed.
func TestSnifferContextArchiveLifecycle(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "webpub-package.unknown"))
	require.NoError(t, err)
	defer f.Close()

	sc := &SnifferContext{content: NewSnifferFileContent(f)}

	a1, err := sc.ContentAsArchive(t.Context())
	require.NoError(t, err)
	require.NotNil(t, a1)

	// A second call, and archive-entry reads, reuse the same handle instead of reopening.
	a2, err := sc.ContentAsArchive(t.Context())
	require.NoError(t, err)
	assert.True(t, a1 == a2, "archive should be memoized, not reopened")
	assert.True(t, sc.ContainsArchiveEntryAt(t.Context(), "manifest.json"))

	// Closing releases the handle, and accessing the archive afterwards fails
	// gracefully rather than handing back a closed handle or panicking.
	sc.Close()
	assert.Nil(t, sc._contentAsArchive)
	assert.False(t, sc.ContainsArchiveEntryAt(t.Context(), "manifest.json"))
	assert.Nil(t, sc.ReadArchiveEntryAt(t.Context(), "manifest.json"))
	sc.Close() // Idempotent.
}

// Opening non-archive content as an archive memoizes the error, so the archive-entry
// helpers fail gracefully (no nil-handle panic) even when called repeatedly.
func TestSnifferContextArchiveErrorMemoized(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "audiobook.json")) // Not an archive.
	require.NoError(t, err)
	defer f.Close()

	sc := &SnifferContext{content: NewSnifferFileContent(f)}
	_, err = sc.ContentAsArchive(t.Context())
	require.Error(t, err)
	assert.False(t, sc.ContainsArchiveEntryAt(t.Context(), "manifest.json"))
	assert.Nil(t, sc.ReadArchiveEntryAt(t.Context(), "manifest.json"))
	sc.Close() // Must not panic when nothing was opened.
}

// Content sniffing must not consume a non-seekable file: a sniffer running after one
// which read the whole stream (e.g. as JSON) still sees the content.
func TestSnifferNonSeekableFile(t *testing.T) {
	pdf, err := os.ReadFile(filepath.Join("testdata", "pdf.unknown"))
	require.NoError(t, err)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("doc") // No extension, so only the content identifies it
	require.NoError(t, err)
	_, err = w.Write(pdf)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	f, err := zr.Open("doc")
	require.NoError(t, err)
	defer f.Close()
	assert.Equal(t, &PDF, OfFileOnly(t.Context(), f))
}

func TestSnifferUnknownFormat(t *testing.T) {
	assert.Nil(t, OfString("invalid"), "\"invalid\" MediaType should be unsniffable")
	unknownFile, err := os.Open(filepath.Join("testdata", "unknown"))
	require.NoError(t, err)
	assert.Nil(t, OfFileOnly(t.Context(), unknownFile), "MediaType of unknown file should be unsniffable")
}

func TestSnifferValidMediaTypeFallback(t *testing.T) {
	expected, err := NewOfString("fruit/grapes")
	require.NoError(t, err)
	assert.Equal(t, &expected, OfString("fruit/grapes"), "valid MediaType should be sniffable")
	assert.Equal(t, &expected, Of([]string{"invalid", "fruit/grapes"}, nil, Sniffers), "valid MediaType should be discoverable from provided list")
	assert.Equal(t, &expected, Of([]string{"fruit/grapes", "vegetable/brocoli"}, nil, Sniffers), "valid MediaType should be discoverable from provided list")
}

// Filetype-specific sniffing tests

func TestSniffAudiobook(t *testing.T) {
	assert.Equal(t, &ReadiumAudiobook, OfExtension("audiobook"))
	assert.Equal(t, &ReadiumAudiobook, OfString("application/audiobook+zip"))
	assert.Equal(t, &ReadiumAudiobook, sniffFixture(t, "audiobook-package.unknown"))
}

func TestSniffAudiobookManifest(t *testing.T) {
	assert.Equal(t, &ReadiumAudiobookManifest, OfString("application/audiobook+json"))
	assert.Equal(t, &ReadiumAudiobookManifest, sniffFixture(t, "audiobook.json"))
	assert.Equal(t, &ReadiumAudiobookManifest, sniffFixture(t, "audiobook-wrongtype.json"))
}

func TestSniffAVIF(t *testing.T) {
	assert.Equal(t, &AVIF, OfExtension("avif"))
	assert.Equal(t, &AVIF, OfString("image/avif"))
}

func TestSniffBMP(t *testing.T) {
	assert.Equal(t, &BMP, OfExtension("bmp"))
	assert.Equal(t, &BMP, OfExtension("dib"))
	assert.Equal(t, &BMP, OfString("image/bmp"))
	assert.Equal(t, &BMP, OfString("image/x-bmp"))
}

func TestSniffCBZ(t *testing.T) {
	assert.Equal(t, &CBZ, OfExtension("cbz"))
	assert.Equal(t, &CBZ, OfString("application/vnd.comicbook+zip"))
	assert.Equal(t, &CBZ, OfString("application/x-cbz"))
	assert.Equal(t, &CBZ, OfString("application/x-cbr"))

	testCbz, err := os.Open(filepath.Join("testdata", "cbz.unknown"))
	require.NoError(t, err)
	defer testCbz.Close()
	assert.Equal(t, &CBZ, OfFileOnly(t.Context(), testCbz))
}

func TestSniffDiViNa(t *testing.T) {
	assert.Equal(t, &ReadiumDivina, OfExtension("divina"))
	assert.Equal(t, &ReadiumDivina, OfString("application/divina+zip"))
	assert.Equal(t, &ReadiumDivina, sniffFixture(t, "divina-package.unknown"))
}

func TestSniffDiViNaManifest(t *testing.T) {
	assert.Equal(t, &ReadiumDivinaManifest, OfString("application/divina+json"))
	assert.Equal(t, &ReadiumDivinaManifest, sniffFixture(t, "divina.json"))
}

func TestSniffEPUB(t *testing.T) {
	assert.Equal(t, &EPUB, OfExtension("epub"))
	assert.Equal(t, &EPUB, OfString("application/epub+zip"))

	testEpub, err := os.Open(filepath.Join("testdata", "epub.unknown"))
	require.NoError(t, err)
	defer testEpub.Close()
	assert.Equal(t, &EPUB, OfFileOnly(t.Context(), testEpub))
}

func TestSniffGIF(t *testing.T) {
	assert.Equal(t, &GIF, OfExtension("gif"))
	assert.Equal(t, &GIF, OfString("image/gif"))
}

func TestSniffHTML(t *testing.T) {
	assert.Equal(t, &HTML, OfExtension("htm"))
	assert.Equal(t, &HTML, OfExtension("html"))
	assert.Equal(t, &HTML, OfString("text/html"))

	testHtml, err := os.Open(filepath.Join("testdata", "html.unknown"))
	require.NoError(t, err)
	defer testHtml.Close()
	assert.Equal(t, &HTML, OfFileOnly(t.Context(), testHtml))
}

func TestSniffXHTML(t *testing.T) {
	assert.Equal(t, &XHTML, OfExtension("xht"))
	assert.Equal(t, &XHTML, OfExtension("xhtml"))
	assert.Equal(t, &XHTML, OfString("application/xhtml+xml"))

	testXHtml, err := os.Open(filepath.Join("testdata", "xhtml.unknown"))
	require.NoError(t, err)
	defer testXHtml.Close()
	assert.Equal(t, &XHTML, OfFileOnly(t.Context(), testXHtml))
}

func TestSniffJPEG(t *testing.T) {
	assert.Equal(t, &JPEG, OfExtension("jpg"))
	assert.Equal(t, &JPEG, OfExtension("jpeg"))
	assert.Equal(t, &JPEG, OfExtension("jpe"))
	assert.Equal(t, &JPEG, OfExtension("jif"))
	assert.Equal(t, &JPEG, OfExtension("jfif"))
	assert.Equal(t, &JPEG, OfExtension("jfi"))
	assert.Equal(t, &JPEG, OfString("image/jpeg"))
}

func TestSniffJXL(t *testing.T) {
	assert.Equal(t, &JXL, OfExtension("jxl"))
	assert.Equal(t, &JXL, OfString("image/jxl"))
}

func TestSniffOPDS1Feed(t *testing.T) {
	assert.Equal(t, &OPDS1, OfString("application/atom+xml;profile=opds-catalog"))

	testOPDS1Feed, err := os.Open(filepath.Join("testdata", "opds1-feed.unknown"))
	require.NoError(t, err)
	defer testOPDS1Feed.Close()
	assert.Equal(t, &OPDS1, OfFileOnly(t.Context(), testOPDS1Feed))
}

func TestSniffOPDS1Entry(t *testing.T) {
	assert.Equal(t, &OPDS1Entry, OfString("application/atom+xml;type=entry;profile=opds-catalog"))

	testOPDS1Entry, err := os.Open(filepath.Join("testdata", "opds1-entry.unknown"))
	require.NoError(t, err)
	defer testOPDS1Entry.Close()
	assert.Equal(t, &OPDS1Entry, OfFileOnly(t.Context(), testOPDS1Entry))
}

func TestSniffOPDS2Feed(t *testing.T) {
	assert.Equal(t, &OPDS2, OfString("application/opds+json"))
	assert.Equal(t, &OPDS2, sniffFixture(t, "opds2-feed.json"))
}

func TestSniffOPDS2Publication(t *testing.T) {
	assert.Equal(t, &OPDS2Publication, OfString("application/opds-publication+json"))
	assert.Equal(t, &OPDS2Publication, sniffFixture(t, "opds2-publication.json"))
}

// An OPDS 2 publication with a reading order is also a valid RWPM: the acquisition
// links must take precedence over the Readium manifest heuristics.
func TestSniffOPDS2PublicationWithReadingOrder(t *testing.T) {
	pub := []byte(`{
		"metadata": {"title": "An audiobook for sale"},
		"links": [
			{"rel": "self", "href": "http://example.org/pub", "type": "application/opds-publication+json"},
			{"rel": "http://opds-spec.org/acquisition/buy", "href": "/buy", "type": "application/audiobook+zip"}
		],
		"readingOrder": [{"href": "chapter1.mp3", "type": "audio/mpeg"}]
	}`)
	assert.Equal(t, &OPDS2Publication, OfBytesOnly(t.Context(), pub))
}

func TestSniffOPDSAuthenticationDocument(t *testing.T) {
	assert.Equal(t, &OPDSAuthentication, OfString("application/opds-authentication+json"))
	assert.Equal(t, &OPDSAuthentication, OfString("application/vnd.opds.authentication.v1.0+json"))
	assert.Equal(t, &OPDSAuthentication, sniffFixture(t, "opds-authentication.json"))
}

func TestSniffLCPProtectedAudiobook(t *testing.T) {
	assert.Equal(t, &LCPProtectedAudiobook, OfExtension("lcpa"))
	assert.Equal(t, &LCPProtectedAudiobook, OfString("application/audiobook+lcp"))
	assert.Equal(t, &LCPProtectedAudiobook, sniffFixture(t, "audiobook-lcp.unknown"))
}

func TestSniffLCPProtectedPDF(t *testing.T) {
	assert.Equal(t, &LCPProtectedPDF, OfExtension("lcpdf"))
	assert.Equal(t, &LCPProtectedPDF, OfString("application/pdf+lcp"))
	assert.Equal(t, &LCPProtectedPDF, sniffFixture(t, "pdf-lcp.unknown"))
}

func TestSniffLCPLicenseDocument(t *testing.T) {
	assert.Equal(t, &LCPLicenseDocument, OfExtension("lcpl"))
	assert.Equal(t, &LCPLicenseDocument, OfString("application/vnd.readium.lcp.license.v1.0+json"))

	testLCPLicenseDoc, err := os.Open(filepath.Join("testdata", "lcpl.unknown"))
	require.NoError(t, err)
	defer testLCPLicenseDoc.Close()
	assert.Equal(t, &LCPLicenseDocument, OfFileOnly(t.Context(), testLCPLicenseDoc))
}

func TestSniffLPF(t *testing.T) {
	assert.Equal(t, &LPF, OfExtension("lpf"))
	assert.Equal(t, &LPF, OfString("application/lpf+zip"))

	testLPF1, err := os.Open(filepath.Join("testdata", "lpf.unknown"))
	require.NoError(t, err)
	defer testLPF1.Close()
	assert.Equal(t, &LPF, OfFileOnly(t.Context(), testLPF1))

	testLPF2, err := os.Open(filepath.Join("testdata", "lpf-index-html.unknown"))
	require.NoError(t, err)
	defer testLPF2.Close()
	assert.Equal(t, &LPF, OfFileOnly(t.Context(), testLPF2))
}

func TestSniffPDF(t *testing.T) {
	assert.Equal(t, &PDF, OfExtension("pdf"))
	assert.Equal(t, &PDF, OfString("application/pdf"))

	testPDF, err := os.Open(filepath.Join("testdata", "pdf.unknown"))
	require.NoError(t, err)
	defer testPDF.Close()
	assert.Equal(t, &PDF, OfFileOnly(t.Context(), testPDF))
}

func TestSniffPNG(t *testing.T) {
	assert.Equal(t, &PNG, OfExtension("png"))
	assert.Equal(t, &PNG, OfString("image/png"))
}

func TestSniffTIFF(t *testing.T) {
	assert.Equal(t, &TIFF, OfExtension("tiff"))
	assert.Equal(t, &TIFF, OfExtension("tif"))
	assert.Equal(t, &TIFF, OfString("image/tiff"))
	assert.Equal(t, &TIFF, OfString("image/tiff-fx"))
}

func TestSniffWEBP(t *testing.T) {
	assert.Equal(t, &WEBP, OfExtension("webp"))
	assert.Equal(t, &WEBP, OfString("image/webp"))
}

func TestSniffWebPub(t *testing.T) {
	assert.Equal(t, &ReadiumWebpub, OfExtension("webpub"))
	assert.Equal(t, &ReadiumWebpub, OfString("application/webpub+zip"))
	assert.Equal(t, &ReadiumWebpub, sniffFixture(t, "webpub-package.unknown"))
}

func TestSniffWebPubManifest(t *testing.T) {
	assert.Equal(t, &ReadiumWebpubManifest, OfString("application/webpub+json"))
	assert.Equal(t, &ReadiumWebpubManifest, sniffFixture(t, "webpub.json"))
}

func TestSniffW3CWPUBManifest(t *testing.T) {
	testW3CWPUB, err := os.Open(filepath.Join("testdata", "w3c-wpub.json"))
	require.NoError(t, err)
	defer testW3CWPUB.Close()
	assert.Equal(t, &W3CWPUBManifest, OfFileOnly(t.Context(), testW3CWPUB))
}

func TestSniffZAB(t *testing.T) {
	assert.Equal(t, &ZAB, OfExtension("zab"))

	testZAB, err := os.Open(filepath.Join("testdata", "zab.unknown"))
	require.NoError(t, err)
	defer testZAB.Close()
	assert.Equal(t, &ZAB, OfFileOnly(t.Context(), testZAB))
}

func TestSniffJSON(t *testing.T) {
	assert.Equal(t, &JSON, OfString("application/json"))
	assert.Equal(t, &JSON, OfString("application/json; charset=utf-8"))

	testJSON, err := os.Open(filepath.Join("testdata", "any.json"))
	require.NoError(t, err)
	defer testJSON.Close()
	assert.Equal(t, &JSON, OfFileOnly(t.Context(), testJSON))
}

func TestSniffSystemMediaTypes(t *testing.T) {
	err := mime.AddExtensionType(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	require.NoError(t, err)
	xlsx, err := New("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "XLSX", "xlsx")
	assert.NoError(t, err)
	assert.Equal(t, &xlsx, Of([]string{}, []string{"foobar", "xlsx"}, Sniffers))
	assert.Equal(t, &xlsx, Of([]string{"applicaton/foobar", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, []string{}, Sniffers))
}

/*
// TODO needs URLConnection.guessContentTypeFromStream(it) equivalent
// https://github.com/readium/r2-shared-kotlin/blob/develop/r2-shared/src/main/java/org/readium/r2/shared/util/mediatype/Sniffer.kt#L381
func TestSniffSystemMediaTypesFromBytes(t *testing.T) {
	err := mime.AddExtensionType("png", "image/png")
	require.NoError(t, err)
	png, err := NewMediaType("image/png", "PNG", "png")
	require.NoError(t, err)

	testPNG, err := os.Open(filepath.Join("testdata", "png.unknown"))
	require.NoError(t, err)
	defer testPNG.Close()
	assert.Equal(t, png, OfFileOnly(testPNG))
}
*/
