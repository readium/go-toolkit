package epub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadEncryption(ctx context.Context, name string) (map[string]manifest.Encryption, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/encryption/encryption-"+name+".xml"), map[string]string{
		NamespaceENC:  "enc",
		NamespaceSIG:  "ds",
		NamespaceCOMP: "comp",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	enc := ParseEncryption(n)
	ret := make(map[string]manifest.Encryption)
	for k, v := range enc {
		ret[k.String()] = v
	}

	return ret, nil
}

var testEncMap = map[string]manifest.Encryption{
	url.MustURLFromString("OEBPS/xhtml/chapter01.xhtml").String(): {
		Scheme:         "http://readium.org/2014/01/lcp",
		OriginalLength: 13291,
		Algorithm:      "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
		Compression:    "deflate",
	},
	url.MustURLFromString("OEBPS/xhtml/chapter02.xhtml").String(): {
		Scheme:         "http://readium.org/2014/01/lcp",
		OriginalLength: 12914,
		Algorithm:      "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
		Compression:    "none",
	},
}

func TestEncryptionParserNamespacePrefixes(t *testing.T) {
	e, err := loadEncryption(t.Context(), "lcp-prefixes")
	require.NoError(t, err)
	assert.Equal(t, testEncMap, e)
}

func TestEncryptionParserDefaultNamespaces(t *testing.T) {
	e, err := loadEncryption(t.Context(), "lcp-xmlns")
	require.NoError(t, err)
	assert.Equal(t, testEncMap, e)
}

func TestEncryptionParserUnknownRetrievalMethod(t *testing.T) {
	e, err := loadEncryption(t.Context(), "unknown-method")
	require.NoError(t, err)
	assert.Equal(t, map[string]manifest.Encryption{
		url.MustURLFromString("OEBPS/images/image.jpeg").String(): {
			Algorithm: "http://www.w3.org/2001/04/xmlenc#kw-aes128",
		},
		url.MustURLFromString("OEBPS/xhtml/chapter.xhtml").String(): {
			Algorithm:      "http://www.w3.org/2001/04/xmlenc#kw-aes128",
			Compression:    "deflate",
			OriginalLength: 12914,
		},
	}, e)
}
