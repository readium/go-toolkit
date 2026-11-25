package lcp

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"io"
	"math"
	"slices"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// Decryptor decrypts LCP-encrypted resources on the fly, given the
// publication's content key.
type Decryptor struct {
	key         ContentKey
	maxInflated int64 // see inflationLimit
}

// NewDecryptor returns a [Decryptor] using the given content key. It keeps a
// copy of the key, so that the caller may clear its own.
func NewDecryptor(key ContentKey) (Decryptor, error) {
	if err := key.Validate(); err != nil {
		return Decryptor{}, err
	}
	return Decryptor{key: slices.Clone(key)}, nil
}

// Transform is a [fetcher.ResourceTransformer] decrypting the resources
// whose link declares LCP encryption (see [IsLCPEncrypted]). Other resources
// are returned unchanged.
//
// It relies on the link the resource was retrieved with. To decrypt a whole
// publication, use [DecryptPublication], which keeps track of the encrypted
// resources itself.
func (d Decryptor) Transform(res fetcher.Resource) fetcher.Resource {
	enc := res.Link().Properties.Encryption()
	if enc != nil && enc.Scheme == Scheme {
		// An algorithm of the LCP scheme which isn't supported is an
		// error, rather than the resource served still encrypted.
		if err := checkLCPAlgorithm(res.Link().Href.String(), *enc); err != nil {
			res.Close()
			return fetcher.NewFailureResource(res.Link(), fetcher.Other(err))
		}
	}
	if !IsLCPEncrypted(enc) {
		return res
	}
	return d.decrypt(res, *enc)
}

func (d Decryptor) decrypt(res fetcher.Resource, enc manifest.Encryption) fetcher.Resource {
	// Range reads of uncompressed resources don't go through DecryptContent.
	if err := checkDecryptable(enc); err != nil {
		res.Close()
		return fetcher.NewFailureResource(res.Link(), fetcher.Other(errors.Wrap(err, res.Link().Href.String())))
	}
	return &decryptingResource{
		opaqueResource: opaqueResource{fetcher.ProxyResource{Res: res}},
		key:            d.key,
		maxInflated:    d.maxInflated,
		enc:            enc,
	}
}

// decryptingResource serves the plaintext of an LCP-encrypted resource.
//
// Resources which were not compressed before encryption support efficient
// random access: AES-CBC block i only needs ciphertext block i-1 as its
// initialization vector, so a range is decrypted by reading just one block
// more than the range itself. Deflated resources have to be decrypted and
// inflated from the start, but only the requested range is kept in memory.
type decryptingResource struct {
	opaqueResource
	key         ContentKey
	maxInflated int64 // limit of whole reads, see inflationLimit
	enc         manifest.Encryption
	length      lengthCache // of the plaintext
}

func (r *decryptingResource) compressed() bool {
	return r.enc.Compression == CompressionDeflate
}

// Length implements Resource
//
// The length of an uncompressed resource is computed from its padding, rather
// than taken from its declared original length, which may be wrong: range
// reads rely on it to locate the ciphertext. It can't exceed the declared
// original length, though, as whole reads can't. A deflated resource has to
// be inflated to learn its length, so its declared original length is
// trusted when there is one; reads never return more bytes than it says.
func (r *decryptingResource) Length(ctx context.Context) (int64, *fetcher.ResourceError) {
	if length, ok := r.length.get(); ok {
		return length, nil
	}
	switch {
	case !r.compressed():
		length, rerr := r.plaintextLengthFromPadding(ctx)
		if rerr != nil {
			return 0, rerr
		}
		if declared := r.enc.OriginalLength; declared > 0 && length > declared {
			return 0, fetcher.Other(errors.Wrapf(ErrInvalidCiphertext, "content is longer than its declared original length (%d > %d bytes)", length, declared))
		}
		return r.length.set(length), nil
	case r.enc.OriginalLength > 0:
		return r.length.set(r.enc.OriginalLength), nil
	default:
		// Counted while streaming, in constant memory.
		length, rerr := r.Stream(ctx, io.Discard, 0, 0)
		if rerr != nil {
			return 0, rerr
		}
		return r.length.set(length), nil
	}
}

// plaintextLengthFromPadding computes the plaintext length of an uncompressed
// resource by decrypting its last block, which holds the padding length.
func (r *decryptingResource) plaintextLengthFromPadding(ctx context.Context) (int64, *fetcher.ResourceError) {
	length, rerr := r.Res.Length(ctx)
	if rerr != nil {
		return 0, rerr
	}
	if length < 2*blockSize || length%blockSize != 0 {
		return 0, fetcher.Other(ErrInvalidCiphertext)
	}
	tail, rerr := r.Res.Read(ctx, length-2*blockSize, length-1)
	if rerr != nil {
		return 0, rerr
	}
	if len(tail) != 2*blockSize {
		return 0, fetcher.Other(ErrInvalidCiphertext)
	}
	block, err := aes.NewCipher(r.key)
	if err != nil {
		return 0, fetcher.Other(err)
	}
	last := make([]byte, blockSize)
	cipher.NewCBCDecrypter(block, tail[:blockSize]).CryptBlocks(last, tail[blockSize:])
	pad := int64(last[blockSize-1])
	if pad < 1 || pad > blockSize {
		return 0, fetcher.Other(ErrInvalidPadding)
	}
	return length - blockSize - pad, nil
}

