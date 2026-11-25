package lcp

import (
	"bufio"
	"compress/flate"
	"io"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
)

// ShouldCompress is the default policy deciding whether a resource is
// deflated before being encrypted.
//
// Compression is skipped for media which is already compressed (bitmap
// images, audio, video, ZIP-based formats) and for PDF. Besides gaining
// nothing, compressing those would prevent reading systems from accessing
// them with byte ranges, e.g. to seek in an audio or video file. Resources
// of unknown media type are compressed.
func ShouldCompress(mt *mediatype.MediaType) bool {
	if mt == nil {
		return true
	}
	return !mt.IsPrecompressed() && !mt.Matches(&mediatype.PDF)
}

// EncryptContent encrypts the content read from src with the content key, and
// writes the result to dst. When compress is true, the content is deflated
// before being encrypted.
//
// It streams the content, using a constant amount of memory, and returns the
// [manifest.Encryption] describing the encrypted resource, with its
// original length.
func EncryptContent(key ContentKey, src io.Reader, dst io.Writer, compress bool, opts ...CBCOption) (manifest.Encryption, error) {
	enc := manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC}

	ew, err := NewEncryptWriter(key, dst, opts...)
	if err != nil {
		return enc, err
	}

	var n int64
	if compress {
		enc.Compression = CompressionDeflate
		fw, err := flate.NewWriter(ew, flate.BestCompression)
		if err != nil {
			return enc, err
		}
		if n, err = io.Copy(fw, src); err != nil {
			return enc, errors.Wrap(err, "lcp: failed compressing content")
		}
		if err = fw.Close(); err != nil {
			return enc, errors.Wrap(err, "lcp: failed compressing content")
		}
	} else if n, err = io.Copy(ew, src); err != nil {
		return enc, errors.Wrap(err, "lcp: failed encrypting content")
	}

	if err = ew.Close(); err != nil {
		return enc, errors.Wrap(err, "lcp: failed encrypting content")
	}
	enc.OriginalLength = n
	return enc, nil
}

// DefaultMaxInflatedLength is the default maximum length of a deflated
// resource read whole into memory, once inflated.
//
// A few kilobytes of deflated content can inflate to gigabytes, and the
// original length a publication declares is as untrusted as its content.
// Reading a deflated resource whole fails beyond this limit, whatever its
// declared length, with [ErrResourceTooLarge]. Streaming it, or reading
// ranges of it, is not limited, as it doesn't hold the whole resource in
// memory: LCP protects any kind of resource, including large audio and video
// files, which some encoders deflate as well. See
// [ContentProtection.MaxInflatedLength] to change it.
const DefaultMaxInflatedLength = 256 << 20 // 256 MiB

// inflationLimit returns the maximum inflated length for a configured one: 0
// means the default, and a negative value no limit.
func inflationLimit(maxInflated int64) int64 {
	switch {
	case maxInflated == 0:
		return DefaultMaxInflatedLength
	case maxInflated < 0:
		return -1
	default:
		return maxInflated
	}
}

// DecryptContent decrypts the content read from src with the content key,
// according to enc, and writes the result to dst. It returns the number of
// bytes written.
//
// It fails with [ErrInvalidCiphertext] when the content is longer than the
// original length declared by enc, if any.
func DecryptContent(key ContentKey, src io.Reader, dst io.Writer, enc manifest.Encryption) (int64, error) {
	r, err := newContentReader(key, src, enc)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(dst, r)
	if err != nil {
		return n, errors.Wrap(err, "lcp: failed decrypting content")
	}
	return n, nil
}

// checkDecryptable returns an error unless enc describes content this package
// can decrypt: encrypted with [AlgorithmAES256CBC], and either deflated or
// not compressed. Taking an unknown compression for none would serve the
// content still compressed.
func checkDecryptable(enc manifest.Encryption) error {
	if enc.Algorithm != AlgorithmAES256CBC {
		return errors.Errorf("lcp: unsupported encryption algorithm %q", enc.Algorithm)
	}
	switch enc.Compression {
	case "", protection.CompressionNone, CompressionDeflate:
		return nil
	}
	return errors.Errorf("lcp: unsupported compression %q", enc.Compression)
}

// newContentReader returns a reader decrypting, and inflating if needed, the
// content read from src. It fails when the content is longer than its
// declared original length.
func newContentReader(key ContentKey, src io.Reader, enc manifest.Encryption) (io.Reader, error) {
	if err := checkDecryptable(enc); err != nil {
		return nil, err
	}
	r, err := NewDecryptReader(key, src)
	if err != nil {
		return nil, err
	}
	if enc.Compression == CompressionDeflate {
		// The inflater reads byte by byte from tail, so that it stops right
		// after the deflated content: tail must then be at the end of the
		// decrypted content, whose padding is only checked there.
		tail := bufio.NewReader(r)
		r = &finalizedReader{r: flate.NewReader(tail), tail: tail}
	}
	if enc.OriginalLength > 0 {
		// Consumers rely on the declared length, e.g. to allocate buffers.
		r = &lengthLimitedReader{r: r, remaining: enc.OriginalLength,
			err: errors.Wrap(ErrInvalidCiphertext, "content is longer than its declared original length")}
	}
	return r, nil
}

// finalizedReader reads deflated content from r, and fails at its end when
// tail, the decrypted content r inflates, isn't at its end as well. Reaching
// the end of the decrypted content checks its padding: without it, a reader
// would accept corrupted ciphertext, and data appended after the deflated
// content.
type finalizedReader struct {
	r    io.Reader
	tail *bufio.Reader
}

func (f *finalizedReader) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if err == io.EOF {
		if _, terr := f.tail.ReadByte(); terr != io.EOF {
			if terr == nil {
				terr = errors.Wrap(ErrInvalidCiphertext, "data after the deflated content")
			}
			return n, terr
		}
	}
	return n, err
}

// lengthLimitedReader fails with err when the content read from r is longer
// than remaining.
type lengthLimitedReader struct {
	r         io.Reader
	remaining int64
	err       error
}

func (l *lengthLimitedReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if int64(n) > l.remaining {
		n = int(l.remaining)
		l.remaining = 0
		return n, l.err
	}
	l.remaining -= int64(n)
	return n, err
}

// compressedLength returns the length of the content deflated the way
// [EncryptContent] does, without keeping the compressed bytes.
func compressedLength(src io.Reader) (int64, error) {
	cw := &countingWriter{w: io.Discard}
	fw, err := flate.NewWriter(cw, flate.BestCompression)
	if err != nil {
		return 0, err
	}
	if _, err := io.Copy(fw, src); err != nil {
		return 0, err
	}
	if err := fw.Close(); err != nil {
		return 0, err
	}
	return cw.n, nil
}
