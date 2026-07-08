package mediatype

import (
	"context"
	"encoding/json"
	"mime"
	"path/filepath"
	"strings"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

type Sniffer func(ctx context.Context, context SnifferContext) *MediaType

// Sniffs an XHTML document.
// Must precede the HTML sniffer.
func SniffXHTML(ctx context.Context, context SnifferContext) *MediaType {
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
func SniffHTML(ctx context.Context, context SnifferContext) *MediaType {
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
func SniffOPDS(ctx context.Context, context SnifferContext) *MediaType {
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
			switch cxml.XMLName.Local {
			case "feed":
				return &OPDS1
			case "entry":
				return &OPDS1Entry
			}
		}
	}

	// OPDS 2 (Heavy)
	// Only classify JSON that decodes as a RWPM (a `metadata` object with a title),
	// like the Kotlin toolkit, so arbitrary JSON carrying an OPDS-vocabulary link
	// isn't misread as an OPDS 2 document.
	if js := context.ContentAsJSON(); js != nil && rwpmHasMetadataTitle(js) {
		if rwpmSelfLinkMatches(js, &OPDS2) {
			return &OPDS2
		}
		if rwpmHasLinkWithRelPrefix(js, "http://opds-spec.org/acquisition") {
			return &OPDS2Publication
		}
	}

	// OPDS Authentication Document (Heavy)
	if context.ContainsJSONKeys("id", "title", "authentication") {
		return &OPDSAuthentication
	}

	return nil
}

// Sniffs an LCP License Document.
func SniffLCPLicense(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("lcpl") || context.HasMediaType("application/vnd.readium.lcp.license.v1.0+json") {
		return &LCPLicenseDocument
	}
	if context.ContainsJSONKeys("id", "issued", "provider", "encryption") {
		return &LCPLicenseDocument
	}

	return nil
}

// Sniffs a bitmap image.
func SniffBitmap(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("avif", "avifs") || context.HasMediaType("image/avif") {
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
func SniffAudio(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("aac") || context.HasMediaType("audio/aac") {
		return &AAC
	}
	if context.HasFileExtension("aiff", "aif", "aifc") || context.HasMediaType("audio/aiff") {
		return &AIFF
	}
	if context.HasFileExtension("flac") || context.HasMediaType("audio/flac") {
		return &FLAC
	}
	if context.HasFileExtension("mp3") || context.HasMediaType("audio/mpeg") {
		return &MPEGAudio
	}
	if context.HasFileExtension("mp4", "m4a", "m4b", "m4p", "m4r", "alac") || context.HasMediaType("audio/mp4") {
		return &MP4
	}
	if context.HasFileExtension("ogg", "oga", "mogg") || context.HasMediaType("audio/ogg") {
		return &OGG
	}
	if context.HasFileExtension("opus") || context.HasMediaType("audio/opus") {
		return &OPUS
	}
	if context.HasFileExtension("wav", "wave") || context.HasMediaType("audio/wav", "audio/x-wav", "audio/wave") {
		return &WAV
	}
	if context.HasFileExtension("webm") || context.HasMediaType("audio/webm") {
		return &WEBMAudio
	}

	// TODO read magic bytes?

	return nil
}

// Sniffs a Readium Web Publication, protected or not by LCP.
func SniffWebpub(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("audiobook") || context.HasMediaType("application/audiobook+zip") {
		return &ReadiumAudiobook
	}
	if context.HasMediaType("application/audiobook+json") {
		return &ReadiumAudiobookManifest
	}

	if context.HasFileExtension("divina") || context.HasMediaType("application/divina+zip") {
		return &ReadiumDivina
	}
	if context.HasMediaType("application/divina+json") {
		return &ReadiumDivinaManifest
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

	// Heavy sniffing.
	// Reads a RWPM, either as a bare manifest or from a `manifest.json` entry in a package.
	isManifest := true
	rwpm := context.ContentAsJSON()
	if !isRWPMJSON(rwpm) {
		rwpm = nil
		if bin := context.ReadArchiveEntryAt(ctx, "manifest.json"); bin != nil {
			var js map[string]interface{}
			if json.Unmarshal(bin, &js) == nil && isRWPMJSON(js) {
				isManifest = false
				rwpm = js
			}
		}
	}
	if rwpm == nil {
		return nil
	}

	isLCPProtected := !isManifest &&
		(context.ContainsArchiveEntryAt(ctx, "license.lcpl") || rwpmHasLCPScheme(rwpm))

	if rwpmConformsTo(rwpm, rwpmProfileAudiobook, MediaType.IsAudio) {
		if isManifest {
			return &ReadiumAudiobookManifest
		}
		if isLCPProtected {
			return &LCPProtectedAudiobook
		}
		return &ReadiumAudiobook
	}
	if rwpmConformsTo(rwpm, rwpmProfileDivina, MediaType.IsBitmap) {
		if isManifest {
			return &ReadiumDivinaManifest
		}
		return &ReadiumDivina
	}
	if isLCPProtected && rwpmConformsTo(rwpm, rwpmProfilePDF, func(mt MediaType) bool { return mt.Matches(&PDF) }) {
		return &LCPProtectedPDF
	}
	if rwpmSelfLinkMatches(rwpm, &ReadiumWebpubManifest) {
		if isManifest {
			return &ReadiumWebpubManifest
		}
		return &ReadiumWebpub
	}
	if !isManifest {
		// Any package containing a RWPM is a Readium Web Publication.
		return &ReadiumWebpub
	}

	return nil
}

// Sniffs a W3C Web Publication Manifest.
func SniffW3CWPUB(ctx context.Context, context SnifferContext) *MediaType {
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
func SniffEPUB(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("epub") || context.HasMediaType("application/epub+zip") {
		return &EPUB
	}

	if mimetype := context.ReadArchiveEntryAt(ctx, "mimetype"); mimetype != nil {
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
func SniffLPF(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("lpf") || context.HasMediaType("application/lpf+zip") {
		return &LPF
	}
	if context.ContainsArchiveEntryAt(ctx, "index.html") {
		return &LPF
	}

	if entry := context.ReadArchiveEntryAt(ctx, "publication.json"); entry != nil {
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
	// Bitmaps
	"bmp": {}, "dib": {}, "gif": {}, "jif": {}, "jfi": {}, "jfif": {}, "jpg": {},
	"jpeg": {}, "png": {}, "tif": {}, "tiff": {}, "webp": {}, "avif": {}, "jxl": {},

	// Metadata
	"acbf": {}, "xml": {}, "txt": {}, "json": {},
}

// Authorized extensions for resources in a ZAB archive (Zipped Audio Book).
var zab_extensions = map[string]struct{}{
	"aac": {}, "aiff": {}, "aif": {}, "aifc": {}, "alac": {}, "flac": {},
	"m4a": {}, "m4b": {}, "mp3": {}, "mp4": {}, "m4r": {}, "m4p": {},
	"ogg": {}, "oga": {}, "mogg": {}, "opus": {}, "wav": {}, "wave": {},
	"webm": {}, // Audio
	"asx":  {}, "bio": {}, "m3u": {}, "m3u8": {}, "pla": {}, "pls": {},
	"smil": {}, "vlc": {}, "wpl": {}, "xspf": {}, "zpl": {}, "cue": {}, "log": {}, // Playlist
}

// Sniffs a simple Archive-based format, like Comic Book Archive or Zipped Audio Book.
// Reference: https://wiki.mobileread.com/wiki/CBR_and_CBZ
func SniffArchive(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("cbz") || context.HasMediaType("application/vnd.comicbook+zip", "application/x-cbz", "application/x-cbr") {
		return &CBZ
	}
	if context.HasFileExtension("zab") {
		return &ZAB
	}

	if archive, err := context.ContentAsArchive(ctx); err == nil && archive != nil {
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
func SniffPDF(ctx context.Context, context SnifferContext) *MediaType {
	if context.HasFileExtension("pdf") || context.HasMediaType("application/pdf") {
		return &PDF
	}
	if string(context.Read(0, 4)) == "%PDF-" {
		return &PDF
	}

	return nil
}

func SniffKnown(context SnifferContext) *MediaType {
	for k, v := range knownMatches {
		if context.HasMediaType(k) {
			return v
		}
		if v.fileExtension != "" {
			if v.fileExtension != "json" && context.HasFileExtension(v.fileExtension) {
				return v
			}
		}
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
		nm = strings.TrimSuffix(nm, "; charset=utf-8") // Fix for Go assuming file's content is UTF-8
		if nmt, err := New(nm, "", exr[1:]); err == nil {
			return &nmt
		}
	}

	// TODO guessContentTypeFromStream equivalent

	return nil
}
