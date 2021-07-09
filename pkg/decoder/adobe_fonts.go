package decoder

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"unicode"

	"github.com/readium/r2-streamer-go/pkg/pub"
)

func init() {
	decoderList = append(decoderList, List{decoderAlgorithm: "http://ns.adobe.com/pdf/enc#RC", decoder: DecodeAdobeFont})
}

// DecodeAdobeFont decode obfuscate fonts using idpf spec http://www.idpf.org/epub/20/spec/FontManglingSpec.html
func DecodeAdobeFont(publication *pub.Publication, link pub.Link, reader io.ReadSeeker) (io.ReadSeeker, error) {
	var count int

	key := getAdobeHashKey(publication)
	if string(key) == "" {
		return nil, errors.New("can't find hash key")
	}

	buff, _ := ioutil.ReadAll(reader)
	if len(buff) > 1024 {
		count = 1024
	} else {
		count = len(buff)
	}

	j := 0
	for i := 0; i < count; i++ {
		buff[i] = buff[i] ^ key[j]

		j++
		if j == 16 {
			j = 0
		}
	}
	readerSeeker := bytes.NewReader(buff)
	return readerSeeker, nil
}

func getAdobeHashKey(publication *pub.Publication) []byte {
	var stringKey []rune
	var key []byte

	id := strings.Replace(publication.Metadata.Identifier, "urn:uuid:", "", -1)
	id = strings.Replace(id, "-", "", -1)
	for _, c := range id {
		if !unicode.IsSpace(c) {
			stringKey = append(stringKey, c)
		}
	}

	for i := 0; i < 16; i++ {
		byteHex := stringKey[i*2 : i*2+2]
		byteNumer, _ := strconv.ParseInt(string(byteHex), 16, 32)
		key = append(key, byte(byteNumer))
	}

	return key
}
