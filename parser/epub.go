package parser

import (
	"fmt"
	"time"

	"github.com/feedbooks/webpub-streamer/models"
	"github.com/kapmahc/epub"
)

func init() {
	parserList = append(parserList, List{fileExt: "epub", parser: EpubParser})
}

// EpubParser TODO add doc
func EpubParser(filePath string, selfURL string) models.Publication {
	var manifestStruct models.Publication
	var metaStruct models.Metadata

	timeNow := time.Now()
	metaStruct.Modified = &timeNow
	manifestStruct.Links = make([]models.Link, 1)
	manifestStruct.Resources = make([]models.Link, 0)
	manifestStruct.Resources = make([]models.Link, 0)
	if selfURL != "" {
		self := models.Link{
			Rel:      []string{"self"},
			Href:     selfURL,
			TypeLink: "application/json",
		}
		manifestStruct.Links[0] = self
	}

	book, err := epub.Open(filePath)
	if err != nil {
		fmt.Println(err)
		return models.Publication{}
	}
	manifestStruct.Internal = append(manifestStruct.Internal, models.Internal{Name: "epub", Value: book})

	metaStruct.Title = book.Opf.Metadata.Title[0]

	metaStruct.Language = book.Opf.Metadata.Language
	metaStruct.Identifier = book.Opf.Metadata.Identifier[0].Data
	if len(book.Opf.Metadata.Contributor) > 0 {
		aut := models.Contributor{}
		aut.Name = book.Opf.Metadata.Contributor[0].Data
		metaStruct.Author = append(metaStruct.Author, aut)
	}
	if len(book.Opf.Metadata.Creator) > 0 {
		aut := models.Contributor{}
		aut.Name = book.Opf.Metadata.Creator[0].Data
		metaStruct.Author = append(metaStruct.Author, aut)
	}

	for _, item := range book.Opf.Manifest {
		linkItem := models.Link{}
		linkItem.TypeLink = item.MediaType
		linkItem.Href = item.Href
		if linkItem.TypeLink == "application/xhtml+xml" {
			manifestStruct.Spine = append(manifestStruct.Spine, linkItem)
		} else {
			manifestStruct.Resources = append(manifestStruct.Resources, linkItem)
		}
	}

	manifestStruct.Metadata = metaStruct
	return manifestStruct
}
