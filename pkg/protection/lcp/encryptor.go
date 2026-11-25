package lcp

import (
	"bytes"
	"context"
	"io"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// encryptingResource serves the LCP ciphertext of a cleartext resource.
//
// The initialization vector and padding bytes are drawn once, when the
// resource is created, so that every read returns the same ciphertext: a
// consumer may read the length, then ranges, then stream the resource, and
// must see consistent bytes.
type encryptingResource struct {
	opaqueResource
	declaration *manifest.Encryption
	key         ContentKey
	compress    bool
	random      []byte      // IV + padding bytes, consumed by each encryption
	length      lengthCache // of the ciphertext
}

// randomBytesPerEncryption is the number of random bytes an encryption
// consumes at most: the IV and up to a block of padding, minus the length
// byte.
const randomBytesPerEncryption = 2*blockSize - 1

func newEncryptingResource(res fetcher.Resource, key ContentKey, compress bool, random io.Reader) (*encryptingResource, error) {
	r := make([]byte, randomBytesPerEncryption)
	if _, err := io.ReadFull(random, r); err != nil {
		return nil, errors.Wrap(err, "lcp: failed generating initialization vector")
	}
	return &encryptingResource{
		opaqueResource: opaqueResource{fetcher.ProxyResource{Res: res}},
		key:            key,
		compress:       compress,
		random:         r,
	}, nil
}

// Link describes the content after encryption.
func (r *encryptingResource) Link() manifest.Link {
	link := r.Res.Link()
	if r.declaration != nil {
		link.Properties = withEncryption(link.Properties, r.declaration)
	}
	return link
}

func (r *encryptingResource) encrypt(src io.Reader, dst io.Writer) (int64, error) {
	cw := &countingWriter{w: dst}
	_, err := EncryptContent(r.key, src, cw, r.compress, WithRandom(bytes.NewReader(r.random)))
	return cw.n, err
}

// Length implements Resource
func (r *encryptingResource) Length(ctx context.Context) (int64, *fetcher.ResourceError) {
	if length, ok := r.length.get(); ok {
		return length, nil
	}
	var plaintextLength int64
	if r.compress {
		// The compressed length is only known after compressing the content.
		n, rerr := streamThrough(ctx, r.Res, compressedLength)
		if rerr != nil {
			return 0, rerr
		}
		plaintextLength = n
	} else {
		n, rerr := r.Res.Length(ctx)
		if rerr != nil {
			return 0, rerr
		}
		plaintextLength = n
	}
	length := EncryptedLength(plaintextLength)
	if length < 0 {
		return 0, fetcher.Other(errors.New("lcp: unrepresentable ciphertext length"))
	}
	return r.length.set(length), nil
}

// Read implements Resource
func (r *encryptingResource) Read(ctx context.Context, start int64, end int64) ([]byte, *fetcher.ResourceError) {
	var buf bytes.Buffer
	if _, rerr := r.Stream(ctx, &buf, start, end); rerr != nil {
		return nil, rerr
	}
	if buf.Len() == 0 {
		return []byte{}, nil
	}
	return buf.Bytes(), nil
}

// Stream implements Resource
func (r *encryptingResource) Stream(ctx context.Context, w io.Writer, start int64, end int64) (int64, *fetcher.ResourceError) {
	start = max(start, 0)
	if start == 0 && end == 0 {
		return streamThrough(ctx, r.Res, func(src io.Reader) (int64, error) {
			return r.encrypt(src, w)
		})
	}
	// CBC encryption is sequential: any range requires encrypting from the
	// start of the resource, but only the range is written.
	return streamRange(w, start, end, func(w io.Writer) (int64, *fetcher.ResourceError) {
		return r.Stream(ctx, w, 0, 0)
	})
}
