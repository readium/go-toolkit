package mediatype

import (
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnifferIgnoresExtensionCase(t *testing.T) {
	assert.Equal(t, &EPUB, MediaTypeOfExtension("EPUB"), "sniffer should ignore \"EPUB\" case")
}

func TestSnifferIgnoresMediaTypeCase(t *testing.T) {
	assert.Equal(t, &EPUB, MediaTypeOfString("APPLICATION/EPUB+ZIP"), "sniffer should ignore \"APPLICATION/EPUB+ZIP\" case")
}

func TestSnifferIgnoresMediaTypeExtraParams(t *testing.T) {
	assert.Equal(t, &EPUB, MediaTypeOfString("application/epub+zip;param=value"), "sniffer should ignore extra dummy parameter when comparing mediatypes")
}

func TestSnifferFromMetadata(t *testing.T) {
	assert.Nil(t, MediaTypeOfExtension(""))
	assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOfExtension("audiobook"), "\"audiobook\" should be a Readium audiobook")
	assert.Nil(t, MediaTypeOfString(""))
	assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOfString("application/audiobook+zip"), "\"application/audiobook+zip\" should be a Readium audiobook")
	assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOfStringAndExtension("application/audiobook+zip", "audiobook"), "\"audiobook\" + \"application/audiobook+zip\" should be a Readium audiobook")
	assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOf([]string{"application/audiobook+zip"}, []string{"audiobook"}, Sniffers), "\"audiobook\" in a slice + \"application/audiobook+zip\" in a slice should be a Readium audiobook")
}

/*
TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
CBZ sniffing is implemented below as a temporary alternative.

func TestSnifferFromFile(t *testing.T) {
	testAudiobook, err := os.Open(filepath.Join("testdata", "audiobook.json"))
	assert.NoError(t, err)
	defer testAudiobook.Close()
	assert.Equal(t, &READIUM_AUDIOBOOK_MANIFEST, MediaTypeOfFileOnly(testAudiobook))
}

func TestSnifferFromBytes(t *testing.T) {
	testAudiobook, err := os.Open(filepath.Join("testdata", "audiobook.json"))
	assert.NoError(t, err)
	testAudiobookBytes, err := ioutil.ReadAll(testAudiobook)
	testAudiobook.Close()
	assert.NoError(t, err)
	assert.Equal(t, &READIUM_AUDIOBOOK_MANIFEST, MediaTypeOfBytesOnly(testAudiobookBytes))
}
*/

func TestSnifferFromFile(t *testing.T) {
	testCbz, err := os.Open(filepath.Join("testdata", "cbz.unknown"))
	assert.NoError(t, err)
	defer testCbz.Close()
	assert.Equal(t, &CBZ, MediaTypeOfFileOnly(testCbz), "test CBZ should be identified by heavy sniffer")
}

func TestSnifferFromBytes(t *testing.T) {
	testCbz, err := os.Open(filepath.Join("testdata", "cbz.unknown"))
	assert.NoError(t, err)
	testCbzBytes, err := ioutil.ReadAll(testCbz)
	testCbz.Close()
	assert.NoError(t, err)
	assert.Equal(t, &CBZ, MediaTypeOfBytesOnly(testCbzBytes), "test CBZ's bytes should be identified by heavy sniffer")
}

func TestSnifferUnknownFormat(t *testing.T) {
	assert.Nil(t, MediaTypeOfString("invalid"), "\"invalid\" mediatype should be unsniffable")
	unknownFile, err := os.Open(filepath.Join("testdata", "unknown"))
	assert.NoError(t, err)
	assert.Nil(t, MediaTypeOfFileOnly(unknownFile), "mediatype of unknown file should be unsniffable")
}

