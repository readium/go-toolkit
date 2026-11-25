package lcp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"io"
	"math"
	"math/rand/v2"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(seed string) ContentKey {
	k := sha256.Sum256([]byte(seed))
	return k[:]
}

// seededReader is a deterministic randomness source for tests.
func seededReader(seed uint64) io.Reader {
	return rand.NewChaCha8([32]byte{byte(seed), byte(seed >> 8)})
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = seededReader(uint64(n)).Read(b)
	return b
}

var testSizes = []int{0, 1, 15, 16, 17, 31, 32, 33, 4095, 4096, chunkSize - 1, chunkSize, chunkSize + 1, 1 << 20}

func TestCBCRoundTrip(t *testing.T) {
	key := testKey("round-trip")
	for _, size := range testSizes {
		plain := randomBytes(size)

		var cipherBuf bytes.Buffer
		n, err := Encrypt(key, bytes.NewReader(plain), &cipherBuf)
		require.NoError(t, err, "size %d", size)
		assert.Equal(t, EncryptedLength(int64(size)), n, "size %d", size)
		assert.EqualValues(t, cipherBuf.Len(), n, "size %d", size)

		var out bytes.Buffer
		m, err := Decrypt(key, bytes.NewReader(cipherBuf.Bytes()), &out)
		require.NoError(t, err, "size %d", size)
		assert.EqualValues(t, size, m)
		assert.True(t, bytes.Equal(plain, out.Bytes()), "size %d", size)
	}
}

func TestCBCChunkedWritesAndReads(t *testing.T) {
	key := testKey("chunks")
	plain := randomBytes(100_003)

	var reference []byte
	for _, chunk := range []int{1, 7, 16, 4096} {
		var cipherBuf bytes.Buffer
		w, err := NewEncryptWriter(key, &cipherBuf, WithRandom(seededReader(1)))
		require.NoError(t, err)
		for p := plain; len(p) > 0; {
			n := min(chunk, len(p))
			written, err := w.Write(p[:n])
			require.NoError(t, err)
			require.Equal(t, n, written)
			p = p[n:]
		}
		require.NoError(t, w.Close())

		// The chunking must not influence the output.
		if reference == nil {
			reference = cipherBuf.Bytes()
		} else {
			assert.Equal(t, reference, cipherBuf.Bytes(), "chunk %d", chunk)
		}

		// Decrypt through readers returning a single byte at a time, and
		// through a reader delivering EOF along with the last bytes.
		for name, r := range map[string]io.Reader{
			"one byte": iotest.OneByteReader(bytes.NewReader(cipherBuf.Bytes())),
			"data+EOF": iotest.DataErrReader(bytes.NewReader(cipherBuf.Bytes())),
		} {
			dr, err := NewDecryptReader(key, r)
			require.NoError(t, err)
			out, err := io.ReadAll(dr)
			require.NoError(t, err, name)
			assert.True(t, bytes.Equal(plain, out), name)
		}
	}
}

func TestCBCPaddingFormat(t *testing.T) {
	key := testKey("padding")
	for _, size := range []int{0, 1, 15, 16, 17} {
		var cipherBuf bytes.Buffer
		_, err := Encrypt(key, bytes.NewReader(randomBytes(size)), &cipherBuf)
		require.NoError(t, err)

		// Decrypt raw, without removing the padding.
		raw := cipherBuf.Bytes()
		block, err := aes.NewCipher(key)
		require.NoError(t, err)
		plain := make([]byte, len(raw)-blockSize)
		cipher.NewCBCDecrypter(block, raw[:blockSize]).CryptBlocks(plain, raw[blockSize:])

		pad := int(plain[len(plain)-1])
		assert.Equal(t, blockSize-size%blockSize, pad, "size %d", size)
	}
}

func TestCBCDeterministicWithRandom(t *testing.T) {
	key := testKey("deterministic")
	plain := randomBytes(1000)
	encrypt := func(opts ...CBCOption) []byte {
		var buf bytes.Buffer
		_, err := Encrypt(key, bytes.NewReader(plain), &buf, opts...)
		require.NoError(t, err)
		return buf.Bytes()
	}
	assert.Equal(t, encrypt(WithRandom(seededReader(42))), encrypt(WithRandom(seededReader(42))))
	// The default source never repeats IVs.
	assert.NotEqual(t, encrypt(), encrypt())
}

