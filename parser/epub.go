package parser

import (
	"time"

	"github.com/feedbooks/webpub-streamer/models"
	"github.com/kapmahc/epub"
)

// Epub TODO add doc
type Epub struct {
}

// Parse TODO add doc
// func (parser *Epub) Parse(filename string, filepath string, host string) models.Publication {
func Parse(filename string, filepath string, host string) models.Publication {
	var manifestStruct models.Publication
	var metaStruct models.Metadata

	self := models.Link{
		Rel:      []string{"self"},
		Href:     "http://" + host + "/" + filename + "/manifest.json",
		TypeLink: "application/json",
	}
	timeNow := time.Now()
	metaStruct.Modified = &timeNow
	manifestStruct.Links = make([]models.Link, 1)
	manifestStruct.Resources = make([]models.Link, 0)
	manifestStruct.Resources = make([]models.Link, 0)
	manifestStruct.Links[0] = self

	book, _ := epub.Open(filepath)

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