// Read implements Resource
//
// Deflated resources are inflated from the start, even to read a range of
// them, and their declared original length is untrusted: reading them
// fails with [ErrResourceTooLarge] beyond the maximum inflated length.
// Ranges of uncompressed resources take the memory of the range, bounded by
// their ciphertext, and only the ciphertext blocks they span are read.
func (r *decryptingResource) Read(ctx context.Context, start int64, end int64) ([]byte, *fetcher.ResourceError) {
	start = max(start, 0)
	if (start == 0 && end == 0) || r.compressed() {
		return r.readLimited(ctx, start, end)
	}

	length, rerr := r.Length(ctx)
	if rerr != nil {
		return nil, rerr
	}
	from, to, rerr := resolveRange(start, end, length)
	if rerr != nil {
		return nil, rerr
	}
	if from == to {
		return []byte{}, nil
	}

	// Plaintext block i is at ciphertext offset (i+1)*blockSize, right after
	// the ciphertext block serving as its IV.
	firstBlock := from / blockSize
	lastBlock := (to - 1) / blockSize
	ciphertext, rerr := r.Res.Read(ctx, firstBlock*blockSize, (lastBlock+2)*blockSize-1)
	if rerr != nil {
		return nil, rerr
	}
	if int64(len(ciphertext)) != (lastBlock-firstBlock+2)*blockSize {
		return nil, fetcher.Other(ErrInvalidCiphertext)
	}
	block, err := aes.NewCipher(r.key)
	if err != nil {
		return nil, fetcher.Other(err)
	}
	plain := make([]byte, len(ciphertext)-blockSize)
	cipher.NewCBCDecrypter(block, ciphertext[:blockSize]).CryptBlocks(plain, ciphertext[blockSize:])
	offset := firstBlock * blockSize
	return plain[from-offset : to-offset], nil
}

// readLimited reads the inclusive range [start, end] of the plaintext, or
// all of it when start == end == 0, by streaming it into memory. The
// declared original length is not used to preallocate the buffer: it is
// untrusted. Decryption fails when the content turns out to be longer than
// declared, and reading a deflated resource fails with [ErrResourceTooLarge]
// beyond the maximum inflated length.
func (r *decryptingResource) readLimited(ctx context.Context, start, end int64) ([]byte, *fetcher.ResourceError) {
	limit := int64(math.MaxInt64)
	if r.compressed() {
		if l := inflationLimit(r.maxInflated); l > 0 {
			limit = l
		}
	}
	w := &extensions.LimitedBuffer{Limit: limit}
	_, rerr := r.Stream(ctx, w, start, end)
	if w.Exceeded {
		return nil, fetcher.OutOfMemory(errors.Wrapf(ErrResourceTooLarge, "%s inflates beyond %s", r.Link().Href, extensions.FormatSize(limit)))
	}
	if rerr != nil {
		return nil, rerr
	}
	if w.Bytes() == nil {
		return []byte{}, nil
	}
	return w.Bytes(), nil
}

// Stream implements Resource
//
// Ranges are written as they are decrypted, and never held in memory.
func (r *decryptingResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *fetcher.ResourceError) {
	start = max(start, 0)
	if start == 0 && end == 0 {
		return streamThrough(ctx, r.Res, func(src io.Reader) (int64, error) {
			return DecryptContent(r.key, src, w, r.enc)
		})
	}
	if r.compressed() {
		// Inflated from the start, writing only the range.
		return streamRange(w, start, end, func(w io.Writer) (int64, *fetcher.ResourceError) {
			return r.Stream(ctx, w, 0, 0)
		})
	}
	return r.streamBlocks(ctx, w, start, end)
}

// streamBlocks writes to w the inclusive range [start, end] of the plaintext
// of an uncompressed resource, decrypting the ciphertext blocks it spans
// chunk by chunk.
func (r *decryptingResource) streamBlocks(ctx context.Context, w io.Writer, start, end int64) (int64, *fetcher.ResourceError) {
	length, rerr := r.Length(ctx)
	if rerr != nil {
		return 0, rerr
	}
	from, to, rerr := resolveRange(start, end, length)
	if rerr != nil {
		return 0, rerr
	}
	if from == to {
		return 0, nil
	}
	block, err := aes.NewCipher(r.key)
	if err != nil {
		return 0, fetcher.Other(err)
	}

	// As in Read, the ciphertext block preceding the range is its IV.
	firstBlock := from / blockSize
	lastBlock := (to - 1) / blockSize
	offset := firstBlock * blockSize
	out := &rangeForwarder{w: w, from: from - offset, to: to - offset}
	_, rerr = streamRangeThrough(ctx, r.Res, offset, (lastBlock+2)*blockSize-1, func(src io.Reader) (int64, error) {
		iv := make([]byte, blockSize)
		if _, err := io.ReadFull(src, iv); err != nil {
			return 0, ciphertextError(err)
		}
		dec := cipher.NewCBCDecrypter(block, iv)
		buf := make([]byte, chunkSize)
		for remaining := (lastBlock - firstBlock + 1) * blockSize; remaining > 0; {
			n, err := io.ReadFull(src, buf[:min(int64(len(buf)), remaining)])
			if err != nil {
				return 0, ciphertextError(err)
			}
			dec.CryptBlocks(buf[:n], buf[:n])
			if _, err := out.Write(buf[:n]); err != nil {
				return 0, err
			}
			remaining -= int64(n)
		}
		return 0, nil
	})
	if rerr != nil && !errors.Is(rerr.Cause, errRangeRead) {
		return out.written, rerr
	}
	return out.written, nil
}
