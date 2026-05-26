package epub

import (
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpEncEncData    = mustCompileNS("//enc:EncryptedData")
	xpEncCipherData = mustCompileNS("enc:CipherData")
	xpEncCipherRef  = mustCompileNS("enc:CipherReference")
	xpEncKeyInfo    = mustCompileNS("ds:KeyInfo")
	xpEncRetrieval  = mustCompileNS("ds:RetrievalMethod")
	xpEncMethod     = mustCompileNS("enc:EncryptionMethod")
	xpEncProps      = mustCompileNS("enc:EncryptionProperties")
	xpEncProp       = mustCompileNS("enc:EncryptionProperty")
	xpEncCompress   = mustCompileNS("comp:Compression")
)

func ParseEncryption(document *xmlquery.Node) (ret map[url.URL]manifest.Encryption) {
	for _, node := range xmlquery.QuerySelectorAll(document, xpEncEncData) {
		u, e := parseEncryptedData(node)
		if e != nil {
			if ret == nil {
				ret = make(map[url.URL]manifest.Encryption)
			}
			ret[u] = *e
		}
	}
	return
}

func parseEncryptedData(node *xmlquery.Node) (url.URL, *manifest.Encryption) {
	cdat := xmlquery.QuerySelector(node, xpEncCipherData)
	if cdat == nil {
		return nil, nil
	}
	cipherref := xmlquery.QuerySelector(cdat, xpEncCipherRef)
	if cipherref == nil {
		return nil, nil
	}
	resourceURI := cipherref.SelectAttr("URI")

	retrievalMethod := ""
	if keyinfo := xmlquery.QuerySelector(node, xpEncKeyInfo); keyinfo != nil {
		if r := xmlquery.QuerySelector(keyinfo, xpEncRetrieval); r != nil {
			retrievalMethod = r.SelectAttr("URI")
		}
	}

	ret := &manifest.Encryption{
		// TODO: No profile? https://github.com/readium/kotlin-toolkit/blob/develop/readium/streamer/src/main/java/org/readium/r2/streamer/parser/epub/EncryptionParser.kt#L40
	}

	if retrievalMethod == "license.lcpl#/encryption/content_key" {
		ret.Scheme = protection.SchemeLCP
	}

	if encryptionmethod := xmlquery.QuerySelector(node, xpEncMethod); encryptionmethod != nil {
		ret.Algorithm = encryptionmethod.SelectAttr("Algorithm")
	}

	if encryptionproperties := xmlquery.QuerySelector(node, xpEncProps); encryptionproperties != nil {
		originalLength, method := parseEncryptionProperties(encryptionproperties)
		if method != "" {
			ret.Compression = method
			ret.OriginalLength = originalLength
		}
	}

	ru, err := url.FromEPUBHref(resourceURI)
	if err != nil {
		return nil, nil
	}

	return ru, ret
}

func parseEncryptionProperties(encryptionProperties *xmlquery.Node) (int64, string) {
	for _, encryptionProperty := range xmlquery.QuerySelectorAll(encryptionProperties, xpEncProp) {
		if compressionElement := xmlquery.QuerySelector(encryptionProperty, xpEncCompress); compressionElement != nil {
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
