package epub

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"

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
	data       []byte
}

func (d DeobfuscatingResource) Read(start, end int64) ([]byte, *fetcher.ResourceError) {
	algorithm := ""
	penc := d.Res.Link().Properties.Encryption()
	if penc != nil {
		algorithm = penc.Algorithm
	}

	for k, v := range algorithm2length {
		if k == algorithm {
			data, err := d.ProxyResource.Read(start, end)
			if err != nil {
				return nil, err
			}
			d.data = data
			var obfuscationKey []byte
			switch algorithm {
			case "http://ns.adobe.com/pdf/enc#RC":
				obfuscationKey = d.getHashKeyAdobe()
			default:
				shasum := sha1.Sum([]byte(d.identifier))
				obfuscationKey = shasum[:]
			}
			d.deobfuscate(start, obfuscationKey, v)
			return d.data, nil
		}
	}

	// Algorithm not in known, so skip deobfuscation
	return d.ProxyResource.Read(start, end)
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

func (d DeobfuscatingResource) deobfuscate(start int64, obfuscationKey []byte, obfuscationLength int64) {
	if start >= obfuscationLength {
		return
	}
	max := obfuscationLength - start
	dlen := int64(len(d.data))
	if max > dlen {
		max = dlen
	}
	olen := int64(len(obfuscationKey))
	for i := int64(0); i < max; i++ {
		d.data[i] ^= obfuscationKey[(start+i)%olen]
	}
}
