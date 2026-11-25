package epub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadEncryption(ctx context.Context, name string, scheme protection.Scheme) (map[string]manifest.Encryption, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/encryption/encryption-"+name+".xml"))
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseEncryption(n, scheme.URI()), nil
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
		Compression:    "",
	},
}

func TestEncryptionParserNamespacePrefixes(t *testing.T) {
	e, err := loadEncryption(t.Context(), "lcp-prefixes", protection.LCP)
	require.NoError(t, err)
	assert.Equal(t, testEncMap, e)
}

func TestEncryptionParserDefaultNamespaces(t *testing.T) {
	e, err := loadEncryption(t.Context(), "lcp-xmlns", protection.LCP)
	require.NoError(t, err)
	assert.Equal(t, testEncMap, e)
}

func TestEncryptionParserUnknownRetrievalMethod(t *testing.T) {
	e, err := loadEncryption(t.Context(), "unknown-method", protection.NoDRM)
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

func TestEncryptionParserCompression(t *testing.T) {
	e, err := loadEncryption(t.Context(), "compression", protection.NoDRM)
	require.NoError(t, err)
	entry := func(compression string, originalLength int64) manifest.Encryption {
		return manifest.Encryption{
			Algorithm:      "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
			Compression:    compression,
			OriginalLength: originalLength,
		}
	}
	assert.Equal(t, map[string]manifest.Encryption{
		// Method "0" is no compression, left out of manifests rather than "none".
		"OEBPS/stored.xhtml": entry("", 100),
		// The Method tells how to read the resource, whatever its
		// OriginalLength: it used to be ignored along with an invalid one.
		"OEBPS/no-length.xhtml":       entry("deflate", 0),
		"OEBPS/negative-length.xhtml": entry("deflate", 0),
		// A Method EPUB doesn't define used to be taken for no compression.
		"OEBPS/zstd.xhtml": entry("93", 100),
		// Without a Method, nothing is declared.
		"OEBPS/no-method.xhtml": entry("", 0),
	}, e)
}

func TestEncryptionParserPerEntryScheme(t *testing.T) {
	lcpEntry := manifest.Encryption{
		Scheme:    "http://readium.org/2014/01/lcp",
		Algorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
	}
	idpfFont := manifest.Encryption{Algorithm: "http://www.idpf.org/2008/embedding"}
	adobeFont := manifest.Encryption{Algorithm: "http://ns.adobe.com/pdf/enc#RC"}

	// In an LCP container, only entries pointing at the license are LCP,
	// and obfuscated fonts never carry a DRM scheme.
	e, err := loadEncryption(t.Context(), "lcp-mixed", protection.LCP)
	require.NoError(t, err)
	assert.Equal(t, map[string]manifest.Encryption{
		"OEBPS/xhtml/chapter01.xhtml": lcpEntry,
		"OEBPS/fonts/font.otf":        idpfFont,
		"OEBPS/fonts/font2.otf":       adobeFont,
		"OEBPS/images/image.jpeg":     {Algorithm: "http://www.w3.org/2001/04/xmlenc#kw-aes128"},
	}, e)

	// In another DRM's container, non-LCP and non-obfuscation entries
	// inherit the container scheme. The LCP retrieval method still wins.
	e, err = loadEncryption(t.Context(), "lcp-mixed", protection.Adept)
	require.NoError(t, err)
	assert.Equal(t, map[string]manifest.Encryption{
		"OEBPS/xhtml/chapter01.xhtml": lcpEntry,
		"OEBPS/fonts/font.otf":        idpfFont,
		"OEBPS/fonts/font2.otf":       adobeFont,
		"OEBPS/images/image.jpeg": {
			Scheme:    "http://ns.adobe.com/adept",
			Algorithm: "http://www.w3.org/2001/04/xmlenc#kw-aes128",
		},
	}, e)
}
