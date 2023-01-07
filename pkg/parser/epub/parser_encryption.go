package epub

import (
	"strconv"

	"github.com/readium/go-toolkit/pkg/drm"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
	"github.com/readium/xmlquery"
)

func ParseEncryption(document *xmlquery.Node) (ret map[string]manifest.Encryption) {
	for _, node := range document.SelectElements("//" + NSSelect(NamespaceENC, "EncryptedData")) {
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
	cdat := node.SelectElement(NSSelect(NamespaceENC, "CipherData"))
	if cdat == nil {
		return "", nil
	}
	cipherref := cdat.SelectElement(NSSelect(NamespaceENC, "CipherReference"))
	if cipherref == nil {
		return "", nil
	}
	resourceURI := cipherref.SelectAttr("URI")

	retrievalMethod := ""
	if keyinfo := node.SelectElement(NSSelect(NamespaceSIG, "KeyInfo")); keyinfo != nil {
		if r := keyinfo.SelectElement(NSSelect(NamespaceSIG, "RetrievalMethod")); r != nil {
			retrievalMethod = r.SelectAttr("URI")
		}
	}

	ret := &manifest.Encryption{
		// TODO: No profile? https://github.com/readium/kotlin-toolkit/blob/develop/readium/streamer/src/main/java/org/readium/r2/streamer/parser/epub/EncryptionParser.kt#L40
	}

	if retrievalMethod == "license.lcpl#/encryption/content_key" {
		ret.Scheme = drm.SchemeLCP
	}

	if encryptionmethod := node.SelectElement(NSSelect(NamespaceENC, "EncryptionMethod")); encryptionmethod != nil {
		ret.Algorithm = encryptionmethod.SelectAttr("Algorithm")
	}

	if encryptionproperties := node.SelectElement(NSSelect(NamespaceENC, "EncryptionProperties")); encryptionproperties != nil {
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
	for _, encryptionProperty := range encryptionProperties.SelectElements(NSSelect(NamespaceENC, "EncryptionProperty")) {
		if compressionElement := encryptionProperty.SelectElement(NSSelect(NamespaceCOMP, "Compression")); compressionElement != nil {
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
