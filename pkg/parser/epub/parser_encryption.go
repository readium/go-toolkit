package epub

import (
	"path"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpEncEncData    = mustCompileNS("//enc:EncryptedData")
	xpEncCipherData = mustCompileNS("enc:CipherData")
	xpEncCipherRef  = mustCompileNS("enc:CipherReference")
	xpEncMethod     = mustCompileNS("enc:EncryptionMethod")
	xpEncProps      = mustCompileNS("enc:EncryptionProperties")
	xpEncProp       = mustCompileNS("enc:EncryptionProperty")
	xpEncCompress   = mustCompileNS("comp:Compression")
	xpEncRetrieval  = mustCompileNS("ds:KeyInfo/ds:RetrievalMethod")
)

// ParseEncryption parses an EPUB META-INF/encryption.xml document into a map
// of normalized resource hrefs to their [manifest.Encryption].
//
// The DRM scheme is assigned per entry, given the container-level [scheme]:
//   - entries whose key is retrieved from an LCP license get [protection.SchemeLCP];
//   - font obfuscation entries never carry a scheme, obfuscation not being DRM;
//   - other entries get the container-level scheme, except in LCP containers,
//     where only the entries pointing at the license are LCP-encrypted.
//
// A compression Method EPUB doesn't define is kept as the compression, such
// as "93", rather than taken for no compression: consumers then refuse the
// resource instead of serving it still compressed.
//
// An href declared several times takes its last declaration: see
// [ParseEncryptionEntries] for all of them, and [protection.MergeEncryption]
// to merge them, as the parser does.
func ParseEncryption(document *xmlquery.Node, scheme string) (ret map[string]manifest.Encryption) {
	for _, entry := range ParseEncryptionEntries(document, scheme) {
		if ret == nil {
			ret = make(map[string]manifest.Encryption)
		}
		ret[entry.Href] = entry.Encryption
	}
	return
}

// ParseEncryptionEntries parses the entries of an EPUB META-INF/encryption.xml
// document, as [ParseEncryption] does, in their order, including those of an
// href declared several times.
func ParseEncryptionEntries(document *xmlquery.Node, scheme string) []protection.EncryptionEntry {
	var entries []protection.EncryptionEntry
	for _, node := range xmlquery.QuerySelectorAll(document, xpEncEncData) {
		if key, e := parseEncryptedData(node, scheme); e != nil {
			entries = append(entries, protection.EncryptionEntry{Href: key, Encryption: *e})
		}
	}
	return entries
}

// ResourceID returns the identifier of a resource in encryption data, such as
// [PublicationFactory.EncryptionData], and whether it is a resource of the
// container.
//
// A resource of the container is identified by its decoded path, which
// archives resolve, without a leading slash: hrefs spelled differently, such
// as "chapter.xhtml" and "%63hapter.xhtml", are the same resource, as they are
// to fetchers and the LCP remuxes. A remote resource, an absolute URL or a
// network-path reference such as "//cdn.example.com/font.otf", is identified
// by its URL after a slash, so that the two never collide: "file:font.otf" is
// a URL with the scheme "file", while "./file:font.otf" is the file
// "file:font.otf" of the container. Fragments, positions within resources,
// are left out.
func ResourceID(u url.URL) (id string, local bool) {
	if url.IsRemoteReference(u) {
		// Without its fragment, a position within the resource.
		normalized := *u.Normalize().Raw()
		normalized.Fragment, normalized.RawFragment = "", ""
		return "/" + normalized.String(), false
	}
	return strings.TrimPrefix(path.Clean(u.Path()), "/"), true
}

func parseEncryptedData(node *xmlquery.Node, scheme string) (string, *manifest.Encryption) {
	cdat := xmlquery.QuerySelector(node, xpEncCipherData)
	if cdat == nil {
		return "", nil
	}
	cipherref := xmlquery.QuerySelector(cdat, xpEncCipherRef)
	if cipherref == nil {
		return "", nil
	}
	resourceURI := cipherref.SelectAttr("URI")

	ret := &manifest.Encryption{}

	if encryptionmethod := xmlquery.QuerySelector(node, xpEncMethod); encryptionmethod != nil {
		ret.Algorithm = encryptionmethod.SelectAttr("Algorithm")
	}
	ret.Scheme = entryScheme(node, ret.Algorithm, scheme)

	if encryptionproperties := xmlquery.QuerySelector(node, xpEncProps); encryptionproperties != nil {
		ret.OriginalLength, ret.Compression = parseEncryptionProperties(encryptionproperties)
	}

	ru, err := url.FromEPUBHref(resourceURI)
	if err != nil {
		return "", nil
	}

	return ru.Normalize().String(), ret
}

// entryScheme returns the DRM scheme of a single EncryptedData entry, given
// the container-level scheme. See [ParseEncryption].
func entryScheme(node *xmlquery.Node, algorithm string, containerScheme string) string {
	if protection.IsFontObfuscation(algorithm) {
		return ""
	}
	if rm := xmlquery.QuerySelector(node, xpEncRetrieval); rm != nil && rm.SelectAttr("URI") == protection.LCPContentKeyRetrievalURI {
		return protection.SchemeLCP
	}
	if containerScheme == protection.SchemeLCP {
		return ""
	}
	return containerScheme
}

// parseEncryptionProperties returns what the first Compression element with
// a Method declares.
func parseEncryptionProperties(encryptionProperties *xmlquery.Node) (int64, string) {
	for _, encryptionProperty := range xmlquery.QuerySelectorAll(encryptionProperties, xpEncProp) {
		if compressionElement := xmlquery.QuerySelector(encryptionProperty, xpEncCompress); compressionElement != nil {
			if length, compression, found := parseCompressionElement(compressionElement); found {
				return length, compression
			}
		}
	}
	return 0, ""
}

// parseCompressionElement returns the original length of a resource, 0 when
// unknown, and the compression a Compression element declares for it. found
// is false when the element has no Method, and so declares nothing.
//
// EPUB defines the Method "0" (no compression), returned as no compression
// rather than as [protection.CompressionNone] so that manifests leave it out,
// and "8" (Deflate). Other methods are returned as they are. The Method is
// what tells how to read the resource: it is honored even when the
// OriginalLength, a mere size hint, is missing or invalid.
func parseCompressionElement(compressionElement *xmlquery.Node) (originalLength int64, compression string, found bool) {
	method := compressionElement.SelectAttr("Method")
	if method == "" {
		return 0, "", false
	}
	originalLength, err := strconv.ParseInt(compressionElement.SelectAttr("OriginalLength"), 10, 64)
	if err != nil || originalLength < 0 {
		originalLength = 0
	}
	switch method {
	case "0":
		return originalLength, "", true
	case "8":
		return originalLength, protection.CompressionDeflate, true
	}
	return originalLength, method, true
}
