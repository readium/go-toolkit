package epub

import (
	"bytes"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

const identifier = "urn:uuid:36d5078e-ff7d-468e-a5f3-f47c14b91f2f"

func withDeobfuscator(t *testing.T, href string, algorithm string, start, end int64, f func([]byte, []byte)) {
	ft := fetcher.NewFileFetcher("deobfuscation", "./testdata/deobfuscation")
	t.Log(href)

	// Cleartext font
	clean, err := ft.Get(manifest.Link{Href: manifest.MustNewHREFFromString("deobfuscation/cut-cut.woff", false)}).Read(start, end)
	if !assert.Nil(t, err) {
		assert.NoError(t, err.Cause)
		f(nil, nil)
		return
	}

	// Obfuscated font
	link := manifest.Link{
		Href: manifest.MustNewHREFFromString(href, false),
	}
	if algorithm != "" {
		link.Properties = manifest.Properties{
			"encrypted": map[string]interface{}{
				"algorithm": algorithm,
			},
		}
	}
	obfu, err := NewDeobfuscator(identifier).Transform(ft.Get(link)).Read(start, end)
	if !assert.Nil(t, err) {
		assert.NoError(t, err.Cause)
		f(nil, nil)
		return
	}
	f(clean, obfu)

	bbuff := new(bytes.Buffer)
	_, err = NewDeobfuscator(identifier).Transform(ft.Get(link)).Stream(bbuff, start, end)
	if !assert.Nil(t, err) {
		assert.NoError(t, err.Cause)
		f(nil, nil)
		return
	}
	f(clean, bbuff.Bytes())
}

func TestDeobfuscatorIDPF(t *testing.T) {
	withDeobfuscator(t, "deobfuscation/cut-cut.obf.woff", "http://www.idpf.org/2008/embedding", 0, 0, func(clean, obfu []byte) {
		assert.Equal(t, clean, obfu)
	})
}

func TestDeobfuscatorIDPFRangeIn(t *testing.T) {
	withDeobfuscator(t, "deobfuscation/cut-cut.obf.woff", "http://www.idpf.org/2008/embedding", 20, 40, func(clean, obfu []byte) {
		assert.Equal(t, clean, obfu)
	})
}

func TestDeobfuscatorIDPFRangeOut(t *testing.T) {
	withDeobfuscator(t, "deobfuscation/cut-cut.obf.woff", "http://www.idpf.org/2008/embedding", 60, 2000, func(clean, obfu []byte) {
		assert.Equal(t, clean, obfu)
	})
}

func TestDeobfuscatorAdobe(t *testing.T) {
	withDeobfuscator(t, "deobfuscation/cut-cut.adb.woff", "http://ns.adobe.com/pdf/enc#RC", 0, 0, func(clean, obfu []byte) {
		assert.Equal(t, clean, obfu)
	})
}

func TestDeobfuscatorNoAlgorithm(t *testing.T) {
	withDeobfuscator(t, "deobfuscation/cut-cut.woff", "", 0, 0, func(clean, obfu []byte) {
		assert.Equal(t, clean, obfu)
	})
}

func TestDeobfuscatorUnknownAlgorithm(t *testing.T) {
	withDeobfuscator(t, "deobfuscation/cut-cut.woff", "unknown algorithm", 0, 0, func(clean, obfu []byte) {
		assert.Equal(t, clean, obfu)
	})
}
