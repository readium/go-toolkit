package decoder

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/readium/r2-streamer-go/models"
)

func init() {
	decoderList = append(decoderList, List{decoderAlgorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc", decoderScheme: "http://readium.org/2014/01/lcp", decoder: DecodeLCP})
}

// DecodeLCP decode lcp encrypted file
func DecodeLCP(publication models.Publication, link models.Link, reader io.ReadSeeker) (io.ReadSeeker, error) {
	var passwd string
	var contentKey []byte
	var keyCheck []byte
	var lcpID string
	var cipher bytes.Buffer
	var cipherRes bytes.Buffer
	var cipherContentKey bytes.Buffer

	for _, data := range publication.Internal {
		if data.Name == "lcp_content_key" {
			contentKey = data.Value.([]byte)
		}
		if data.Name == "lcp_user_key_check" {
			keyCheck = data.Value.([]byte)
		}
		if data.Name == "lcp_id" {
			lcpID = data.Value.(string)
		}
		if data.Name == "lcp_passphrase" {
			passwd = data.Value.(string)
		}
	}

	if passwd == "" {
		passwd = "test"
		// return nil, errors.New("password")
	}

	hashPasswd := sha256.Sum256([]byte(passwd))

	keyCheckReader := bytes.NewBuffer(keyCheck)
	err := decryptAESCBC(hashPasswd[:], keyCheckReader, &cipher)
	if err != nil {
		fmt.Println(err)
	}

	if cipher.String() != lcpID {
		return nil, errors.New("password")
	}

	if link.Properties.Encrypted.Algorithm == "http://www.w3.org/2001/04/xmlenc#aes256-cbc" {

		contentKeyReader := bytes.NewBuffer(contentKey)
		err := decryptAESCBC(hashPasswd[:], contentKeyReader, &cipherContentKey)
		if err != nil {
			fmt.Println(err)
		}

		errRes := decryptAESCBC(cipherContentKey.Bytes(), reader, &cipherRes)
		if errRes != nil {
			fmt.Println(err)
		}

		if link.Properties.Encrypted.Package == "deflate" {
			flateReader := flate.NewReader(bytes.NewReader(cipherRes.Bytes()))
			buff, _ := ioutil.ReadAll(flateReader)
			flateReader.Close()
			readerSeeker := bytes.NewReader(buff)

			return readerSeeker, nil
		}

		readerSeeker := bytes.NewReader(cipherRes.Bytes())
		return readerSeeker, nil
	}

	return nil, errors.New("can't find algorithm")
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
