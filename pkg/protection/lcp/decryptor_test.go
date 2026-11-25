package lcp

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encryptedResource encrypts plain and returns it as a resource whose link
// declares the resulting encryption.
func encryptedResource(t *testing.T, key ContentKey, plain []byte, compress bool) (fetcher.Resource, manifest.Encryption) {
	t.Helper()
	var ciphertext bytes.Buffer
	enc, err := EncryptContent(key, bytes.NewReader(plain), &ciphertext, compress)
	require.NoError(t, err)
	link := manifest.Link{
		Href:       manifest.MustNewHREFFromString("resource", false),
		Properties: manifest.Properties{"encrypted": enc.ToMap()},
	}
	data := ciphertext.Bytes()
	return fetcher.NewBytesResource(link, func() []byte { return data }), enc
}

var testRanges = [][2]int64{
	{0, 0}, {0, 15}, {0, 16}, {15, 16}, {16, 31}, {17, 17}, {1, 9998},
	{4000, 8191}, {9990, 9999}, {9999, 9999}, {9990, 20000}, {10000, 10005},
}

func expectedRange(plain []byte, start, end int64) []byte {
	from, to, _ := resolveRange(start, end, int64(len(plain)))
	return plain[from:to]
}

func TestDecryptorRandomAccess(t *testing.T) {
	key := testKey("decryptor")
	d, err := NewDecryptor(key)
	require.NoError(t, err)

	for _, compress := range []bool{false, true} {
		plain := randomBytes(10_000)
		if compress {
			plain = textBytes(10_000)
		}
		encrypted, _ := encryptedResource(t, key, plain, compress)
		res := d.Transform(encrypted)
		require.IsType(t, &decryptingResource{}, res)

		length, rerr := res.Length(t.Context())
		require.Nil(t, rerr)
		assert.EqualValues(t, len(plain), length)

		for _, r := range testRanges {
			data, rerr := res.Read(t.Context(), r[0], r[1])
			require.Nil(t, rerr, "range %v", r)
			assert.Equal(t, expectedRange(plain, r[0], r[1]), data, "compress=%v range %v", compress, r)

			var buf bytes.Buffer
			n, rerr := res.Stream(t.Context(), &buf, r[0], r[1])
			require.Nil(t, rerr, "range %v", r)
			assert.EqualValues(t, buf.Len(), n)
			assert.True(t, bytes.Equal(expectedRange(plain, r[0], r[1]), buf.Bytes()), "compress=%v range %v", compress, r)
		}

		_, rerr = res.Read(t.Context(), 10, 5)
		require.NotNil(t, rerr)
		assert.Equal(t, fetcher.CodeRequestedRangeNotSatisfiable, rerr.Code)
	}
}

func TestDecryptorLengthWithoutOriginalLength(t *testing.T) {
	key := testKey("length")
	for _, size := range []int{1, 15, 16, 17, 1000} {
		plain := randomBytes(size)
		encrypted, enc := encryptedResource(t, key, plain, false)
		enc.OriginalLength = 0

		res := Decryptor{key: key}.decrypt(encrypted, enc)
		length, rerr := res.Length(t.Context())
		require.Nil(t, rerr)
		assert.EqualValues(t, size, length)

		data, rerr := res.Read(t.Context(), 0, int64(size))
		require.Nil(t, rerr)
		assert.Equal(t, plain, data)
	}
}

func TestDecryptorWrongKey(t *testing.T) {
	encrypted, _ := encryptedResource(t, testKey("right"), textBytes(5000), true)
	res := Decryptor{key: testKey("wrong")}.Transform(encrypted)
	_, rerr := res.Read(t.Context(), 0, 0)
	assert.NotNil(t, rerr)
	_, rerr = res.Stream(t.Context(), &bytes.Buffer{}, 0, 0)
	assert.NotNil(t, rerr)
}

func TestDecryptorPassesThroughOtherResources(t *testing.T) {
	d, err := NewDecryptor(testKey("pass"))
	require.NoError(t, err)

	for name, props := range map[string]manifest.Properties{
		"no encryption": nil,
		"obfuscated font in LCP publication": {"encrypted": manifest.Encryption{
			Scheme:    Scheme,
			Algorithm: "http://www.idpf.org/2008/embedding",
		}.ToMap()},
		"other DRM": {"encrypted": manifest.Encryption{
			Scheme:    "http://ns.adobe.com/adept",
			Algorithm: AlgorithmAES256CBC,
		}.ToMap()},
	} {
		res := fetcher.NewBytesResource(manifest.Link{Properties: props}, func() []byte { return []byte("x") })
		assert.Same(t, res, d.Transform(res), name)
	}

	_, err = NewDecryptor(ContentKey("short"))
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestDecryptingResourceHidesCompressedForm(t *testing.T) {
	// The obfuscated font of the fixture is a deflated archive entry.
	af, err := fetcher.NewArchiveFetcherFromPath(t.Context(), "testdata/compat-old-lcp.epub")
	require.NoError(t, err)
	defer af.Close()
	entry := af.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("OEBPS/fonts/font.otf", false)})
	require.True(t, entry.(fetcher.CompressedResource).CompressedAs(archive.CompressionMethodDeflate))

	res := Decryptor{key: testKey("x")}.decrypt(entry, manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC})
	cres, ok := res.(fetcher.CompressedResource)
	require.True(t, ok)
	assert.False(t, cres.CompressedAs(archive.CompressionMethodDeflate))
	assert.EqualValues(t, -1, cres.CompressedLength(t.Context()))
	assert.Nil(t, cres.CRC32Checksum(t.Context()))
	_, rerr := cres.ReadCompressed(t.Context())
	assert.NotNil(t, rerr)
}

func TestProtectedResourcesNegativeRanges(t *testing.T) {
	key := testKey("ranges")
	plain := []byte("a chapter with more than one block of text")
	for _, compress := range []bool{false, true} {
		src := fetcher.NewBytesResource(manifest.Link{}, func() []byte { return plain })
		enc, err := newEncryptingResource(src, key, compress, rand.Reader)
		require.NoError(t, err)
		ciphertext, rerr := enc.Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		protected, declaration := encryptedResource(t, key, plain, compress)
		dec := Decryptor{key: key}.decrypt(protected, declaration)
		for _, tc := range []struct {
			r    fetcher.Resource
			want []byte
		}{{enc, ciphertext}, {dec, plain}} {
			data, rerr := tc.r.Read(t.Context(), -1, 0)
			require.Nil(t, rerr)
			assert.Equal(t, tc.want, data)
			var buf bytes.Buffer
			n, rerr := tc.r.Stream(t.Context(), &buf, -1, 0)
			require.Nil(t, rerr)
			assert.EqualValues(t, len(tc.want), n)
			assert.Equal(t, tc.want, buf.Bytes())
			_, rerr = tc.r.Read(t.Context(), -2, -1)
			require.NotNil(t, rerr)
			assert.Equal(t, fetcher.CodeRequestedRangeNotSatisfiable, rerr.Code)
		}
	}
}
