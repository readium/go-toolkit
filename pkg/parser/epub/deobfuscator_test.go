package epub

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const identifier = "urn:uuid:36d5078e-ff7d-468e-a5f3-f47c14b91f2f"

func withDeobfuscator(t *testing.T, href string, algorithm string, start, end int64, f func([]byte, []byte)) {
	ft := fetcher.NewFileFetcher("deobfuscation", "./testdata/deobfuscation")
	t.Log(href)

	// Cleartext font
	clean, err := ft.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("deobfuscation/cut-cut.woff", false)}).Read(t.Context(), start, end)
	if !assert.Nil(t, err) {
		require.NoError(t, err.Cause)
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
	obfu, err := NewDeobfuscator(identifier).Transform(ft.Get(t.Context(), link)).Read(t.Context(), start, end)
	if !assert.Nil(t, err) {
		require.NoError(t, err.Cause)
		f(nil, nil)
		return
	}
	f(clean, obfu)

	bbuff := new(bytes.Buffer)
	_, err = NewDeobfuscator(identifier).Transform(ft.Get(t.Context(), link)).Stream(t.Context(), bbuff, start, end)
	if !assert.Nil(t, err) {
		require.NoError(t, err.Cause)
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

func TestDeobfuscatorRanges(t *testing.T) {
	for _, algorithm := range []struct {
		name string
		href string
		uri  string
	}{
		{"IDPF", "deobfuscation/cut-cut.obf.woff", "http://www.idpf.org/2008/embedding"},
		{"Adobe", "deobfuscation/cut-cut.adb.woff", "http://ns.adobe.com/pdf/enc#RC"},
	} {
		for _, span := range [][2]int64{{1, 2}, {13, 31}, {13, 2000}, {1023, 2000}, {1024, 2000}, {1039, 2000}, {1040, 2000}} {
			t.Run(fmt.Sprintf("%s/%d-%d", algorithm.name, span[0], span[1]), func(t *testing.T) {
				withDeobfuscator(t, algorithm.href, algorithm.uri, span[0], span[1], func(clean, obfu []byte) {
					assert.Equal(t, clean, obfu)
				})
			})
		}
	}
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
