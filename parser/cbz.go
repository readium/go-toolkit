package parser

import (
	"archive/zip"
	"errors"
	"path/filepath"
	"strings"

	"github.com/readium/r2-streamer-go/models"
)

func init() {
	parserList = append(parserList, List{fileExt: "cbz", parser: CbzParser})
}

// CbzParser TODO add doc
func CbzParser(filePath string) (models.Publication, error) {
	var publication models.Publication

	publication.Metadata.Title.SingleString = filePathToTitle(filePath)
	publication.Metadata.Identifier = filePath
	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return publication, errors.New("can't open or parse cbz file with err : " + err.Error())
	}

	publication.Internal = append(publication.Internal, models.Internal{Name: "type", Value: "cbz"})
	publication.Internal = append(publication.Internal, models.Internal{Name: "cbz", Value: zipReader})

	for _, f := range zipReader.File {
		linkItem := models.Link{}
		linkItem.TypeLink = getMediaTypeByName(f.Name)
		linkItem.Href = f.Name
		if linkItem.TypeLink != "" {
			publication.Spine = append(publication.Spine, linkItem)
		}
	}

	return publication, nil
}

func filePathToTitle(filePath string) string {
	_, filename := filepath.Split(filePath)
	filename = strings.Split(filename, ".")[0]
	title := strings.Replace(filename, "_", " ", -1)

	return title
}

func getMediaTypeByName(filePath string) string {
	ext := filepath.Ext(filePath)

	switch strings.ToLower(ext) {
	case ".jpg":
		return "image/jpeg"
	case ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return ""
	}
}
