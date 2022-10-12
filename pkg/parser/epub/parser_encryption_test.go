package epub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func loadEncryption(name string) (map[string]manifest.Encryption, error) {
	n, rerr := fetcher.NewFileResource(manifest.Link{}, "./testdata/encryption/encryption-"+name+".xml").ReadAsXML(map[string]string{
		NamespaceENC:  "enc",
		NamespaceSIG:  "ds",
		NamespaceCOMP: "comp",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseEncryption(n), nil
}

var testEncMap = map[string]manifest.Encryption{
	"/OEBPS/xhtml/chapter01.xhtml": {
		Scheme:         "http://readium.org/2014/01/lcp",
		OriginalLength: 13291,
		Algorithm:      "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
		Compression:    "deflate",
	},
	"/OEBPS/xhtml/chapter02.xhtml": {
		Scheme:         "http://readium.org/2014/01/lcp",
		OriginalLength: 12914,
		Algorithm:      "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
		Compression:    "none",
	},
}

func TestEncryptionParserNamespacePrefixes(t *testing.T) {
	e, err := loadEncryption("lcp-prefixes")
	assert.NoError(t, err)
	assert.Equal(t, testEncMap, e)
}

func TestEncryptionParserDefaultNamespaces(t *testing.T) {
	e, err := loadEncryption("lcp-xmlns")
	assert.NoError(t, err)
	assert.Equal(t, testEncMap, e)
}

func TestEncryptionParserUnknownRetrievalMethod(t *testing.T) {
	e, err := loadEncryption("unknown-method")
	assert.NoError(t, err)
	assert.Equal(t, map[string]manifest.Encryption{
		"/OEBPS/xhtml/chapter.xhtml": {
			Algorithm:      "http://www.w3.org/2001/04/xmlenc#kw-aes128",
			Compression:    "deflate",
			OriginalLength: 12914,
		},
		"/OEBPS/images/image.jpeg": {
			Algorithm: "http://www.w3.org/2001/04/xmlenc#kw-aes128",
		},
	}, e)
}
