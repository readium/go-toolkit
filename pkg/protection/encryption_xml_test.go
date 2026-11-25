package protection

import (
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalEPUBEncryptionCompression(t *testing.T) {
	marshal := func(compression string, originalLength int64) (string, error) {
		data, err := MarshalEPUBEncryption([]EncryptionEntry{{
			Href: "OPS/chapter.xhtml",
			Encryption: manifest.Encryption{
				Algorithm:      "http://www.w3.org/2001/04/xmlenc#aes256-cbc",
				Compression:    compression,
				OriginalLength: originalLength,
			},
		}})
		return string(data), err
	}

	for compression, method := range map[string]string{"deflate": "8", "none": "0", "": "0"} {
		data, err := marshal(compression, 42)
		require.NoError(t, err, compression)
		assert.Contains(t, data, `<comp:Compression Method="`+method+`" OriginalLength="42">`, compression)
	}

	data, err := marshal("", 0)
	require.NoError(t, err)
	assert.NotContains(t, data, "comp:Compression")

	// Other compressions can't be declared: "0" would have reading systems
	// serve the resource still compressed.
	_, err = marshal("zstd", 42)
	assert.EqualError(t, err, `OPS/chapter.xhtml: compression "zstd" can't be declared in encryption.xml, which only supports deflate`)
}

func TestMarshalEPUBEncryptionSizeLimit(t *testing.T) {
	// Documents readers refuse used to be written.
	_, err := MarshalEPUBEncryption([]EncryptionEntry{{
		Href:       strings.Repeat("a", MaxMetadataSize),
		Encryption: manifest.Encryption{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc"},
	}})
	assert.EqualError(t, err, "encryption.xml would be larger than 32 MiB, the size limit of readers")
}
