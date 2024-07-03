package epub

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
)

var algorithm2length = map[string]int64{
	"http://www.idpf.org/2008/embedding": 1040,
	"http://ns.adobe.com/pdf/enc#RC":     1024,
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

func (d DeobfuscatingResource) Read(start, end int64) ([]byte, *fetcher.ResourceError) {
	algorithm, v := d.obfuscation()
	if v > 0 {
		data, err := d.ProxyResource.Read(start, end)
		if err != nil {
			return nil, err
		}
		var obfuscationKey []byte
		switch algorithm {
		case "http://ns.adobe.com/pdf/enc#RC":
			obfuscationKey = d.getHashKeyAdobe()
		default:
			shasum := sha1.Sum([]byte(d.identifier))
			obfuscationKey = shasum[:]
		}
		deobfuscateFont(data, start, obfuscationKey, v)
		return data, nil
	}

	// Algorithm not in known, so skip deobfuscation
	return d.ProxyResource.Read(start, end)
}

func (d DeobfuscatingResource) Stream(w io.Writer, start int64, end int64) (int64, *fetcher.ResourceError) {
	algorithm, v := d.obfuscation()
	if v > 0 {
		if start >= v {
			// We're past the obfuscated part, just proxy it
			return d.ProxyResource.Stream(w, start, end)
		}

		// Create a pipe to proxy the stream for deobfuscation
		pr, pw := io.Pipe()

		// Start piping the resource's stream in a goroutine
		go func() {
			_, err := d.ProxyResource.Stream(pw, start, end)
			if err != nil {
				pw.CloseWithError(err)
			} else {
				pw.Close()
			}
		}()

		// First, we just read the obfuscated portion (1040 or 1024 first bytes)
		obfuscatedPortion := make([]byte, v)
		on, err := pr.Read(obfuscatedPortion)
		if err != nil && err != io.EOF {
			if fre, ok := err.(*fetcher.ResourceError); ok {
				return 0, fre
			} else {
				return 0, fetcher.Other(errors.Wrap(err, "error reading obfuscated portion of font"))
			}
		}

		// Handle filesize <= the obfuscated portion's length or the requested length
		atEnd := false
		if on < len(obfuscatedPortion) || (end != 0 && end <= start+int64(on)) {
			obfuscatedPortion = obfuscatedPortion[:on]
			atEnd = true
			pr.Close()
		}

		// Deobfuscate just the obfuscated portion
		var obfuscationKey []byte
		switch algorithm {
		case "http://ns.adobe.com/pdf/enc#RC":
			obfuscationKey = d.getHashKeyAdobe()
		default:
			shasum := sha1.Sum([]byte(d.identifier))
			obfuscationKey = shasum[:]
		}
		deobfuscateFont(obfuscatedPortion, start, obfuscationKey, v)

		defer pr.Close()

		// And write it to the stream
		_, err = w.Write(obfuscatedPortion)
		if err != nil {
			return 0, fetcher.Other(errors.Wrap(err, "error writing obfuscated portion of font"))
		}

		// The rest of the font is not obfuscated, so it's "copied" directly using a 32KB buffer
		var wn int64
		if !atEnd {
			wn, err = io.Copy(w, pr)
			if err != nil {
				return 0, fetcher.Other(errors.Wrap(err, "error writing unobfuscated portion of font"))
			}
		}
		return int64(on) + wn, nil
	}

	// Algorithm not in known, so skip deobfuscation
	return d.ProxyResource.Stream(w, start, end)
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
func (d DeobfuscatingResource) CompressedLength() int64 {
	_, v := d.obfuscation()
	if v > 0 {
		return -1
	}

	return d.ProxyResource.CompressedLength()
}

// StreamCompressed implements CompressedResource
func (d DeobfuscatingResource) StreamCompressed(w io.Writer) (int64, *fetcher.ResourceError) {
	_, v := d.obfuscation()
	if v > 0 {
		return 0, fetcher.Other(errors.New("cannot stream compressed resource when obfuscated"))
	}

	return d.ProxyResource.StreamCompressed(w)
}

func (d DeobfuscatingResource) getHashKeyAdobe() []byte {
	hexbytes, _ := hex.DecodeString(
		strings.Replace(
			strings.Replace(d.identifier, "urn:uuid:", "", -1),
			"-", "", -1,
		),
	)
	return hexbytes
}

func deobfuscateFont(data []byte, start int64, obfuscationKey []byte, obfuscationLength int64) {
	if start >= obfuscationLength {
		return
	}
	max := obfuscationLength - start
	dlen := int64(len(data))
	if max > dlen {
		max = dlen
	}
	olen := int64(len(obfuscationKey))
	for i := int64(0); i < max; i++ {
		data[i] ^= obfuscationKey[(start+i)%olen]
	}
}
