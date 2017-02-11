package decoder

import (
	"errors"
	"fmt"
	"io"

	"github.com/feedbooks/r2-streamer-go/models"
)

// List TODO add doc
type List struct {
	decoderAlgorithm string
	decoder          (func(models.Publication, models.Link, io.ReadSeeker) (io.ReadSeeker, error))
}

var decoderList []List

// Decode decode the ressource
func Decode(publication models.Publication, link models.Link, reader io.ReadSeeker) (io.ReadSeeker, error) {

	fmt.Println(link.CryptAlgorithm)
	for _, decoderFunc := range decoderList {
		if link.CryptAlgorithm == decoderFunc.decoderAlgorithm {
			return decoderFunc.decoder(publication, link, reader)
		}
	}

	return nil, errors.New("can't find fetcher")
}
