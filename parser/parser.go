package parser

import (
	"fmt"
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
	fmt.Println(fileExt)
	for _, parserFunc := range parserList {
		if fileExt == "."+parserFunc.fileExt {
			fmt.Println("Parse " + parserFunc.fileExt)
			return parserFunc.parser(filePath, selfURL)
		}
	}

	return models.Publication{}
}
