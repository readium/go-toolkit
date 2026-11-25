package lcp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"math"

	"github.com/pkg/errors"
)

// Ciphertext layout produced and expected by this package, compatible with
// the LCP specification and readium-lcp-server:
//
//	IV (16 bytes) || AES-256-CBC(plaintext || padding)
//
// The padding follows the W3C XML Encryption scheme, not PKCS#7: it is 1 to
// 16 bytes long, the last byte holds the padding length and the others are
// arbitrary. A plaintext which is a multiple of the block size gets a full
// block of padding.

const blockSize = aes.BlockSize

// chunkSize is the size of the buffers used when streaming, a multiple of
// the block size.
const chunkSize = 32 * 1024

// CBCOption configures the AES-CBC encryption.
type CBCOption func(*cbcConfig)

type cbcConfig struct {
	random io.Reader
}

// WithRandom sets the source of the initialization vectors and padding bytes,
// which defaults to crypto/rand.
//
// A deterministic source makes the output reproducible, which is useful for
// tests and reproducible builds. Never use one for publications that are
// distributed: reusing an initialization vector with the same key reveals
// which resources start with the same bytes.
func WithRandom(r io.Reader) CBCOption {
	return func(c *cbcConfig) {
		if r != nil {
			c.random = r
		}
	}
}

