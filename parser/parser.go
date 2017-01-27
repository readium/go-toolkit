package parser

import (
	"errors"
	"path/filepath"

	"github.com/feedbooks/r2-streamer-go/models"
)

// List TODO add doc
type List struct {
	fileExt string
	parser  (func(filePath string) (models.Publication, error))
}

var parserList []List

// Parse TODO add doc
func Parse(filePath string) (models.Publication, error) {

	fileExt := filepath.Ext(filePath)
	for _, parserFunc := range parserList {
		if fileExt == "."+parserFunc.fileExt {
			return parserFunc.parser(filePath)
		}
	}

	return models.Publication{}, errors.New("can't find parser")
}
