package epub

import (
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/drm"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
)

func ParseEncryption(document *xmlquery.Node) (ret map[string]manifest.Encryption) {
	for _, node := range document.SelectElements("//EncryptedData[namespace-uri()='" + NamespaceENC + "']") {
		u, e := parseEncryptedData(node)
		if e != nil {
			ret[u] = *e
		}
	}
	return
}

func parseEncryptedData(node *xmlquery.Node) (string, *manifest.Encryption) {
	cdat := node.SelectElement("CipherData[namespace-uri()='" + NamespaceENC + "']")
	if cdat == nil {
		return "", nil
	}
	cipherref := cdat.SelectElement("CipherReference[namespace-uri()='" + NamespaceENC + "']")
	if cipherref == nil {
		return "", nil
	}
	resourceURI := cipherref.SelectAttr("URI")

	retrievalMethod := ""
	if keyinfo := node.SelectElement("KeyInfo[namespace-uri()='" + NamespaceSIG + "']"); keyinfo != nil {
		if retrivalmethod := keyinfo.SelectElement("RetrievalMethod[namespace-uri()='" + NamespaceSIG + "']"); retrivalmethod != nil {
			retrievalMethod = retrivalmethod.SelectAttr("URI")
		}
	}

	ret := &manifest.Encryption{
		// No profile? https://github.com/readium/kotlin-toolkit/blob/develop/readium/streamer/src/main/java/org/readium/r2/streamer/parser/epub/EncryptionParser.kt#L40
	}

	if retrievalMethod == "license.lcpl#/encryption/content_key" {
		ret.Scheme = drm.SchemeLCP
	}

	if encryptionmethod := node.SelectElement("EncryptionMethod[namespace-uri()='" + NamespaceENC + "']"); encryptionmethod != nil {
		ret.Algorithm = encryptionmethod.SelectAttr("Algorithm")
	}

	if encryptionproperties := node.SelectElement("EncryptionProperties[namespace-uri()='" + NamespaceENC + "']"); encryptionproperties != nil {
		originalLength, method := parseEncryptionProperties(encryptionproperties)
		if method != "" {
			ret.Compression = method
			ret.OriginalLength = originalLength
		}
	}

	ru, _ := util.NewHREF(resourceURI, "").String()
	return ru, ret
}

func parseEncryptionProperties(encryptionProperties *xmlquery.Node) (int64, string) {
	for _, encryptionProperty := range encryptionProperties.SelectElements("EncryptionProperty[namespace-uri()='" + NamespaceENC + "']") {
		if compressionElement := encryptionProperty.SelectElement("Compression[namespace-uri()='" + NamespaceCOMP + "']"); compressionElement != nil {
			if originalLength, method := parseCompressionElement(compressionElement); method != "" {
				return originalLength, method
			}
		}
	}
	return -1, ""
}

func parseCompressionElement(compressionElement *xmlquery.Node) (int64, string) {
	originalLength, err := strconv.ParseInt(compressionElement.SelectAttr("OriginalLength"), 10, 64)
	if err != nil {
		return -1, ""
	}
	method := compressionElement.SelectAttr("Method")
	if method == "" {
		return -1, ""
	}
	if method == "8" {
		return originalLength, "deflate"
	} else {
		return originalLength, "none"
	}
}
