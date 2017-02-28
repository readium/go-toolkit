package epub

//Encryption encruption.xml
type Encryption struct {
	EncryptedData []EncryptedData `xml:"EncryptedData"`
}

type EncryptedData struct {
	EncryptionMethod     EncryptionMethod     `xml:"EncryptionMethod"`
	KeyInfo              KeyInfo              `xml:"KeyInfo"`
	CipherData           CipherData           `xml:"CipherData"`
	EncryptionProperties []EncryptionProperty `xml:"EncryptionProperties>EncryptionProperty"`
}

type EncryptionProperty struct {
	Compression Compression `xml:"Compression"`
}

type Compression struct {
	Method         string `xml:"Method,attr"`
	OriginalLength string `xml:"OriginalLength,attr"`
}

type EncryptionMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type KeyInfo struct {
	Resource string `xml:",chardata"`
}

type CipherData struct {
	CipherReference CipherReference `xml:"CipherReference"`
}

type CipherReference struct {
	URI string `xml:"URI,attr"`
}
