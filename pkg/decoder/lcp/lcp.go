package lcp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"

	"github.com/readium/r2-streamer-go/pkg/pub"
)

// DecryptData decrypt data from the stream
func DecryptData(publication *pub.Manifest, link pub.Link, reader io.ReadSeeker) (bytes.Buffer, error) {
	var cipherRes bytes.Buffer
	var cipherContentKey bytes.Buffer

	contentKey := publication.GetBytesFromInternal("lcp_content_key")
	hashPassphrase := publication.GetBytesFromInternal("lcp_hash_passphrase")

	if link.Properties.Encryption.Algorithm == "http://www.w3.org/2001/04/xmlenc#aes256-cbc" {

		contentKeyReader := bytes.NewBuffer(contentKey)
		err := decryptAESCBC(hashPassphrase, contentKeyReader, &cipherContentKey)
		if err != nil {
			fmt.Println(err)
			return bytes.Buffer{}, err
		}

		errRes := decryptAESCBC(cipherContentKey.Bytes(), reader, &cipherRes)
		if errRes != nil {
			fmt.Println(err)
			return bytes.Buffer{}, errRes
		}

		return cipherRes, nil
	}

	return bytes.Buffer{}, errors.New("can't find algorithm")
}

func decryptAESCBC(key []byte, r io.Reader, w io.Writer) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	io.Copy(&buffer, r)

	buf := buffer.Bytes()
	iv := buf[:aes.BlockSize]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(buf[aes.BlockSize:], buf[aes.BlockSize:])

	padding := buf[len(buf)-1] // padding length valid for both PKCS#7 and W3C schemes
	w.Write(buf[aes.BlockSize : len(buf)-int(padding)])

	return nil
}
