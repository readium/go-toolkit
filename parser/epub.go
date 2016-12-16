package parser

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/feedbooks/epub"
	"github.com/feedbooks/webpub-streamer/models"
)

func init() {
	parserList = append(parserList, List{fileExt: "epub", parser: EpubParser})
}

// EpubParser TODO add doc
func EpubParser(filePath string, selfURL string) (models.Publication, error) {
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
		return models.Publication{}, errors.New("can't open or parse epub file with err : " + err.Error())
	}

	if book.Container.Rootfile.Version != "" {
		epubVersion = book.Container.Rootfile.Version
	} else if book.Opf.Version != "" {
		epubVersion = book.Opf.Version
	}

	publication.Internal = append(publication.Internal, models.Internal{Name: "type", Value: "epub"})
	publication.Internal = append(publication.Internal, models.Internal{Name: "epub", Value: book.ZipReader()})
	publication.Internal = append(publication.Internal, models.Internal{Name: "rootfile", Value: book.Container.Rootfile.Path})

	addTitle(&publication, &book.Opf, epubVersion)
	publication.Metadata.Language = book.Opf.Metadata.Language
	addIdentifier(&publication, book, epubVersion)
	publication.Metadata.Right = strings.Join(book.Opf.Metadata.Rights, " ")

	if len(book.Opf.Metadata.Publisher) > 0 {
		for _, pub := range book.Opf.Metadata.Publisher {
			publication.Metadata.Publisher = append(publication.Metadata.Publisher, models.Contributor{Name: pub})
		}
	}

	if book.Opf.Spine.PageProgression != "" {
		publication.Metadata.Direction = book.Opf.Spine.PageProgression
	} else {
		publication.Metadata.Direction = "default"
	}

	if len(book.Opf.Metadata.Contributor) > 0 {
		for _, cont := range book.Opf.Metadata.Contributor {
			addContributor(&publication, book, epubVersion, cont)
		}
	}
	if len(book.Opf.Metadata.Creator) > 0 {
		for _, cont := range book.Opf.Metadata.Creator {
			addContributor(&publication, book, epubVersion, cont)
		}
	}

	fillSpineAndResource(&publication, book)
	addCoverRel(&publication, book)

	fillTOCFromNavDoc(&publication, book)
	if len(publication.TOC) == 0 {
		fillTOCFromNCX(&publication, book)
		fillPageListFromNCX(&publication, book)
	}
	return publication, nil
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
			addMediaOverlay(&linkItem, &item, book)
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
			addMediaOverlay(&linkItem, &item, book)
			return linkItem
		}
	}
	return models.Link{}
}

func addContributor(publication *models.Publication, book *epub.Book, epubVersion string, cont epub.Author) {
	var contributor models.Contributor

	contributor.Name = cont.Data

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
		if epubVersion == "3.0" {
			meta := findMetaByRefineAndProperty(book, cont.ID, "role")
			if meta.Property == "role" {
				contributor.Role = meta.Data
			}
		} else {
			contributor.Role = cont.Role
		}
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

	if linkEpub.Properties == "cover-image" {
		link.Rel = append(link.Rel, "cover")
	}

	if linkEpub.Properties == "nav" {
		link.Rel = append(link.Rel, "contents")
	}

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

	// Second method use item manifest properties is done in addRelToLink

}

func findMetaByRefineAndProperty(book *epub.Book, ID string, property string) epub.Metafield {
	for _, metaTag := range book.Opf.Metadata.Meta {
		if metaTag.Refine == "#"+ID && metaTag.Property == property {
			return metaTag
		}
	}
	return epub.Metafield{}
}

func addMediaOverlay(link *models.Link, linkEpub *epub.Manifest, book *epub.Book) {
	if linkEpub.MediaOverlay != "" {
		meta := findMetaByRefineAndProperty(book, linkEpub.MediaOverlay, "media:duration")
		// format 0:33:35.025
		// splitDuration := strings.Split(meta.Data, ":")
		link.Duration = meta.Data
	}

}

func fillTOCFromNavDoc(publication *models.Publication, book *epub.Book) {

	navLink, err := publication.GetNavDoc()
	if err != nil {
		return
	}

	navReader, err := book.Open(navLink.Href)
	if err != nil {
		return
	}
	defer navReader.Close()
	doc, err := goquery.NewDocumentFromReader(navReader)
	if err != nil {
		return
	}

	doc.Find("nav").Each(func(j int, navElem *goquery.Selection) {
		typeNav, _ := navElem.Attr("epub:type")
		fmt.Println(typeNav)
		if typeNav == "toc" {
			olElem := navElem.Find("ol")
			olElem.Find("li").Each(func(i int, s *goquery.Selection) {
				// For each item found, get the band and title
				href, _ := s.Find("a").Attr("href")
				title := s.Find("a").Text()
				link := models.Link{}
				link.Href = href
				link.Title = title
				publication.TOC = append(publication.TOC, link)
			})
		}
	})

}

func fillPageListFromNCX(publication *models.Publication, book *epub.Book) {
	if len(book.Ncx.PageList.PageTarget) > 0 {
		for _, pageTarget := range book.Ncx.PageList.PageTarget {
			link := models.Link{}
			link.Href = pageTarget.Content.Src
			link.Title = pageTarget.Text
			publication.PageList = append(publication.PageList, link)
		}
	}
}

func fillTOCFromNCX(publication *models.Publication, book *epub.Book) {
	if len(book.Ncx.Points) > 0 {
		for _, point := range book.Ncx.Points {
			fillTOCFromNavPoint(publication, book, point)
		}
	}
}

func fillTOCFromNavPoint(publication *models.Publication, book *epub.Book, point epub.NavPoint) {

	link := models.Link{}
	link.Href = point.Content.Src
	link.Title = point.Text
	publication.TOC = append(publication.TOC, link)
	if len(point.Points) > 0 {
		for _, p := range point.Points {
			fillTOCFromNavPoint(publication, book, p)
		}
	}

}
