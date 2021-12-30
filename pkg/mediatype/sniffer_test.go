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

/*
TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
CBZ sniffing is implemented below as a temporary alternative.

func TestSnifferFromFile(t *testing.T) {
	testAudiobook, err := os.Open(filepath.Join("testdata", "audiobook.json"))
	assert.NoError(t, err)
	defer testAudiobook.Close()
	assert.Equal(t, &ReadiumAudiobookManifest, OfFileOnly(testAudiobook))
}

func TestSnifferFromBytes(t *testing.T) {
	testAudiobook, err := os.Open(filepath.Join("testdata", "audiobook.json"))
	assert.NoError(t, err)
	testAudiobookBytes, err := ioutil.ReadAll(testAudiobook)
	testAudiobook.Close()
	assert.NoError(t, err)
	assert.Equal(t, &ReadiumAudiobookManifest, MediaTypeOfBytesOnly(testAudiobookBytes))
}
*/

func TestSnifferFromFile(t *testing.T) {
	testCbz, err := os.Open(filepath.Join("testdata", "cbz.unknown"))
	assert.NoError(t, err)
	defer testCbz.Close()
	assert.Equal(t, &CBZ, OfFileOnly(testCbz), "test CBZ should be identified by heavy Sniffer")
}

func TestSnifferFromBytes(t *testing.T) {
	testCbz, err := os.Open(filepath.Join("testdata", "cbz.unknown"))
	assert.NoError(t, err)
	testCbzBytes, err := ioutil.ReadAll(testCbz)
	testCbz.Close()
	assert.NoError(t, err)
	assert.Equal(t, &CBZ, OfBytesOnly(testCbzBytes), "test CBZ's bytes should be identified by heavy Sniffer")
}

func TestSnifferUnknownFormat(t *testing.T) {
	assert.Nil(t, OfString("invalid"), "\"invalid\" MediaType should be unsniffable")
	unknownFile, err := os.Open(filepath.Join("testdata", "unknown"))
	assert.NoError(t, err)
	assert.Nil(t, OfFileOnly(unknownFile), "MediaType of unknown file should be unsniffable")
}

func TestSnifferValidMediaTypeFallback(t *testing.T) {
	expected, err := NewOfString("fruit/grapes")
	assert.NoError(t, err)
	assert.Equal(t, &expected, OfString("fruit/grapes"), "valid MediaType should be sniffable")
	assert.Equal(t, &expected, Of([]string{"invalid", "fruit/grapes"}, nil, Sniffers), "valid MediaType should be discoverable from provided list")
	assert.Equal(t, &expected, Of([]string{"fruit/grapes", "vegetable/brocoli"}, nil, Sniffers), "valid MediaType should be discoverable from provided list")
}

// Filetype-specific sniffing tests

