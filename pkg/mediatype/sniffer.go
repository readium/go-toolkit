package mediatype

import (
	"archive/zip"
	"encoding/json"
	"mime"
	"path/filepath"
	"strings"
)

type Sniffer func(context SnifferContext) *MediaType

// Sniffs an HTML document.
func SniffHTML(context SnifferContext) *MediaType {
	if context.HasFileExtension("htm", "html", "xht", "xhtml") || context.HasMediaType("text/html", "application/xhtml+xml") {
		return &HTML
	}

	// [contentAsXml] will fail if the HTML is not a proper XML document, hence the doctype check after this.
	if cxml := context.ContentAsXML(); cxml != nil {
		if cxml.XMLName.Local == "html" {
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
	if context.HasMediaType("application/atom+xml;profile=opds-catalog") {
		return &OPDS1
	}
	if context.HasMediaType("application/atom+xml;type=entry;profile=opds-catalog") {
		return &OPDS1_ENTRY
	}

	// OPDS 2 (Light)
	if context.HasMediaType("application/opds+json") {
		return &OPDS2
	}
	if context.HasMediaType("application/opds-publication+json") {
		return &OPDS2_PUBLICATION
	}

	// OPDS Authentication Document (Light)
	if context.HasMediaType("application/opds-authentication+json") || context.HasMediaType("application/vnd.opds.authentication.v1.0+json") {
		return &OPDS_AUTHENTICATION
	}

	// OPDS 1 (Heavy)
	if cxml := context.ContentAsXML(); cxml != nil {
		if cxml.XMLName.Space == "http://www.w3.org/2005/Atom" {
			if cxml.XMLName.Local == "feed" {
				return &OPDS1
			} else if cxml.XMLName.Local == "entry" {
				return &OPDS1_ENTRY
			}
		}
	}

	// OPDS 2 (Heavy)
	// TODO requires context.ContentAsRWPM()

	// OPDS Authentication Document (Heavy)
	if context.ContainsJSONKeys("id", "title", "authentication") {
		return &OPDS_AUTHENTICATION
	}

	return nil
}

// Sniffs an LCP License Document.
func SniffLCPLicense(context SnifferContext) *MediaType {
	if context.HasFileExtension("lcpl") || context.HasMediaType("application/vnd.readium.lcp.license.v1.0+json") {
		return &LCP_LICENSE_DOCUMENT
	}
	if context.ContainsJSONKeys("id", "issued", "provider", "encryption") {
		return &LCP_LICENSE_DOCUMENT
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

// Sniffs a Readium Web Publication, protected or not by LCP.
func SniffWebpub(context SnifferContext) *MediaType {
	if context.HasFileExtension("audiobook") || context.HasMediaType("application/audiobook+zip") {
		return &READIUM_AUDIOBOOK
	}
	if context.HasMediaType("application/audiobook+json") {
		return &READIUM_AUDIOBOOK_MANIFEST
	}

	if context.HasFileExtension("divina") || context.HasMediaType("application/divina+zip") {
		return &DIVINA
	}
	if context.HasMediaType("application/divina+json") {
		return &DIVINA_MANIFEST
	}

	if context.HasFileExtension("webpub") || context.HasMediaType("application/webpub+zip") {
		return &READIUM_WEBPUB
	}
	if context.HasMediaType("application/webpub+json") {
		return &READIUM_WEBPUB_MANIFEST
	}

	if context.HasFileExtension("lcpa") || context.HasMediaType("application/audiobook+lcp") {
		return &LCP_PROTECTED_AUDIOBOOK
	}
	if context.HasFileExtension("lcpdf") || context.HasMediaType("application/pdf+lcp") {
		return &LCP_PROTECTED_PDF
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
							return &W3C_WPUB_MANIFEST
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
//  - https://www.w3.org/TR/lpf/
//  - https://www.w3.org/TR/pub-manifest/
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
var cbz_extensions = []string{
	"bmp", "dib", "gif", "jif", "jfi", "jfif", "jpg", "jpeg", "png", "tif", "tiff", "webp", // Bitmap. Note there's no AVIF or JXL
	"acbf", "xml", // Metadata
}

// Authorized extensions for resources in a ZAB archive (Zipped Audio Book).
var zab_extensions = []string{
	"aac", "aiff", "alac", "flac", "m4a", "m4b", "mp3", "ogg", "oga", "mogg", "opus", "wav", "webm", // Audio
	"asx", "bio", "m3u", "m3u8", "pla", "pls", "smil", "vlc", "wpl", "xspf", "zpl", // Playlist
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
		isIgnored := func(file *zip.File) bool {
			if strings.HasPrefix(file.Name, ".") || strings.HasPrefix(file.Name, "__MACOSX") || file.Name == "Thumbs.db" {
				return true
			}
			return false
		}
		archiveContainsOnlyExtensions := func(exts []string) bool {
			for _, zf := range archive.File {
				if isIgnored(zf) || zf.FileInfo().IsDir() {
					continue
				}
				contains := false
				fext := filepath.Ext(strings.ToLower(zf.Name))
				if len(fext) > 1 {
					fext = fext[1:] // Remove "." from extension
				}
				for _, v := range exts {
					if v == fext {
						contains = true
						break
					}
				}
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
		if nmt, err := NewMediaType(nm, "", exts[0]); err == nil {
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
		if nmt, err := NewMediaType(nm, "", exts[0]); err == nil {
			return &nmt
		}
	}

	// TODO guessContentTypeFromStream equivalent

	return nil
}
