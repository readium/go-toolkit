package epub

import (
	"bytes"
	"context"
	"io"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/obfuscation"
	"github.com/readium/go-toolkit/pkg/protection"
)

var algorithm2length = map[string]int64{
	protection.AlgorithmIDPFObfuscation:  1040,
	protection.AlgorithmAdobeObfuscation: 1024,
}

type Deobfuscator struct {
	identifier string
}

func NewDeobfuscator(identifier string) Deobfuscator {
	return Deobfuscator{identifier: identifier}
}

func (d Deobfuscator) Transform(resource fetcher.Resource) fetcher.Resource {
	return DeobfuscatingResource{ProxyResource: fetcher.ProxyResource{Res: resource}, identifier: d.identifier}
}

type DeobfuscatingResource struct {
	fetcher.ProxyResource
	identifier string
}

func (d DeobfuscatingResource) obfuscation() (string, int64) {
	algorithm := ""
	penc := d.Res.Link().Properties.Encryption()
	if penc != nil {
		algorithm = penc.Algorithm
	}

	v, ok := algorithm2length[algorithm]
	if !ok {
		return algorithm, 0
	}
	return algorithm, v
}

func (d DeobfuscatingResource) Read(ctx context.Context, start, end int64) ([]byte, *fetcher.ResourceError) {
	algorithm, prefix := d.obfuscation()
	start = max(start, 0)
	if start >= prefix {
		return d.Res.Read(ctx, start, end)
	}
	key := obfuscation.Key(algorithm, d.identifier)
	if len(key) == 0 {
		return nil, fetcher.Other(errors.New("error deriving font deobfuscation key"))
	}
	data, err := d.Res.Read(ctx, start, end)
	if err != nil {
		return nil, err
	}
	data = bytes.Clone(data)
	deobfuscateFont(data, start, key, prefix)
	return data, nil
}

func (d DeobfuscatingResource) Stream(ctx context.Context, w io.Writer, start, end int64) (int64, *fetcher.ResourceError) {
	algorithm, prefix := d.obfuscation()
	start = max(start, 0)
	if start >= prefix {
		return d.Res.Stream(ctx, w, start, end)
	}
	key := obfuscation.Key(algorithm, d.identifier)
	if len(key) == 0 {
		return 0, fetcher.Other(errors.New("error deriving font deobfuscation key"))
	}
	return d.Res.Stream(ctx, &fontWriter{out: w, offset: start, prefix: prefix, key: key}, start, end)
}

// fontWriter transforms only the prefix. A private buffer keeps the input
// supplied by the streaming resource intact.
type fontWriter struct {
	out            io.Writer
	offset, prefix int64
	key            []byte
}

func (w *fontWriter) Write(p []byte) (int, error) {
	var written int
	if w.offset < w.prefix {
		n := min(int64(len(p)), w.prefix-w.offset)
		part := bytes.Clone(p[:n])
		obfuscation.XOR(part, w.key, w.offset)
		count, err := w.out.Write(part)
		w.offset += int64(count)
		if err != nil {
			return count, err
		}
		if count != len(part) {
			return count, io.ErrShortWrite
		}
		written = count
		p = p[count:]
	}
	if len(p) == 0 {
		return written, nil
	}
	n, err := w.out.Write(p)
	return written + n, err
}

// File implements Resource. The file of an obfuscated font holds its
// obfuscated bytes, so it is not exposed: a consumer serving it directly
// would bypass the deobfuscation.
func (d DeobfuscatingResource) File() string {
	if _, v := d.obfuscation(); v > 0 {
		return ""
	}
	return d.ProxyResource.File()
}

// CompressedAs implements CompressedResource
func (d DeobfuscatingResource) CompressedAs(compressionMethod archive.CompressionMethod) bool {
	_, v := d.obfuscation()
	if v > 0 {
		return false
	}

	return d.ProxyResource.CompressedAs(compressionMethod)
}

// CompressedLength implements CompressedResource
func (d DeobfuscatingResource) CompressedLength(ctx context.Context) int64 {
	_, v := d.obfuscation()
	if v > 0 {
		return -1
	}

	return d.ProxyResource.CompressedLength(ctx)
}

func (d DeobfuscatingResource) CRC32Checksum(ctx context.Context) *uint32 {
	_, v := d.obfuscation()
	if v > 0 {
		return nil
	}

	return d.ProxyResource.CRC32Checksum(ctx)
}

// StreamCompressed implements CompressedResource
func (d DeobfuscatingResource) StreamCompressed(ctx context.Context, w io.Writer) (int64, *fetcher.ResourceError) {
	_, v := d.obfuscation()
	if v > 0 {
		return 0, fetcher.Other(errors.New("cannot stream compressed resource when obfuscated"))
	}

	return d.ProxyResource.StreamCompressed(ctx, w)
}

// StreamCompressedGzip implements CompressedResource
func (d DeobfuscatingResource) StreamCompressedGzip(ctx context.Context, w io.Writer) (int64, *fetcher.ResourceError) {
	_, v := d.obfuscation()
	if v > 0 {
		return 0, fetcher.Other(errors.New("cannot stream compressed resource when obfuscated"))
	}

	return d.ProxyResource.StreamCompressedGzip(ctx, w)
}

// ReadCompressed implements CompressedResource
func (d DeobfuscatingResource) ReadCompressed(ctx context.Context) ([]byte, *fetcher.ResourceError) {
	_, v := d.obfuscation()
	if v > 0 {
		return nil, fetcher.Other(errors.New("cannot read compressed resource when obfuscated"))
	}

	return d.ProxyResource.ReadCompressed(ctx)
}

// ReadCompressedGzip implements CompressedResource
func (d DeobfuscatingResource) ReadCompressedGzip(ctx context.Context) ([]byte, *fetcher.ResourceError) {
	_, v := d.obfuscation()
	if v > 0 {
		return nil, fetcher.Other(errors.New("cannot read compressed resource when obfuscated"))
	}

	return d.ProxyResource.ReadCompressedGzip(ctx)
}

func deobfuscateFont(data []byte, start int64, obfuscationKey []byte, obfuscationLength int64) {
	if start >= obfuscationLength || len(obfuscationKey) == 0 {
		return
	}
	max := obfuscationLength - start
	dlen := int64(len(data))
	if max > dlen {
		max = dlen
	}
	obfuscation.XOR(data[:max], obfuscationKey, start)
}
