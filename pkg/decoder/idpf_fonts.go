package decoder

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"io"
	"io/ioutil"
	"unicode"

	"github.com/readium/r2-streamer-go/pkg/pub"
)

func init() {
	decoderList = append(decoderList, List{decoderAlgorithm: "http://www.idpf.org/2008/embedding", decoder: DecodeIdpfFont})
}

// DecodeIdpfFont decode obfuscate fonts using idpf spec http://www.idpf.org/epub/20/spec/FontManglingSpec.html
func DecodeIdpfFont(publication *pub.Manifest, link pub.Link, reader io.ReadSeeker) (io.ReadSeeker, error) {
	var count int

	key := getHashKey(publication)
	if string(key) == "" {
		return nil, errors.New("can't find hash key")
	}

	buff, _ := ioutil.ReadAll(reader)
	if len(buff) > 1040 {
		count = 1040
	} else {
		count = len(buff)
	}

	j := 0
	for i := 0; i < count; i++ {
		buff[i] = buff[i] ^ key[j]

		j++
		if j == 20 {
			j = 0
		}
	}
	readerSeeker := bytes.NewReader(buff)
	return readerSeeker, nil
}

func getHashKey(publication *pub.Manifest) []byte {
	var stringKey []rune

	for _, c := range publication.Metadata.Identifier {
		if !unicode.IsSpace(c) {
			stringKey = append(stringKey, c)
		}
	}

	h := sha1.New()
	io.WriteString(h, string(stringKey))

	return h.Sum(nil)
}
