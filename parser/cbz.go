package parser

import (
	"archive/zip"
	"fmt"
	"path/filepath"

	"github.com/feedbooks/webpub-streamer/models"
)

func init() {
	parserList = append(parserList, List{fileExt: "cbz", parser: CbzParser})
}

// CbzParser TODO add doc
func CbzParser(filePath string, selfURL string) models.Publication {
	var publication models.Publication

	publication.Metadata.Title = filePath
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		fmt.Println("failed to open zip " + filePath)
		fmt.Println(err)
		return publication
	}

	publication.Internal = append(publication.Internal, models.Internal{Name: "type", Value: "cbz"})
	publication.Internal = append(publication.Internal, models.Internal{Name: "cbz", Value: zipReader})

	for _, f := range zipReader.File {
		linkItem := models.Link{}
		linkItem.TypeLink = getMediaTypeByName(f.Name)
		linkItem.Href = f.Name
		publication.Spine = append(publication.Spine, linkItem)
	}

	return publication
}

func getMediaTypeByName(filePath string) string {
	ext := filepath.Ext(filePath)

	if ext == ".jpg" {
		return "image/jpeg"
	}

	return ""
}
