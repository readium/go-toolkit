package mediatype

import (
	"encoding/json"
	"mime"
	"path/filepath"
	"strings"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

type Sniffer func(context SnifferContext) *MediaType

// Sniffs an XHTML document.
// Must precede the HTML sniffer.
func SniffXHTML(context SnifferContext) *MediaType {
	if context.HasFileExtension("xht", "xhtml") || context.HasMediaType("application/xhtml+xml") {
		return &XHTML
	}

	if cxml := context.ContentAsXML(); cxml != nil {
		if strings.ToLower(cxml.XMLName.Local) == "html" && strings.Contains(strings.ToLower(cxml.XMLName.Space), "xhtml") {
			return &XHTML
		}
	}

	return nil
}

// Sniffs an HTML document.
func SniffHTML(context SnifferContext) *MediaType {
	if context.HasFileExtension("htm", "html") || context.HasMediaType("text/html") {
		return &HTML
	}

	// [contentAsXml] will fail if the HTML is not a proper XML document, hence the doctype check after this.
	if cxml := context.ContentAsXML(); cxml != nil {
		if strings.ToLower(cxml.XMLName.Local) == "html" {
			return &HTML
		}
	}

	// Check if begins with "<!DOCTYPE html>"
	s, _ := context.ContentAsString()
	ts := strings.TrimSpace(s) // Trim space
	if len(ts) < 15 {          // If less than 15 chars, no use comparing
		return nil
	}
	// Compare the lowercased first 15 characters with the target
	if strings.ToLower(ts[:15]) == "<!doctype html>" {
		return &HTML
	}

	return nil
}

// Sniffs an OPDS document.
func SniffOPDS(context SnifferContext) *MediaType {
	// OPDS 1 (Light)
	if context.HasMediaType("application/atom+xml;type=entry;profile=opds-catalog") {
		return &OPDS1Entry
	}
	if context.HasMediaType("application/atom+xml;profile=opds-catalog") {
		return &OPDS1
	}

	// OPDS 2 (Light)
	if context.HasMediaType("application/opds+json") {
		return &OPDS2
	}
	if context.HasMediaType("application/opds-publication+json") {
		return &OPDS2Publication
	}

	// OPDS Authentication Document (Light)
	if context.HasMediaType("application/opds-authentication+json") || context.HasMediaType("application/vnd.opds.authentication.v1.0+json") {
		return &OPDSAuthentication
	}

	// OPDS 1 (Heavy)
	if cxml := context.ContentAsXML(); cxml != nil {
		if cxml.XMLName.Space == "http://www.w3.org/2005/Atom" {
			if cxml.XMLName.Local == "feed" {
				return &OPDS1
			} else if cxml.XMLName.Local == "entry" {
				return &OPDS1Entry
			}
		}
	}

	// OPDS 2 (Heavy)
	// TODO requires context.ContentAsRWPM()

	// OPDS Authentication Document (Heavy)
	if context.ContainsJSONKeys("id", "title", "authentication") {
		return &OPDSAuthentication
	}

	return nil
}

// Sniffs an LCP License Document.
func SniffLCPLicense(context SnifferContext) *MediaType {
	if context.HasFileExtension("lcpl") || context.HasMediaType("application/vnd.readium.lcp.license.v1.0+json") {
		return &LCPLicenseDocument
	}
	if context.ContainsJSONKeys("id", "issued", "provider", "encryption") {
		return &LCPLicenseDocument
	}

	return nil
}

// Sniffs a bitmap image.
func SniffBitmap(context SnifferContext) *MediaType {
	if context.HasFileExtension("avif") || context.HasMediaType("image/avif") {
		return &AVIF
	}
	if context.HasFileExtension("bmp", "dib") || context.HasMediaType("image/bmp", "image/x-bmp") {
		return &BMP
	}
	if context.HasFileExtension("gif") || context.HasMediaType("image/gif") {
		return &GIF
	}
	if context.HasFileExtension("jpg", "jpeg", "jpe", "jif", "jfif", "jfi") || context.HasMediaType("image/jpeg") {
		return &JPEG
	}
	if context.HasFileExtension("jxl") || context.HasMediaType("image/jxl") {
		return &JXL
	}
	if context.HasFileExtension("png") || context.HasMediaType("image/png") {
		return &PNG
	}
	if context.HasFileExtension("tiff", "tif") || context.HasMediaType("image/tiff", "image/tiff-fx") {
		return &TIFF
	}
	if context.HasFileExtension("webp") || context.HasMediaType("image/webp") {
		return &WEBP
	}

	// TODO read magic bytes?

	return nil
}

// Sniffs audio files.
func SniffAudio(context SnifferContext) *MediaType {
	if context.HasFileExtension("aac") || context.HasMediaType("audio/aac") {
		return &AAC
	}
	if context.HasFileExtension("aiff") || context.HasMediaType("audio/aiff") {
		return &AIFF
	}
	// TODO flac, m4a
	if context.HasFileExtension("mp3") || context.HasMediaType("audio/mpeg") {
		return &MP3
	}
	if context.HasFileExtension("ogg", "oga") || context.HasMediaType("audio/ogg") {
		return &OGG
	}
	if context.HasFileExtension("opus") || context.HasMediaType("audio/opus") {
		return &OPUS
	}
	if context.HasFileExtension("wav") || context.HasMediaType("audio/wav") {
		return &WAV
	}
	if context.HasFileExtension("webm") || context.HasMediaType("audio/webm") {
		// Note: .webm extension could also be a video
		return &WEBMAudio
	}

	// TODO read magic bytes?

	return nil
}

// Sniffs a Readium Web Publication, protected or not by LCP.
func SniffWebpub(context SnifferContext) *MediaType {
	if context.HasFileExtension("audiobook") || context.HasMediaType("application/audiobook+zip") {
		return &ReadiumAudiobook
	}
	if context.HasMediaType("application/audiobook+json") {
		return &ReadiumAudiobookManifest
	}

	if context.HasFileExtension("divina") || context.HasMediaType("application/divina+zip") {
		return &Divina
	}
	if context.HasMediaType("application/divina+json") {
		return &DivinaManifest
	}

	if context.HasFileExtension("webpub") || context.HasMediaType("application/webpub+zip") {
		return &ReadiumWebpub
	}
	if context.HasMediaType("application/webpub+json") {
		return &ReadiumWebpubManifest
	}

	if context.HasFileExtension("lcpa") || context.HasMediaType("application/audiobook+lcp") {
		return &LCPProtectedAudiobook
	}
	if context.HasFileExtension("lcpdf") || context.HasMediaType("application/pdf+lcp") {
		return &LCPProtectedPDF
	}

	// isManifest := true
	// TODO implement heavy sniffing, which requires context.ContentAsRWPM()
	// https://github.com/readium/r2-shared-kotlin/blob/develop/r2-shared/src/main/java/org/readium/r2/shared/util/mediatype/Sniffer.kt#L165

	return nil
}

// Sniffs a W3C Web Publication Manifest.
func SniffW3CWPUB(context SnifferContext) *MediaType {
	if js := context.ContentAsJSON(); js != nil {
		if ctx, ok := js["@context"]; ok {
			if context, ok := ctx.([]interface{}); ok {
				for _, v := range context {
					if val, ok := v.(string); ok {
						if val == "https://www.w3.org/ns/wp-context" {
							return &W3CWPUBManifest
						}
					}
				}
			}
		}
	}

	return nil
}

// Sniffs an EPUB publication.
// Reference: https://www.w3.org/publishing/epub3/epub-ocf.html#sec-zip-container-mime
func SniffEPUB(context SnifferContext) *MediaType {
	if context.HasFileExtension("epub") || context.HasMediaType("application/epub+zip") {
		return &EPUB
	}

	if mimetype := context.ReadArchiveEntryAt("mimetype"); mimetype != nil {
		if strings.TrimSpace(string(mimetype)) == "application/epub+zip" {
			return &EPUB
		}
	}

	return nil
}

// Sniffs a Lightweight Packaging Format (LPF).
// References:
//   - https://www.w3.org/TR/lpf/
//   - https://www.w3.org/TR/pub-manifest/
func SniffLPF(context SnifferContext) *MediaType {
	if context.HasFileExtension("lpf") || context.HasMediaType("application/lpf+zip") {
		return &LPF
	}
	if context.ContainsArchiveEntryAt("index.html") {
		return &LPF
	}

	if entry := context.ReadArchiveEntryAt("publication.json"); entry != nil {
		var js map[string]interface{}
		if err := json.Unmarshal(entry, &js); err == nil && js != nil {
			if ctx, ok := js["@context"]; ok {
				if context, ok := ctx.([]interface{}); ok {
					for _, v := range context {
						if val, ok := v.(string); ok {
							if val == "https://www.w3.org/ns/pub-context" {
								return &LPF
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// Authorized extensions for resources in a CBZ archive.
// Reference: https://wiki.mobileread.com/wiki/CBR_and_CBZ
var cbz_extensions = map[string]struct{}{
	"bmp": {}, "dib": {}, "gif": {}, "jif": {}, "jfi": {}, "jfif": {}, "jpg": {}, "jpeg": {}, "png": {}, "tif": {}, "tiff": {}, "webp": {}, // Bitmap. Note there's no AVIF or JXL
	"acbf": {}, "xml": {}, "txt": {}, // Metadata
}

// Authorized extensions for resources in a ZAB archive (Zipped Audio Book).
var zab_extensions = map[string]struct{}{
	"aac": {}, "aiff": {}, "alac": {}, "flac": {}, "m4a": {}, "m4b": {}, "mp3": {}, "ogg": {}, "oga": {}, "mogg": {}, "opus": {}, "wav": {}, "webm": {}, // Audio
	"asx": {}, "bio": {}, "m3u": {}, "m3u8": {}, "pla": {}, "pls": {}, "smil": {}, "vlc": {}, "wpl": {}, "xspf": {}, "zpl": {}, // Playlist
}

// Sniffs a simple Archive-based format, like Comic Book Archive or Zipped Audio Book.
// Reference: https://wiki.mobileread.com/wiki/CBR_and_CBZ
func SniffArchive(context SnifferContext) *MediaType {
	if context.HasFileExtension("cbz") || context.HasMediaType("application/vnd.comicbook+zip", "application/x-cbz", "application/x-cbr") {
		return &CBZ
	}
	if context.HasFileExtension("zab") {
		return &ZAB
	}

	if archive, err := context.ContentAsArchive(); err == nil && archive != nil {
		archiveContainsOnlyExtensions := func(exts map[string]struct{}) bool {
			for _, zf := range archive.Entries() {
				if extensions.IsHiddenOrThumbs(zf.Path()) {
					continue
				}
				fext := filepath.Ext(strings.ToLower(zf.Path()))
				if len(fext) > 1 {
					fext = fext[1:] // Remove "." from extension
				}
				_, contains := exts[fext]
				if !contains { // File extension not it allowed extensions
					return false
				}
			}
			return true
		}

		if archiveContainsOnlyExtensions(cbz_extensions) {
			return &CBZ
		}

		if archiveContainsOnlyExtensions(zab_extensions) {
			return &ZAB
		}
	}

	return nil
}

// Sniffs a PDF document.
// Reference: https://www.loc.gov/preservation/digital/formats/fdd/fdd000123.shtml
func SniffPDF(context SnifferContext) *MediaType {
	if context.HasFileExtension("pdf") || context.HasMediaType("application/pdf") {
		return &PDF
	}
	if string(context.Read(0, 4)) == "%PDF-" {
		return &PDF
	}

	return nil
}

func SniffSystem(context SnifferContext) *MediaType {
	for _, mt := range context.MediaTypes() {
		mts := mt.String()
		exts, err := mime.ExtensionsByType(mts)
		if len(exts) == 0 || err != nil {
			continue
		}
		nm := mime.TypeByExtension(exts[0])
		if nm == "" {
			continue
		}
		nm = strings.TrimSuffix(nm, "; charset=utf-8") // Fix for Go assuming file's content is UTF-8
		exr := exts[0]
		if exr == ".htm" {
			exr = ".html" // Fix for Go's first html extension being .htm
		}
		if nmt, err := New(nm, "", exr[1:]); err == nil {
			return &nmt
		}
	}

	for _, ext := range context.FileExtensions() {
		nm := mime.TypeByExtension("." + ext)
		if nm == "" {
			continue
		}
		exts, err := mime.ExtensionsByType(nm)
		if len(exts) == 0 || err != nil {
			continue
		}
		exr := exts[0]
		if exr == ".htm" {
			exr = ".html" // Fix for Go's first html extension being .htm
		}
		nm = strings.TrimSuffix(nm, "; charset=utf-8") // Fix for Go assuming file's content is UTF-8
		if nmt, err := New(nm, "", exr[1:]); err == nil {
			return &nmt
		}
	}

	// TODO guessContentTypeFromStream equivalent

	return nil
}
