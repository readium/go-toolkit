package lcp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func textBytes(n int) []byte {
	return []byte(strings.Repeat("All work and no play makes Jack a dull boy. ", n/44+1)[:n])
}

func TestEncryptContentRoundTrip(t *testing.T) {
	key := testKey("content")
	for _, compress := range []bool{false, true} {
		for _, size := range []int{0, 1, 16, 1000, 100_000} {
			plain := textBytes(size)
			var ciphertext bytes.Buffer
			enc, err := EncryptContent(key, bytes.NewReader(plain), &ciphertext, compress)
			require.NoError(t, err)

			expected := manifest.Encryption{
				Scheme:         Scheme,
				Algorithm:      AlgorithmAES256CBC,
				OriginalLength: int64(size),
			}
			if compress {
				expected.Compression = CompressionDeflate
				if size > 1000 {
					assert.Less(t, ciphertext.Len(), size/2, "compressible content should shrink")
				}
			} else {
				assert.EqualValues(t, EncryptedLength(int64(size)), ciphertext.Len())
			}
			assert.Equal(t, expected, enc)

			var out bytes.Buffer
			n, err := DecryptContent(key, &ciphertext, &out, enc)
			require.NoError(t, err)
			assert.EqualValues(t, size, n)
			assert.Equal(t, plain, out.Bytes())
		}
	}
}

func TestDecryptContentRejectsOtherAlgorithms(t *testing.T) {
	_, err := DecryptContent(testKey("x"), bytes.NewReader(nil), &bytes.Buffer{}, manifest.Encryption{
		Algorithm: "http://www.idpf.org/2008/embedding",
	})
	assert.ErrorContains(t, err, "unsupported encryption algorithm")
}

func TestShouldCompress(t *testing.T) {
	for mt, expected := range map[*mediatype.MediaType]bool{
		nil:                         true,
		&mediatype.XHTML:            true,
		&mediatype.HTML:             true,
		&mediatype.CSS:              true,
		&mediatype.SVG:              true,
		&mediatype.JSON:             true,
		&mediatype.NCX:              true,
		&mediatype.OTF:              true,
		&mediatype.JPEG:             false,
		&mediatype.PNG:              false,
		&mediatype.WEBP:             false,
		&mediatype.MPEGAudio:        false,
		&mediatype.MPEGVideo:        false,
		&mediatype.MP4:              false,
		&mediatype.PDF:              false,
		&mediatype.ZIP:              false,
		&mediatype.EPUB:             false,
		&mediatype.ReadiumAudiobook: false,
	} {
		name := "nil"
		if mt != nil {
			name = mt.String()
		}
		assert.Equal(t, expected, ShouldCompress(mt), name)
	}
}

func TestIsLCPEncrypted(t *testing.T) {
	assert.True(t, IsLCPEncrypted(&manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC}))
	assert.False(t, IsLCPEncrypted(nil))
	assert.False(t, IsLCPEncrypted(&manifest.Encryption{Algorithm: AlgorithmAES256CBC}))
	// Obfuscated fonts in an LCP publication.
	assert.False(t, IsLCPEncrypted(&manifest.Encryption{Scheme: Scheme, Algorithm: "http://www.idpf.org/2008/embedding"}))
}
