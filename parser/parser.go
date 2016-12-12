package parser

import (
	"path/filepath"

	"github.com/feedbooks/webpub-streamer/models"
)

// List TODO add doc
type List struct {
	fileExt string
	parser  (func(filePath string, selfURL string) models.Publication)
}

var parserList []List

// Parse TODO add doc
func Parse(filePath string, selfURL string) models.Publication {

	fileExt := filepath.Ext(filePath)
	for _, parserFunc := range parserList {
		if fileExt == "."+parserFunc.fileExt {
			return parserFunc.parser(filePath, selfURL)
		}
	}

	return models.Publication{}
}