func TestCBCErrors(t *testing.T) {
	key := testKey("errors")

	_, err := NewEncryptWriter(key[:16], io.Discard)
	assert.ErrorIs(t, err, ErrInvalidKey)
	_, err = NewDecryptReader(nil, bytes.NewReader(nil))
	assert.ErrorIs(t, err, ErrInvalidKey)

	var cipherBuf bytes.Buffer
	_, err = Encrypt(key, bytes.NewReader(randomBytes(100)), &cipherBuf)
	require.NoError(t, err)
	ciphertext := cipherBuf.Bytes()

	for name, input := range map[string][]byte{
		"empty":             {},
		"partial IV":        ciphertext[:10],
		"IV only":           ciphertext[:blockSize],
		"misaligned":        ciphertext[:len(ciphertext)-1],
		"trailing garbage":  append(bytes.Clone(ciphertext), 1, 2, 3),
		"partial block end": ciphertext[:blockSize+5],
	} {
		_, err := Decrypt(key, bytes.NewReader(input), io.Discard)
		assert.ErrorIs(t, err, ErrInvalidCiphertext, name)
	}

	// Corrupting the last block (through the previous one's ciphertext)
	// breaks the padding byte in a predictable way.
	corrupted := bytes.Clone(ciphertext)
	corrupted[len(corrupted)-blockSize-1] ^= 0xFF
	_, err = Decrypt(key, bytes.NewReader(corrupted), io.Discard)
	assert.ErrorIs(t, err, ErrInvalidPadding)

	// Errors of the underlying reader and writer are propagated.
	_, err = Decrypt(key, iotest.ErrReader(io.ErrClosedPipe), io.Discard)
	assert.ErrorIs(t, err, io.ErrClosedPipe)
	_, err = Encrypt(key, iotest.ErrReader(io.ErrClosedPipe), io.Discard)
	assert.ErrorIs(t, err, io.ErrClosedPipe)
	_, err = Encrypt(key, bytes.NewReader(randomBytes(100)), errWriter{})
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

func TestCBCWrongKey(t *testing.T) {
	var cipherBuf bytes.Buffer
	_, err := Encrypt(testKey("right"), bytes.NewReader([]byte("cleartext")), &cipherBuf)
	require.NoError(t, err)

	var out bytes.Buffer
	_, err = Decrypt(testKey("wrong"), bytes.NewReader(cipherBuf.Bytes()), &out)
	if err == nil {
		// One in sixteen wrong keys yields a valid-looking padding.
		assert.NotEqual(t, "cleartext", out.String())
	} else {
		assert.ErrorIs(t, err, ErrInvalidPadding)
	}
}

func TestEncryptWriterCloseTwice(t *testing.T) {
	w, err := NewEncryptWriter(testKey("close"), io.Discard)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	assert.NoError(t, w.Close())
	_, err = w.Write([]byte("late"))
	assert.Error(t, err)
}

func TestContentKey(t *testing.T) {
	k, err := GenerateContentKey()
	require.NoError(t, err)
	assert.Len(t, k, KeySize)
	assert.NoError(t, k.Validate())

	decoded, err := ContentKeyFromBase64(k.Base64())
	require.NoError(t, err)
	assert.Equal(t, k, decoded)

	_, err = ContentKeyFromBase64("AAAA")
	assert.ErrorIs(t, err, ErrInvalidKey)
	_, err = ContentKeyFromBase64("not base64!")
	assert.Error(t, err)
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

func TestEncryptedLengthBounds(t *testing.T) {
	assert.EqualValues(t, int64(math.MaxInt64)-15, EncryptedLength(math.MaxInt64-32))
	assert.EqualValues(t, -1, EncryptedLength(math.MaxInt64-31))
	assert.EqualValues(t, -1, EncryptedLength(-1))
}
