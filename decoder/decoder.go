package decoder

import (
	"errors"
	"io"

	"github.com/readium/r2-streamer-go/models"
)

// List TODO add doc
type List struct {
	decoderAlgorithm string
	decoderScheme    string // only for lcp or other encrypted resource
	decoder          (func(models.Publication, models.Link, io.ReadSeeker) (io.ReadSeeker, error))
}

var decoderList []List

// Decode decode the ressource
func Decode(publication models.Publication, link models.Link, reader io.ReadSeeker) (io.ReadSeeker, error) {

	for _, decoderFunc := range decoderList {
		if link.Properties != nil && link.Properties.Encrypted != nil && link.Properties.Encrypted.Algorithm == decoderFunc.decoderAlgorithm && decoderFunc.decoderScheme == link.Properties.Encrypted.Scheme {
			return decoderFunc.decoder(publication, link, reader)
		}
	}

	return nil, errors.New("can't find fetcher")
}

// NeedToDecode check if there a decoder for this resource
func NeedToDecode(publication models.Publication, link models.Link) bool {
	for _, decoderFunc := range decoderList {
		if link.Properties != nil && link.Properties.Encrypted != nil && link.Properties.Encrypted.Algorithm == decoderFunc.decoderAlgorithm && decoderFunc.decoderScheme == link.Properties.Encrypted.Scheme {
			return true
		}
	}

	return false
}
