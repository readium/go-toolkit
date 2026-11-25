package protection

import (
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
)

const (
	SchemeLCP      = "http://readium.org/2014/01/lcp"
	SchemeAdept    = "http://ns.adobe.com/adept"
	SchemeFairPlay = "http://itunes.apple.com/ns/epub"
)

// Well-known locations and identifiers of LCP-protected packages.
const (
	// PathEPUBEncryption is the location of the encryption document in an EPUB container.
	PathEPUBEncryption = "META-INF/encryption.xml"
	// PathEPUBLCPLicense is the location of the LCP license in an EPUB container.
	PathEPUBLCPLicense = "META-INF/license.lcpl"
	// PathRWPLCPLicense is the location of the LCP license in a Readium Web Publication package.
	PathRWPLCPLicense = "license.lcpl"

	// LCPContentKeyRetrievalURI is the ds:RetrievalMethod URI pointing an
	// EPUB encryption.xml entry at the LCP license's encrypted content key.
	LCPContentKeyRetrievalURI = "license.lcpl#/encryption/content_key"
	// LCPContentKeyRetrievalType is the ds:RetrievalMethod type of
	// [LCPContentKeyRetrievalURI].
	LCPContentKeyRetrievalType = "http://readium.org/2014/01/lcp#EncryptedContentKey"
)

// Font obfuscation algorithms, which XOR the first bytes of a font with a key
// derived from the publication's unique identifier. Obfuscation is not DRM:
// any reading system can undo it.
const (
	// AlgorithmIDPFObfuscation is the IDPF font obfuscation algorithm.
	AlgorithmIDPFObfuscation = "http://www.idpf.org/2008/embedding"
	// AlgorithmAdobeObfuscation is the legacy Adobe font obfuscation algorithm.
	AlgorithmAdobeObfuscation = "http://ns.adobe.com/pdf/enc#RC"
)

// Compressions applied to resources before their encryption, as declared by
// [manifest.Encryption]. Deflate is the only one EPUB defines, as the Method
// "8" of the encryption.xml Compression property, besides "0" for none.
const (
	// CompressionDeflate declares a resource deflated before its encryption.
	CompressionDeflate = "deflate"
	// CompressionNone declares a resource encrypted without compression. It
	// means the same as an empty compression, which this toolkit writes
	// instead, but other producers declare it.
	CompressionNone = "none"
)

// IsFontObfuscation reports whether the encryption algorithm is a font
// obfuscation algorithm.
func IsFontObfuscation(algorithm string) bool {
	return algorithm == AlgorithmIDPFObfuscation || algorithm == AlgorithmAdobeObfuscation
}

// MergeEncryption merges two declarations of the encryption of a resource,
// e.g. by two links, or two entries of an EPUB encryption.xml: the strongest
// constraint on its content applies, a declared original length rather than
// none. Declarations which can't hold together, with different schemes,
// algorithms, compressions or original lengths, are an error. Readers taking
// one of them instead, e.g. the last, would drop a constraint the other
// declares.
func MergeEncryption(a, b manifest.Encryption) (manifest.Encryption, error) {
	compression := func(e manifest.Encryption) string {
		if e.Compression == CompressionNone {
			return ""
		}
		return e.Compression
	}
	switch {
	case a.Scheme != b.Scheme || a.Algorithm != b.Algorithm:
		return a, errors.Errorf("declared encrypted differently, with %s and %s", a.Algorithm, b.Algorithm)
	case compression(a) != compression(b):
		return a, errors.Errorf("declared encrypted with different compressions, %q and %q", a.Compression, b.Compression)
	case a.OriginalLength > 0 && b.OriginalLength > 0 && a.OriginalLength != b.OriginalLength:
		return a, errors.Errorf("declared encrypted with different original lengths, %d and %d", a.OriginalLength, b.OriginalLength)
	}
	a.OriginalLength = max(a.OriginalLength, b.OriginalLength)
	return a, nil
}
