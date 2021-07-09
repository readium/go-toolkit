package parser

import (
	"errors"
	"path/filepath"

	"github.com/readium/r2-streamer-go/pkg/pub"
)

// List TODO add doc
type List struct {
	fileExt  string
	parser   (func(filePath string) (pub.Publication, error))
	callback (func(*pub.Publication))
}

var parserList []List

// Parse TODO add doc
func Parse(filePath string) (pub.Publication, error) {

	fileExt := filepath.Ext(filePath)
	if fileExt == "" {
		fileExt = ".epub"
	}
	for _, parserFunc := range parserList {
		if fileExt == "."+parserFunc.fileExt {
			return parserFunc.parser(filePath)
		}
	}

	return pub.Publication{}, errors.New("can't find parser")
}

// CallbackParse call function too parse element that can be encrypted or obfuscated
func CallbackParse(publication *pub.Publication) {
	var typePublication string

	for _, key := range publication.Internal {
		if key.Name == "type" {
			typePublication = key.Value.(string)
		}
	}

	for _, parserFunc := range parserList {
		if typePublication == parserFunc.fileExt {
			parserFunc.callback(publication)
		}
	}
}