func TestSnifferValidMediaTypeFallback(t *testing.T) {
	expected, err := NewMediaTypeOfString("fruit/grapes")
	assert.NoError(t, err)
	assert.Equal(t, &expected, MediaTypeOfString("fruit/grapes"), "valid mediatype should be sniffable")
	assert.Equal(t, &expected, MediaTypeOf([]string{"invalid", "fruit/grapes"}, nil, Sniffers), "valid mediatype should be discoverable from provided list")
	assert.Equal(t, &expected, MediaTypeOf([]string{"fruit/grapes", "vegetable/brocoli"}, nil, Sniffers), "valid mediatype should be discoverable from provided list")
}

// Filetype-specific sniffing tests

func TestSniffAudiobook(t *testing.T) {
	assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOfExtension("audiobook"))
	assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOfString("application/audiobook+zip"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &READIUM_AUDIOBOOK, MediaTypeOfFileOnly("audiobook"))
}

func TestSniffAudiobookManifest(t *testing.T) {
	assert.Equal(t, &READIUM_AUDIOBOOK_MANIFEST, MediaTypeOfString("application/audiobook+json"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &READIUM_AUDIOBOOK_MANIFEST, MediaTypeOfFileOnly("audiobook.json"))
	// assert.Equal(t, &READIUM_AUDIOBOOK_MANIFEST, MediaTypeOfFileOnly("audiobook-wrongtype.json"))
}

func TestSniffAVIF(t *testing.T) {
	assert.Equal(t, &AVIF, MediaTypeOfExtension("avif"))
	assert.Equal(t, &AVIF, MediaTypeOfString("image/avif"))
}

func TestSniffBMP(t *testing.T) {
	assert.Equal(t, &BMP, MediaTypeOfExtension("bmp"))
	assert.Equal(t, &BMP, MediaTypeOfExtension("dib"))
	assert.Equal(t, &BMP, MediaTypeOfString("image/bmp"))
	assert.Equal(t, &BMP, MediaTypeOfString("image/x-bmp"))
}

func TestSniffCBZ(t *testing.T) {
	assert.Equal(t, &CBZ, MediaTypeOfExtension("cbz"))
	assert.Equal(t, &CBZ, MediaTypeOfString("application/vnd.comicbook+zip"))
	assert.Equal(t, &CBZ, MediaTypeOfString("application/x-cbz"))
	assert.Equal(t, &CBZ, MediaTypeOfString("application/x-cbr"))

	testCbz, err := os.Open(filepath.Join("testdata", "cbz.unknown"))
	assert.NoError(t, err)
	defer testCbz.Close()
	assert.Equal(t, &CBZ, MediaTypeOfFileOnly(testCbz))
}

func TestSniffDiViNa(t *testing.T) {
	assert.Equal(t, &DIVINA, MediaTypeOfExtension("divina"))
	assert.Equal(t, &DIVINA, MediaTypeOfString("application/divina+zip"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &DIVINA, MediaTypeOfFileOnly("divina-package.unknown"))
}

func TestSniffDiViNaManifest(t *testing.T) {
	assert.Equal(t, &DIVINA_MANIFEST, MediaTypeOfString("application/divina+json"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &DIVINA_MANIFEST, MediaTypeOfFileOnly("divina.json"))
}

func TestSniffEPUB(t *testing.T) {
	assert.Equal(t, &EPUB, MediaTypeOfExtension("epub"))
	assert.Equal(t, &EPUB, MediaTypeOfString("application/epub+zip"))

	testEpub, err := os.Open(filepath.Join("testdata", "epub.unknown"))
	assert.NoError(t, err)
	defer testEpub.Close()
	assert.Equal(t, &EPUB, MediaTypeOfFileOnly(testEpub))
}

func TestSniffGIF(t *testing.T) {
	assert.Equal(t, &GIF, MediaTypeOfExtension("gif"))
	assert.Equal(t, &GIF, MediaTypeOfString("image/gif"))
}

func TestSniffHTML(t *testing.T) {
	assert.Equal(t, &HTML, MediaTypeOfExtension("htm"))
	assert.Equal(t, &HTML, MediaTypeOfExtension("html"))
	assert.Equal(t, &HTML, MediaTypeOfString("text/html"))

	testHtml, err := os.Open(filepath.Join("testdata", "html.unknown"))
	assert.NoError(t, err)
	defer testHtml.Close()
	assert.Equal(t, &HTML, MediaTypeOfFileOnly(testHtml))
}

func TestSniffXHTML(t *testing.T) {
	assert.Equal(t, &XHTML, MediaTypeOfExtension("xht"))
	assert.Equal(t, &XHTML, MediaTypeOfExtension("xhtml"))
	assert.Equal(t, &XHTML, MediaTypeOfString("application/xhtml+xml"))

	testXHtml, err := os.Open(filepath.Join("testdata", "xhtml.unknown"))
	assert.NoError(t, err)
	defer testXHtml.Close()
	assert.Equal(t, &XHTML, MediaTypeOfFileOnly(testXHtml))
}

func TestSniffJPEG(t *testing.T) {
	assert.Equal(t, &JPEG, MediaTypeOfExtension("jpg"))
	assert.Equal(t, &JPEG, MediaTypeOfExtension("jpeg"))
	assert.Equal(t, &JPEG, MediaTypeOfExtension("jpe"))
	assert.Equal(t, &JPEG, MediaTypeOfExtension("jif"))
	assert.Equal(t, &JPEG, MediaTypeOfExtension("jfif"))
	assert.Equal(t, &JPEG, MediaTypeOfExtension("jfi"))
	assert.Equal(t, &JPEG, MediaTypeOfString("image/jpeg"))
}

func TestSniffJXL(t *testing.T) {
	assert.Equal(t, &JXL, MediaTypeOfExtension("jxl"))
	assert.Equal(t, &JXL, MediaTypeOfString("image/jxl"))
}

func TestSniffOPDS1Feed(t *testing.T) {
	assert.Equal(t, &OPDS1, MediaTypeOfString("application/atom+xml;profile=opds-catalog"))

	testOPDS1Feed, err := os.Open(filepath.Join("testdata", "opds1-feed.unknown"))
	assert.NoError(t, err)
	defer testOPDS1Feed.Close()
	assert.Equal(t, &OPDS1, MediaTypeOfFileOnly(testOPDS1Feed))
}

func TestSniffOPDS1Entry(t *testing.T) {
	assert.Equal(t, &OPDS1_ENTRY, MediaTypeOfString("application/atom+xml;type=entry;profile=opds-catalog"))

	testOPDS1Entry, err := os.Open(filepath.Join("testdata", "opds1-entry.unknown"))
	assert.NoError(t, err)
	defer testOPDS1Entry.Close()
	assert.Equal(t, &OPDS1_ENTRY, MediaTypeOfFileOnly(testOPDS1Entry))
}

func TestSniffOPDS2Feed(t *testing.T) {
	assert.Equal(t, &OPDS2, MediaTypeOfString("application/opds+json"))

	/*
		// TODO needs webpub heavy parsing. See func SniffOPDS in sniffer.go for details.
		testOPDS2Feed, err := os.Open(filepath.Join("testdata", "opds2-feed.json"))
		assert.NoError(t, err)
		defer testOPDS2Feed.Close()
		assert.Equal(t, &OPDS2, MediaTypeOfFileOnly(testOPDS2Feed))
	*/
}

func TestSniffOPDS2Publication(t *testing.T) {
	assert.Equal(t, &OPDS2_PUBLICATION, MediaTypeOfString("application/opds-publication+json"))

	/*
		// TODO needs webpub heavy parsing. See func SniffOPDS in sniffer.go for details.
		testOPDS2Feed, err := os.Open(filepath.Join("testdata", "opds2-publication.json"))
		assert.NoError(t, err)
		defer testOPDS2Feed.Close()
		assert.Equal(t, &OPDS2_PUBLICATION, MediaTypeOfFileOnly(testOPDS2Feed))
	*/
}

func TestSniffOPDSAuthenticationDocument(t *testing.T) {
	assert.Equal(t, &OPDS_AUTHENTICATION, MediaTypeOfString("application/opds-authentication+json"))
	assert.Equal(t, &OPDS_AUTHENTICATION, MediaTypeOfString("application/vnd.opds.authentication.v1.0+json"))

	/*
		// TODO needs webpub heavy parsing. See func SniffOPDS in sniffer.go for details.
		testOPDSAuthDoc, err := os.Open(filepath.Join("testdata", "opds-authentication.json"))
		assert.NoError(t, err)
		defer testOPDSAuthDoc.Close()
		assert.Equal(t, &OPDS_AUTHENTICATION, MediaTypeOfFileOnly(testOPDSAuthDoc))
	*/
}

func TestSniffLCPProtectedAudiobook(t *testing.T) {
	assert.Equal(t, &LCP_PROTECTED_AUDIOBOOK, MediaTypeOfExtension("lcpa"))
	assert.Equal(t, &LCP_PROTECTED_AUDIOBOOK, MediaTypeOfString("application/audiobook+lcp"))

	/*
		// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
		testLCPAudiobook, err := os.Open(filepath.Join("testdata", "audiobook-lcp.unknown"))
		assert.NoError(t, err)
		defer testLCPAudiobook.Close()
		assert.Equal(t, &LCP_PROTECTED_AUDIOBOOK, MediaTypeOfFileOnly(testLCPAudiobook))
	*/
}

func TestSniffLCPProtectedPDF(t *testing.T) {
	assert.Equal(t, &LCP_PROTECTED_PDF, MediaTypeOfExtension("lcpdf"))
	assert.Equal(t, &LCP_PROTECTED_PDF, MediaTypeOfString("application/pdf+lcp"))

	/*
		// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
		testLCPPDF, err := os.Open(filepath.Join("testdata", "pdf-lcp.unknown"))
		assert.NoError(t, err)
		defer testLCPPDF.Close()
		assert.Equal(t, &LCP_PROTECTED_PDF, MediaTypeOfFileOnly(testLCPPDF))
	*/
}

func TestSniffLCPLicenseDocument(t *testing.T) {
	assert.Equal(t, &LCP_LICENSE_DOCUMENT, MediaTypeOfExtension("lcpl"))
	assert.Equal(t, &LCP_LICENSE_DOCUMENT, MediaTypeOfString("application/vnd.readium.lcp.license.v1.0+json"))

	testLCPLicenseDoc, err := os.Open(filepath.Join("testdata", "lcpl.unknown"))
	assert.NoError(t, err)
	defer testLCPLicenseDoc.Close()
	assert.Equal(t, &LCP_LICENSE_DOCUMENT, MediaTypeOfFileOnly(testLCPLicenseDoc))
}

func TestSniffLPF(t *testing.T) {
	assert.Equal(t, &LPF, MediaTypeOfExtension("lpf"))
	assert.Equal(t, &LPF, MediaTypeOfString("application/lpf+zip"))

	testLPF1, err := os.Open(filepath.Join("testdata", "lpf.unknown"))
	assert.NoError(t, err)
	defer testLPF1.Close()
	assert.Equal(t, &LPF, MediaTypeOfFileOnly(testLPF1))

	testLPF2, err := os.Open(filepath.Join("testdata", "lpf-index-html.unknown"))
	assert.NoError(t, err)
	defer testLPF2.Close()
	assert.Equal(t, &LPF, MediaTypeOfFileOnly(testLPF2))
}

func TestSniffPDF(t *testing.T) {
	assert.Equal(t, &PDF, MediaTypeOfExtension("pdf"))
	assert.Equal(t, &PDF, MediaTypeOfString("application/pdf"))

	testPDF, err := os.Open(filepath.Join("testdata", "pdf.unknown"))
	assert.NoError(t, err)
	defer testPDF.Close()
	assert.Equal(t, &PDF, MediaTypeOfFileOnly(testPDF))
}

func TestSniffPNG(t *testing.T) {
	assert.Equal(t, &PNG, MediaTypeOfExtension("png"))
	assert.Equal(t, &PNG, MediaTypeOfString("image/png"))
}

func TestSniffTIFF(t *testing.T) {
	assert.Equal(t, &TIFF, MediaTypeOfExtension("tiff"))
	assert.Equal(t, &TIFF, MediaTypeOfExtension("tif"))
	assert.Equal(t, &TIFF, MediaTypeOfString("image/tiff"))
	assert.Equal(t, &TIFF, MediaTypeOfString("image/tiff-fx"))
}

func TestSniffWEBP(t *testing.T) {
	assert.Equal(t, &WEBP, MediaTypeOfExtension("webp"))
	assert.Equal(t, &WEBP, MediaTypeOfString("image/webp"))
}

func TestSniffWebPub(t *testing.T) {
	assert.Equal(t, &READIUM_WEBPUB, MediaTypeOfExtension("webpub"))
	assert.Equal(t, &READIUM_WEBPUB, MediaTypeOfString("application/webpub+zip"))

	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &READIUM_WEBPUB, MediaTypeOfFileOnly("webpub-package.unknown"))
}

func TestSniffWebPubManifest(t *testing.T) {
	assert.Equal(t, &READIUM_WEBPUB_MANIFEST, MediaTypeOfString("application/webpub+json"))

	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &READIUM_WEBPUB_MANIFEST, MediaTypeOfFileOnly("webpub.json"))
}

func TestSniffW3CWPUBManifest(t *testing.T) {
	testW3CWPUB, err := os.Open(filepath.Join("testdata", "w3c-wpub.json"))
	assert.NoError(t, err)
	defer testW3CWPUB.Close()
	assert.Equal(t, &W3C_WPUB_MANIFEST, MediaTypeOfFileOnly(testW3CWPUB))
}

func TestSniffZAB(t *testing.T) {
	assert.Equal(t, &ZAB, MediaTypeOfExtension("zab"))

	testZAB, err := os.Open(filepath.Join("testdata", "zab.unknown"))
	assert.NoError(t, err)
	defer testZAB.Close()
	assert.Equal(t, &ZAB, MediaTypeOfFileOnly(testZAB))
}

func TestSniffJSON(t *testing.T) {
	assert.Equal(t, &JSON, MediaTypeOfString("application/json"))
	assert.Equal(t, &JSON, MediaTypeOfString("application/json; charset=utf-8"))

	testJSON, err := os.Open(filepath.Join("testdata", "any.json"))
	assert.NoError(t, err)
	defer testJSON.Close()
	assert.Equal(t, &JSON, MediaTypeOfFileOnly(testJSON))
}

func TestSniffSystemMediaTypes(t *testing.T) {
	err := mime.AddExtensionType(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	assert.NoError(t, err)
	xlsx, err := NewMediaType("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "XLSX", "xlsx")
	assert.NoError(t, err)
	assert.Equal(t, &xlsx, MediaTypeOf([]string{}, []string{"foobar", "xlsx"}, Sniffers))
	assert.Equal(t, &xlsx, MediaTypeOf([]string{"applicaton/foobar", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, []string{}, Sniffers))
}

/*
// TODO needs URLConnection.guessContentTypeFromStream(it) equivalent
// https://github.com/readium/r2-shared-kotlin/blob/develop/r2-shared/src/main/java/org/readium/r2/shared/util/mediatype/Sniffer.kt#L381
func TestSniffSystemMediaTypesFromBytes(t *testing.T) {
	err := mime.AddExtensionType("png", "image/png")
	assert.NoError(t, err)
	png, err := NewMediaType("image/png", "PNG", "png")
	assert.NoError(t, err)

	testPNG, err := os.Open(filepath.Join("testdata", "png.unknown"))
	assert.NoError(t, err)
	defer testPNG.Close()
	assert.Equal(t, png, MediaTypeOfFileOnly(testPNG))
}
*/