func TestSniffAudiobook(t *testing.T) {
	assert.Equal(t, &ReadiumAudiobook, OfExtension("audiobook"))
	assert.Equal(t, &ReadiumAudiobook, OfString("application/audiobook+zip"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &ReadiumAudiobook, OfFileOnly("audiobook"))
}

func TestSniffAudiobookManifest(t *testing.T) {
	assert.Equal(t, &ReadiumAudiobookManifest, OfString("application/audiobook+json"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &ReadiumAudiobookManifest, OfFileOnly("audiobook.json"))
	// assert.Equal(t, &ReadiumAudiobookManifest, OfFileOnly("audiobook-wrongtype.json"))
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
	assert.NoError(t, err)
	defer testCbz.Close()
	assert.Equal(t, &CBZ, OfFileOnly(testCbz))
}

func TestSniffDiViNa(t *testing.T) {
	assert.Equal(t, &Divina, OfExtension("divina"))
	assert.Equal(t, &Divina, OfString("application/divina+zip"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &DIVINA, OfFileOnly("divina-package.unknown"))
}

func TestSniffDiViNaManifest(t *testing.T) {
	assert.Equal(t, &DivinaManifest, OfString("application/divina+json"))
	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &DIVINA_MANIFEST, OfFileOnly("divina.json"))
}

func TestSniffEPUB(t *testing.T) {
	assert.Equal(t, &EPUB, OfExtension("epub"))
	assert.Equal(t, &EPUB, OfString("application/epub+zip"))

	testEpub, err := os.Open(filepath.Join("testdata", "epub.unknown"))
	assert.NoError(t, err)
	defer testEpub.Close()
	assert.Equal(t, &EPUB, OfFileOnly(testEpub))
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
	assert.NoError(t, err)
	defer testHtml.Close()
	assert.Equal(t, &HTML, OfFileOnly(testHtml))
}

func TestSniffXHTML(t *testing.T) {
	assert.Equal(t, &XHTML, OfExtension("xht"))
	assert.Equal(t, &XHTML, OfExtension("xhtml"))
	assert.Equal(t, &XHTML, OfString("application/xhtml+xml"))

	testXHtml, err := os.Open(filepath.Join("testdata", "xhtml.unknown"))
	assert.NoError(t, err)
	defer testXHtml.Close()
	assert.Equal(t, &XHTML, OfFileOnly(testXHtml))
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
	assert.NoError(t, err)
	defer testOPDS1Feed.Close()
	assert.Equal(t, &OPDS1, OfFileOnly(testOPDS1Feed))
}

func TestSniffOPDS1Entry(t *testing.T) {
	assert.Equal(t, &OPDS1Entry, OfString("application/atom+xml;type=entry;profile=opds-catalog"))

	testOPDS1Entry, err := os.Open(filepath.Join("testdata", "opds1-entry.unknown"))
	assert.NoError(t, err)
	defer testOPDS1Entry.Close()
	assert.Equal(t, &OPDS1Entry, OfFileOnly(testOPDS1Entry))
}

func TestSniffOPDS2Feed(t *testing.T) {
	assert.Equal(t, &OPDS2, OfString("application/opds+json"))

	/*
		// TODO needs webpub heavy parsing. See func SniffOPDS in sniffer.go for details.
		testOPDS2Feed, err := os.Open(filepath.Join("testdata", "opds2-feed.json"))
		assert.NoError(t, err)
		defer testOPDS2Feed.Close()
		assert.Equal(t, &OPDS2, OfFileOnly(testOPDS2Feed))
	*/
}

func TestSniffOPDS2Publication(t *testing.T) {
	assert.Equal(t, &OPDS2Publication, OfString("application/opds-publication+json"))

	/*
		// TODO needs webpub heavy parsing. See func SniffOPDS in sniffer.go for details.
		testOPDS2Feed, err := os.Open(filepath.Join("testdata", "opds2-publication.json"))
		assert.NoError(t, err)
		defer testOPDS2Feed.Close()
		assert.Equal(t, &OPDS2_PUBLICATION, OfFileOnly(testOPDS2Feed))
	*/
}

func TestSniffOPDSAuthenticationDocument(t *testing.T) {
	assert.Equal(t, &OPDSAuthentication, OfString("application/opds-authentication+json"))
	assert.Equal(t, &OPDSAuthentication, OfString("application/vnd.opds.authentication.v1.0+json"))

	/*
		// TODO needs webpub heavy parsing. See func SniffOPDS in sniffer.go for details.
		testOPDSAuthDoc, err := os.Open(filepath.Join("testdata", "opds-authentication.json"))
		assert.NoError(t, err)
		defer testOPDSAuthDoc.Close()
		assert.Equal(t, &OPDS_AUTHENTICATION, OfFileOnly(testOPDSAuthDoc))
	*/
}

func TestSniffLCPProtectedAudiobook(t *testing.T) {
	assert.Equal(t, &LCPProtectedAudiobook, OfExtension("lcpa"))
	assert.Equal(t, &LCPProtectedAudiobook, OfString("application/audiobook+lcp"))

	/*
		// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
		testLCPAudiobook, err := os.Open(filepath.Join("testdata", "audiobook-lcp.unknown"))
		assert.NoError(t, err)
		defer testLCPAudiobook.Close()
		assert.Equal(t, &LCP_PROTECTED_AUDIOBOOK, OfFileOnly(testLCPAudiobook))
	*/
}

func TestSniffLCPProtectedPDF(t *testing.T) {
	assert.Equal(t, &LCPProtectedPDF, OfExtension("lcpdf"))
	assert.Equal(t, &LCPProtectedPDF, OfString("application/pdf+lcp"))

	/*
		// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
		testLCPPDF, err := os.Open(filepath.Join("testdata", "pdf-lcp.unknown"))
		assert.NoError(t, err)
		defer testLCPPDF.Close()
		assert.Equal(t, &LCP_PROTECTED_PDF, OfFileOnly(testLCPPDF))
	*/
}

func TestSniffLCPLicenseDocument(t *testing.T) {
	assert.Equal(t, &LCPLicenseDocument, OfExtension("lcpl"))
	assert.Equal(t, &LCPLicenseDocument, OfString("application/vnd.readium.lcp.license.v1.0+json"))

	testLCPLicenseDoc, err := os.Open(filepath.Join("testdata", "lcpl.unknown"))
	assert.NoError(t, err)
	defer testLCPLicenseDoc.Close()
	assert.Equal(t, &LCPLicenseDocument, OfFileOnly(testLCPLicenseDoc))
}

func TestSniffLPF(t *testing.T) {
	assert.Equal(t, &LPF, OfExtension("lpf"))
	assert.Equal(t, &LPF, OfString("application/lpf+zip"))

	testLPF1, err := os.Open(filepath.Join("testdata", "lpf.unknown"))
	assert.NoError(t, err)
	defer testLPF1.Close()
	assert.Equal(t, &LPF, OfFileOnly(testLPF1))

	testLPF2, err := os.Open(filepath.Join("testdata", "lpf-index-html.unknown"))
	assert.NoError(t, err)
	defer testLPF2.Close()
	assert.Equal(t, &LPF, OfFileOnly(testLPF2))
}

func TestSniffPDF(t *testing.T) {
	assert.Equal(t, &PDF, OfExtension("pdf"))
	assert.Equal(t, &PDF, OfString("application/pdf"))

	testPDF, err := os.Open(filepath.Join("testdata", "pdf.unknown"))
	assert.NoError(t, err)
	defer testPDF.Close()
	assert.Equal(t, &PDF, OfFileOnly(testPDF))
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

	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &ReadiumWebpub, OfFileOnly("webpub-package.unknown"))
}

func TestSniffWebPubManifest(t *testing.T) {
	assert.Equal(t, &ReadiumWebpubManifest, OfString("application/webpub+json"))

	// TODO needs webpub heavy parsing. See func SniffWebpub in sniffer.go for details.
	// assert.Equal(t, &ReadiumWebpubManifest, OfFileOnly("webpub.json"))
}

func TestSniffW3CWPUBManifest(t *testing.T) {
	testW3CWPUB, err := os.Open(filepath.Join("testdata", "w3c-wpub.json"))
	assert.NoError(t, err)
	defer testW3CWPUB.Close()
	assert.Equal(t, &W3CWPUBManifest, OfFileOnly(testW3CWPUB))
}

func TestSniffZAB(t *testing.T) {
	assert.Equal(t, &ZAB, OfExtension("zab"))

	testZAB, err := os.Open(filepath.Join("testdata", "zab.unknown"))
	assert.NoError(t, err)
	defer testZAB.Close()
	assert.Equal(t, &ZAB, OfFileOnly(testZAB))
}

func TestSniffJSON(t *testing.T) {
	assert.Equal(t, &JSON, OfString("application/json"))
	assert.Equal(t, &JSON, OfString("application/json; charset=utf-8"))

	testJSON, err := os.Open(filepath.Join("testdata", "any.json"))
	assert.NoError(t, err)
	defer testJSON.Close()
	assert.Equal(t, &JSON, OfFileOnly(testJSON))
}

func TestSniffSystemMediaTypes(t *testing.T) {
	err := mime.AddExtensionType(".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	png, err := NewMediaType("image/png", "PNG", "png")
	assert.NoError(t, err)

	testPNG, err := os.Open(filepath.Join("testdata", "png.unknown"))
	assert.NoError(t, err)
	defer testPNG.Close()
	assert.Equal(t, png, OfFileOnly(testPNG))
}
*/
