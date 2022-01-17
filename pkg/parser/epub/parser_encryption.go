package epub

import (
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/drm"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
)

func ParseEncryption(document *xmlquery.Node) (ret map[string]manifest.Encryption) {
	for _, node := range document.SelectElements("//*[namespace-uri()='" + NamespaceENC + "' and local-name()='EncryptedData']") {
		u, e := parseEncryptedData(node)
		if e != nil {
			if ret == nil {
				ret = make(map[string]manifest.Encryption)
			}
			ret[u] = *e
		}
	}
	return
}

func parseEncryptedData(node *xmlquery.Node) (string, *manifest.Encryption) {
	cdat := node.SelectElement("*[namespace-uri()='" + NamespaceENC + "' and local-name()='CipherData']")
	if cdat == nil {
		return "", nil
	}
	cipherref := cdat.SelectElement("*[namespace-uri()='" + NamespaceENC + "' and local-name()='CipherReference']")
	if cipherref == nil {
		return "", nil
	}
	resourceURI := cipherref.SelectAttr("URI")

	retrievalMethod := ""
	if keyinfo := node.SelectElement("*[namespace-uri()='" + NamespaceSIG + "' and local-name()='KeyInfo']"); keyinfo != nil {
		if r := keyinfo.SelectElement("*[namespace-uri()='" + NamespaceSIG + "' and local-name()='RetrievalMethod']"); r != nil {
			retrievalMethod = r.SelectAttr("URI")
		}
	}

	ret := &manifest.Encryption{
		// TODO: No profile? https://github.com/readium/kotlin-toolkit/blob/develop/readium/streamer/src/main/java/org/readium/r2/streamer/parser/epub/EncryptionParser.kt#L40
	}

	if retrievalMethod == "license.lcpl#/encryption/content_key" {
		ret.Scheme = drm.SchemeLCP
	}

	if encryptionmethod := node.SelectElement("*[namespace-uri()='" + NamespaceENC + "' and local-name()='EncryptionMethod']"); encryptionmethod != nil {
		ret.Algorithm = encryptionmethod.SelectAttr("Algorithm")
	}

	if encryptionproperties := node.SelectElement("*[namespace-uri()='" + NamespaceENC + "' and local-name()='EncryptionProperties']"); encryptionproperties != nil {
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
	for _, encryptionProperty := range encryptionProperties.SelectElements("*[namespace-uri()='" + NamespaceENC + "' and local-name()='EncryptionProperty']") {
		if compressionElement := encryptionProperty.SelectElement("*[namespace-uri()='" + NamespaceCOMP + "' and local-name()='Compression']"); compressionElement != nil {
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
