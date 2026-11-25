package lcp

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The testdata/compat* fixtures were generated with the readium-lcp-server
// implementation this package replaces (crypto and pack.Do, as vendored in
// river-worker), with the key testKey("lcp-compat"):
//
//   - compat-blob.bin: compatBlobPlaintext() encrypted with the old AES-CBC encrypter.
//   - compat.epub: a small EPUB with a cover, a navigation document, an NCX, two
//     chapters (one with a space in its name), a stylesheet, an image and an
//     IDPF-obfuscated font.
//   - compat-old-lcp.epub: compat.epub encrypted by the old pack.Do.

func compatBlobPlaintext() []byte {
	var b strings.Builder
	for i := 0; b.Len() < 1000; i++ {
		fmt.Fprintf(&b, "line %d of the compatibility blob\n", i)
	}
	return []byte(b.String()[:1000])
}

func TestCompatDecryptOldBlob(t *testing.T) {
	ciphertext, err := os.ReadFile("testdata/compat-blob.bin")
	require.NoError(t, err)
	var out bytes.Buffer
	_, err = Decrypt(testKey("lcp-compat"), bytes.NewReader(ciphertext), &out)
	require.NoError(t, err)
	assert.Equal(t, compatBlobPlaintext(), out.Bytes())
}

func TestCompatDecryptOldEPUB(t *testing.T) {
	key := testKey("lcp-compat")
	assert.Equal(t, protection.LCP, identify(t, "testdata/compat-old-lcp.epub"))

	// Through a publication.
	cleartext := openPublication(t, "testdata/compat.epub", streamer.Config{})
	decrypted, err := DecryptPublication(openPublication(t, "testdata/compat-old-lcp.epub", streamer.Config{}), key)
	require.NoError(t, err)
	assertSameResources(t, cleartext, decrypted)

	// Through the remuxer: every entry is restored.
	source, _ := readZip(t, "testdata/compat.epub")
	restoredPath := decryptEPUBToFile(t, "testdata/compat-old-lcp.epub", key)
	restored, _ := readZip(t, restoredPath)
	for name, src := range source {
		if name == protection.PathEPUBEncryption {
			continue
		}
		assert.True(t, bytes.Equal(src.content, restored[name].content), "%s differs", name)
	}
	assert.Equal(t, encryptionEntries(t, "testdata/compat.epub"), encryptionEntries(t, restoredPath))
}

func TestCompatEncryptionMatchesOldSelection(t *testing.T) {
	// The default selection and compression policy match the old pack.Do on
	// this fixture.
	encrypted := encryptEPUBToFile(t, "testdata/compat.epub", testKey("lcp-compat"), Options{})
	actual := encryptionEntries(t, encrypted)
	expected := encryptionEntries(t, "testdata/compat-old-lcp.epub")
	assert.Equal(t, expected, actual)
}
