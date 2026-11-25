package protection

import (
	"bytes"
	"encoding/xml"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// EncryptionEntry describes one <EncryptedData> element of an EPUB
// META-INF/encryption.xml document.
type EncryptionEntry struct {
	// Href is the resource path relative to the container root, in its
	// percent-encoded form, as it appears in the CipherReference URI.
	Href string
	// Encryption describes how the resource is encrypted.
	Encryption manifest.Encryption
}

// MarshalEPUBEncryption serializes an EPUB META-INF/encryption.xml document
// declaring the given entries. It returns nil when there is nothing to
// declare.
//
// Entries whose scheme is [SchemeLCP] point at the LCP license's content key
// through a ds:RetrievalMethod. A Compression property is written when the
// entry declares a compression or an original length. EPUB only defines
// Deflate compression: any other one, such as Zstandard, is an error. So is a
// document larger than [MaxMetadataSize], which readers would refuse.
//
// This is the counterpart of the EPUB parser's ParseEncryption, and
// round-trips through the same map of [manifest.Encryption].
func MarshalEPUBEncryption(entries []EncryptionEntry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	doc := encEncryption{
		Xmlns:     namespaceOPC,
		XmlnsEnc:  namespaceENC,
		XmlnsDS:   namespaceSIG,
		XmlnsComp: namespaceCOMP,
	}
	for _, entry := range entries {
		data := encEncryptedData{
			Method:     encMethod{Algorithm: entry.Encryption.Algorithm},
			CipherData: encCipherData{CipherReference: encCipherReference{URI: entry.Href}},
		}
		if entry.Encryption.Scheme == SchemeLCP {
			data.KeyInfo = &encKeyInfo{RetrievalMethod: &encRetrievalMethod{
				URI:  LCPContentKeyRetrievalURI,
				Type: LCPContentKeyRetrievalType,
			}}
		}
		if entry.Encryption.Compression != "" || entry.Encryption.OriginalLength > 0 {
			method, err := compressionMethod(entry.Encryption.Compression)
			if err != nil {
				return nil, errors.Wrap(err, entry.Href)
			}
			data.Properties = &encProperties{Properties: []encProperty{{
				Compression: &encCompression{
					Method:         method,
					OriginalLength: entry.Encryption.OriginalLength,
				},
			}}}
		}
		doc.Data = append(doc.Data, data)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	if buf.Len() > MaxMetadataSize {
		return nil, errors.Errorf("encryption.xml would be larger than %s, the size limit of readers", extensions.FormatSize(MaxMetadataSize))
	}
	return buf.Bytes(), nil
}

// compressionMethod returns the Method attribute of the Compression property
// declaring a [manifest.Encryption] compression. EPUB only defines "0" (no
// compression) and "8" (Deflate). Writing "0" for another compression would
// have reading systems serve the resource still compressed.
func compressionMethod(compression string) (string, error) {
	switch compression {
	case "", CompressionNone:
		return "0", nil
	case CompressionDeflate:
		return "8", nil
	}
	return "", errors.Errorf("compression %q can't be declared in encryption.xml, which only supports deflate", compression)
}

// encryption.xml write-only model. Elements carry explicit namespace prefixes
// declared on the root, matching the shape produced by common EPUB tooling.

type encEncryption struct {
	XMLName   xml.Name           `xml:"encryption"`
	Xmlns     string             `xml:"xmlns,attr"`
	XmlnsEnc  string             `xml:"xmlns:enc,attr"`
	XmlnsDS   string             `xml:"xmlns:ds,attr"`
	XmlnsComp string             `xml:"xmlns:comp,attr"`
	Data      []encEncryptedData `xml:"enc:EncryptedData"`
}

type encEncryptedData struct {
	Method     encMethod      `xml:"enc:EncryptionMethod"`
	KeyInfo    *encKeyInfo    `xml:"ds:KeyInfo,omitempty"`
	CipherData encCipherData  `xml:"enc:CipherData"`
	Properties *encProperties `xml:"enc:EncryptionProperties,omitempty"`
}

type encMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type encKeyInfo struct {
	RetrievalMethod *encRetrievalMethod `xml:"ds:RetrievalMethod,omitempty"`
}

type encRetrievalMethod struct {
	URI  string `xml:"URI,attr"`
	Type string `xml:"Type,attr"`
}

type encCipherData struct {
	CipherReference encCipherReference `xml:"enc:CipherReference"`
}

type encCipherReference struct {
	URI string `xml:"URI,attr"`
}

type encProperties struct {
	Properties []encProperty `xml:"enc:EncryptionProperty"`
}

type encProperty struct {
	Compression *encCompression `xml:"comp:Compression,omitempty"`
}

type encCompression struct {
	Method         string `xml:"Method,attr"`
	OriginalLength int64  `xml:"OriginalLength,attr"`
}