func newCBCConfig(opts []CBCOption) cbcConfig {
	c := cbcConfig{random: rand.Reader}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// EncryptedLength returns the length of the ciphertext of a plaintext of the
// given length, or -1 if either length is outside the int64 range.
func EncryptedLength(plaintextLength int64) int64 {
	if plaintextLength < 0 || plaintextLength > math.MaxInt64-2*blockSize {
		return -1
	}
	return blockSize + (plaintextLength/blockSize+1)*blockSize
}

// Encrypt encrypts everything read from r and writes the ciphertext to w. It
// returns the number of bytes written.
func Encrypt(key []byte, r io.Reader, w io.Writer, opts ...CBCOption) (int64, error) {
	cw := &countingWriter{w: w}
	ew, err := NewEncryptWriter(key, cw, opts...)
	if err != nil {
		return cw.n, err
	}
	if _, err := io.Copy(ew, r); err != nil {
		return cw.n, err
	}
	err = ew.Close()
	return cw.n, err
}

// Decrypt decrypts everything read from r and writes the plaintext to w. It
// returns the number of bytes written.
func Decrypt(key []byte, r io.Reader, w io.Writer) (int64, error) {
	dr, err := NewDecryptReader(key, r)
	if err != nil {
		return 0, err
	}
	return io.Copy(w, dr)
}

// NewEncryptWriter returns a writer encrypting what is written to it into w.
//
// The initialization vector is written to w immediately. Close must be called
// to write the padding block, completing the ciphertext; it doesn't close w.
func NewEncryptWriter(key []byte, w io.Writer, opts ...CBCOption) (io.WriteCloser, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	config := newCBCConfig(opts)

	iv := make([]byte, blockSize)
	if _, err := io.ReadFull(config.random, iv); err != nil {
		return nil, errors.Wrap(err, "lcp: failed generating initialization vector")
	}
	if _, err := w.Write(iv); err != nil {
		return nil, err
	}

	return &encryptWriter{
		w:      w,
		mode:   cipher.NewCBCEncrypter(block, iv),
		random: config.random,
	}, nil
}

type encryptWriter struct {
	w       io.Writer
	mode    cipher.BlockMode
	random  io.Reader
	pending [blockSize]byte // incomplete block waiting for more data
	npend   int
	out     []byte
	err     error
}

// Write implements io.Writer
func (e *encryptWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	written := len(p)

	// Complete the pending block first.
	if e.npend > 0 {
		n := copy(e.pending[e.npend:], p)
		e.npend += n
		p = p[n:]
		if e.npend < blockSize {
			return written, nil
		}
		if err := e.writeBlocks(e.pending[:]); err != nil {
			return 0, err
		}
		e.npend = 0
	}

	// Encrypt whole blocks straight from p, through the output buffer so
	// that the caller's slice is left untouched.
	whole := len(p) - len(p)%blockSize
	if whole > 0 {
		if err := e.writeBlocks(p[:whole]); err != nil {
			return 0, err
		}
	}
	e.npend = copy(e.pending[:], p[whole:])
	return written, nil
}

func (e *encryptWriter) writeBlocks(src []byte) error {
	if e.out == nil {
		e.out = make([]byte, chunkSize)
	}
	for len(src) > 0 {
		n := min(len(src), len(e.out))
		e.mode.CryptBlocks(e.out[:n], src[:n])
		if _, err := e.w.Write(e.out[:n]); err != nil {
			e.err = err
			return err
		}
		src = src[n:]
	}
	return nil
}

// Close implements io.Closer, writing the final padded block. It doesn't
// close the underlying writer.
func (e *encryptWriter) Close() error {
	if e.err != nil {
		if e.err == errClosed {
			return nil
		}
		return e.err
	}
	padLength := blockSize - e.npend // 1..16
	if padLength > 1 {
		if _, err := io.ReadFull(e.random, e.pending[e.npend:blockSize-1]); err != nil {
			e.err = errors.Wrap(err, "lcp: failed generating padding")
			return e.err
		}
	}
	e.pending[blockSize-1] = byte(padLength)
	if err := e.writeBlocks(e.pending[:]); err != nil {
		return err
	}
	e.err = errClosed
	return nil
}

var errClosed = errors.New("lcp: write to closed encrypt writer")

// NewDecryptReader returns a reader decrypting the ciphertext read from r.
//
// The padding is checked and removed when the end of the ciphertext is
// reached, so a truncated ciphertext or a wrong key is reported by Read as
// [ErrInvalidCiphertext] or [ErrInvalidPadding].
func NewDecryptReader(key []byte, r io.Reader) (io.Reader, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &decryptReader{r: r, block: block}, nil
}

type decryptReader struct {
	r     io.Reader
	block cipher.Block
	mode  cipher.BlockMode // nil until the IV has been read

	in    []byte // ciphertext buffer, len = buffered bytes, cap = chunk + 1 block
	ready []byte // decrypted plaintext available to the caller
	held  []byte // last decrypted block, which may contain the padding
	plain []byte // backing buffer for ready and held
	err   error
}

// Read implements io.Reader
func (d *decryptReader) Read(p []byte) (int, error) {
	for len(d.ready) == 0 {
		if d.err != nil {
			return 0, d.err
		}
		d.fill()
	}
	n := copy(p, d.ready)
	d.ready = d.ready[n:]
	return n, nil
}

// fill decrypts the next chunk of ciphertext into d.ready, or sets d.err.
func (d *decryptReader) fill() {
	if d.mode == nil {
		iv := make([]byte, blockSize)
		if _, err := io.ReadFull(d.r, iv); err != nil {
			d.err = ciphertextError(err)
			return
		}
		d.mode = cipher.NewCBCDecrypter(d.block, iv)
		d.in = make([]byte, 0, chunkSize+blockSize)
		d.plain = make([]byte, chunkSize+2*blockSize)
	}

	// Read until at least one whole block is available, or EOF.
	var rerr error
	for len(d.in) < blockSize && rerr == nil {
		var n int
		n, rerr = d.r.Read(d.in[len(d.in):cap(d.in)])
		d.in = d.in[:len(d.in)+n]
	}
	if rerr != nil && rerr != io.EOF {
		d.err = rerr
		return
	}

	whole := len(d.in) - len(d.in)%blockSize

	// The previously held block can be released now that more ciphertext
	// follows it; the last block decrypted is held back in its place.
	stream := d.plain[:0]
	stream = append(stream, d.held...)
	if whole > 0 {
		start := len(stream)
		stream = stream[:start+whole]
		d.mode.CryptBlocks(stream[start:], d.in[:whole])
		remaining := copy(d.in, d.in[whole:])
		d.in = d.in[:remaining]
	}

	if rerr == io.EOF {
		if len(d.in) != 0 || len(stream) == 0 {
			d.err = ErrInvalidCiphertext
			return
		}
		pad := int(stream[len(stream)-1])
		if pad < 1 || pad > blockSize {
			d.err = ErrInvalidPadding
			return
		}
		d.ready = stream[:len(stream)-pad]
		d.held = nil
		d.err = io.EOF
		return
	}

	if len(stream) <= blockSize {
		d.held = stream
		d.ready = nil
		return
	}
	d.ready = stream[:len(stream)-blockSize]
	d.held = stream[len(stream)-blockSize:]
}

func ciphertextError(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return ErrInvalidCiphertext
	}
	return err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
