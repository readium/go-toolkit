package decoder

import (
	"errors"
	"io"

	"github.com/readium/r2-streamer-go/models"
)

// List TODO add doc
type List struct {
	decoderAlgorithm string
	decoder          (func(models.Publication, models.Link, io.ReadSeeker) (io.ReadSeeker, error))
}

var decoderList []List

// Decode decode the ressource
func Decode(publication models.Publication, link models.Link, reader io.ReadSeeker) (io.ReadSeeker, error) {

	for _, decoderFunc := range decoderList {
		if link.CryptAlgorithm == decoderFunc.decoderAlgorithm {
			return decoderFunc.decoder(publication, link, reader)
		}
	}

	return nil, errors.New("can't find fetcher")
}

// NeedToDecode check if there a decoder for this resource
func NeedToDecode(publication models.Publication, link models.Link) bool {
	for _, decoderFunc := range decoderList {
		if link.CryptAlgorithm == decoderFunc.decoderAlgorithm {
			return true
		}
	}

	return false
}
