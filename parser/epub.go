package parser

import (
	"fmt"
	"time"

	"github.com/feedbooks/epub"
	"github.com/feedbooks/webpub-streamer/models"
)

func init() {
	parserList = append(parserList, List{fileExt: "epub", parser: EpubParser})
}

// EpubParser TODO add doc
func EpubParser(filePath string, selfURL string) models.Publication {
	var manifestStruct models.Publication
	var metaStruct models.Metadata
	var epubVersion string

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
	epubVersion = book.Container.Rootfile.Version
	manifestStruct.Internal = append(manifestStruct.Internal, models.Internal{Name: "epub", Value: book.ZipReader()})
	manifestStruct.Internal = append(manifestStruct.Internal, models.Internal{Name: "rootfile", Value: book.Container.Rootfile.Path})

	addTitle(&metaStruct, &book.Opf, epubVersion)
	metaStruct.Language = book.Opf.Metadata.Language
	metaStruct.Identifier = book.Opf.Metadata.Identifier[0].Data
	if len(book.Opf.Metadata.Contributor) > 0 {
		for _, cont := range book.Opf.Metadata.Contributor {
			addContributor(&metaStruct, cont)
		}
	}
	if len(book.Opf.Metadata.Creator) > 0 {
		for _, cont := range book.Opf.Metadata.Creator {
			addContributor(&metaStruct, cont)
		}
	}

	for _, item := range book.Opf.Spine.Items {
		linkItem := findInManifestByID(book, item.IDref)
		if linkItem.Href != "" {
			manifestStruct.Spine = append(manifestStruct.Spine, linkItem)
		}
	}

	for _, item := range book.Opf.Manifest {

		linkSpine := findInSpineByHref(&manifestStruct, item.Href)
		if linkSpine.Href == "" {
			linkItem := models.Link{}
			linkItem.TypeLink = item.MediaType
			linkItem.Href = item.Href
			manifestStruct.Resources = append(manifestStruct.Resources, linkItem)
		}
	}

	manifestStruct.Metadata = metaStruct
	return manifestStruct
}

func findInSpineByHref(publication *models.Publication, href string) models.Link {
	for _, l := range publication.Spine {
		if l.Href == href {
			return l
		}
	}

	return models.Link{}
}

func findInManifestByID(book *epub.Book, ID string) models.Link {
	for _, item := range book.Opf.Manifest {
		if item.ID == ID {
			linkItem := models.Link{}
			linkItem.TypeLink = item.MediaType
			linkItem.Href = item.Href
			return linkItem
		}
	}
	return models.Link{}
}

func addContributor(metadata *models.Metadata, cont epub.Author) {
	var contributor models.Contributor

	contributor.Name = cont.Data
	contributor.Role = cont.Role
	switch contributor.Role {
	case "aut":
		metadata.Author = append(metadata.Author, contributor)
	case "trl":
		metadata.Translator = append(metadata.Author, contributor)
	case "art":
		metadata.Artist = append(metadata.Artist, contributor)
	case "edt":
		metadata.Editor = append(metadata.Editor, contributor)
	case "ill":
		metadata.Illustrator = append(metadata.Illustrator, contributor)
	case "nrt":
		metadata.Narrator = append(metadata.Narrator, contributor)
	default:
		metadata.Contributor = append(metadata.Contributor, contributor)
	}
}

func addTitle(metadata *models.Metadata, opf *epub.Opf, epubVersion string) {

	if len(opf.Metadata.Title) > 1 && epubVersion == "3.0" {
		for _, titleTag := range opf.Metadata.Title {
			for _, metaTag := range opf.Metadata.Meta {
				if metaTag.Refine == "#"+titleTag.ID {
					if metaTag.Data == "main" {
						metadata.Title = titleTag.Data
					}
				}
			}
		}
	} else {
		metadata.Title = opf.Metadata.Title[0].Data
	}

}
