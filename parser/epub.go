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
	var publication models.Publication
	var metaStruct models.Metadata
	var epubVersion string

	timeNow := time.Now()
	metaStruct.Modified = &timeNow
	publication.Metadata = metaStruct
	publication.Links = make([]models.Link, 1)
	publication.Resources = make([]models.Link, 0)

	if selfURL != "" {
		self := models.Link{
			Rel:      []string{"self"},
			Href:     selfURL,
			TypeLink: "application/json",
		}
		publication.Links[0] = self
	}

	book, err := epub.Open(filePath)
	if err != nil {
		fmt.Println(err)
		return models.Publication{}
	}
	epubVersion = book.Container.Rootfile.Version
	publication.Internal = append(publication.Internal, models.Internal{Name: "type", Value: "epub"})
	publication.Internal = append(publication.Internal, models.Internal{Name: "epub", Value: book.ZipReader()})
	publication.Internal = append(publication.Internal, models.Internal{Name: "rootfile", Value: book.Container.Rootfile.Path})

	addTitle(&publication, &book.Opf, epubVersion)
	publication.Metadata.Language = book.Opf.Metadata.Language
	addIdentifier(&publication, book, epubVersion)
	if len(book.Opf.Metadata.Contributor) > 0 {
		for _, cont := range book.Opf.Metadata.Contributor {
			addContributor(&publication, cont)
		}
	}
	if len(book.Opf.Metadata.Creator) > 0 {
		for _, cont := range book.Opf.Metadata.Creator {
			addContributor(&publication, cont)
		}
	}

	fillSpineAndResource(&publication, book)
	addCoverRel(&publication, book)

	return publication
}

func fillSpineAndResource(publication *models.Publication, book *epub.Book) {

	for _, item := range book.Opf.Spine.Items {
		linkItem := findInManifestByID(book, item.IDref)
		if linkItem.Href != "" {
			publication.Spine = append(publication.Spine, linkItem)
		}
	}

	for _, item := range book.Opf.Manifest {
		linkSpine := findInSpineByHref(publication, item.Href)
		if linkSpine.Href == "" {
			linkItem := models.Link{}
			linkItem.TypeLink = item.MediaType
			linkItem.Href = item.Href
			addRelToLink(&linkItem, &item)
			publication.Resources = append(publication.Resources, linkItem)
		}
	}

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
			addRelToLink(&linkItem, &item)
			return linkItem
		}
	}
	return models.Link{}
}

func addContributor(publication *models.Publication, cont epub.Author) {
	var contributor models.Contributor

	contributor.Name = cont.Data
	contributor.Role = cont.Role
	switch contributor.Role {
	case "aut":
		publication.Metadata.Author = append(publication.Metadata.Author, contributor)
	case "trl":
		publication.Metadata.Translator = append(publication.Metadata.Author, contributor)
	case "art":
		publication.Metadata.Artist = append(publication.Metadata.Artist, contributor)
	case "edt":
		publication.Metadata.Editor = append(publication.Metadata.Editor, contributor)
	case "ill":
		publication.Metadata.Illustrator = append(publication.Metadata.Illustrator, contributor)
		//	case "???":
		//		metadata.Letterer = append(metadata.Letterer, contributor)
		//	case "pen":
		//		metadata.Penciler = append(metadata.Penciler, contributor)
	case "clr":
		publication.Metadata.Colorist = append(publication.Metadata.Colorist, contributor)
		//	case "ink":
		//		metadata.Inker = append(metadata.Inker, contributor)
	case "nrt":
		publication.Metadata.Narrator = append(publication.Metadata.Narrator, contributor)
	case "pbl":
		publication.Metadata.Publisher = append(publication.Metadata.Publisher, contributor)
	default:
		publication.Metadata.Contributor = append(publication.Metadata.Contributor, contributor)
	}
}

func addTitle(publication *models.Publication, opf *epub.Opf, epubVersion string) {

	if len(opf.Metadata.Title) > 1 && epubVersion == "3.0" {
		for _, titleTag := range opf.Metadata.Title {
			for _, metaTag := range opf.Metadata.Meta {
				if metaTag.Refine == "#"+titleTag.ID {
					if metaTag.Data == "main" {
						publication.Metadata.Title = titleTag.Data
					}
				}
			}
		}
	} else {
		publication.Metadata.Title = opf.Metadata.Title[0].Data
	}

}

func addIdentifier(publication *models.Publication, book *epub.Book, epubVersion string) {
	if len(book.Opf.Metadata.Identifier) > 1 {
		uniqueID := book.Opf.UniqueIdentifier
		for _, iden := range book.Opf.Metadata.Identifier {
			if iden.ID == uniqueID {
				publication.Metadata.Identifier = iden.Data
			}
		}
	} else {
		publication.Metadata.Identifier = book.Opf.Metadata.Identifier[0].Data
	}
}

func addRelToLink(link *models.Link, linkEpub *epub.Manifest) {
	//fmt.Println(linkEpub.Properties)
	//if linkEpub.Properties == "cover" {
	//	link.Rel = append(link.Rel, "cover")
	//}

}

func addCoverRel(publication *models.Publication, book *epub.Book) {
	// First method using meta content
	var coverID string

	for _, meta := range book.Opf.Metadata.Meta {
		if meta.Name == "cover" {
			coverID = meta.Content
		}
	}
	if coverID != "" {
		manifestInfo := findInManifestByID(book, coverID)
		if manifestInfo.Href != "" {
			for i, item := range publication.Resources {
				if item.Href == manifestInfo.Href {
					publication.Resources[i].Rel = append(item.Rel, "cover")
				}
			}
		}

	}

}
