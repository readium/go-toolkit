package parser

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/feedbooks/epub"
	"github.com/feedbooks/r2-streamer-go/models"
)

const epub3 = "3.0"

func init() {
	parserList = append(parserList, List{fileExt: "epub", parser: EpubParser})
}

// EpubParser TODO add doc
func EpubParser(filePath string) (models.Publication, error) {
	var publication models.Publication
	var metaStruct models.Metadata
	var epubVersion string

	timeNow := time.Now()
	metaStruct.Modified = &timeNow
	publication.Metadata = metaStruct
	publication.Resources = make([]models.Link, 0)

	book, err := epub.Open(filePath)
	if err != nil {
		return models.Publication{}, errors.New("can't open or parse epub file with err : " + err.Error())
	}

	if book.Container.Rootfile.Version != "" {
		epubVersion = book.Container.Rootfile.Version
	} else if book.Opf.Version != "" {
		epubVersion = book.Opf.Version
	}

	_, filename := filepath.Split(filePath)

	publication.Internal = append(publication.Internal, models.Internal{Name: "filename", Value: filename})
	publication.Internal = append(publication.Internal, models.Internal{Name: "type", Value: "epub"})
	publication.Internal = append(publication.Internal, models.Internal{Name: "epub", Value: book.ZipReader()})
	publication.Internal = append(publication.Internal, models.Internal{Name: "rootfile", Value: book.Container.Rootfile.Path})

	addTitle(&publication, book, epubVersion)
	publication.Metadata.Language = book.Opf.Metadata.Language
	addIdentifier(&publication, book, epubVersion)
	publication.Metadata.Right = strings.Join(book.Opf.Metadata.Rights, " ")
	if len(book.Opf.Metadata.Description) > 0 {
		publication.Metadata.Description = book.Opf.Metadata.Description[0]
	}

	if len(book.Opf.Metadata.Publisher) > 0 {
		for _, pub := range book.Opf.Metadata.Publisher {
			publication.Metadata.Publisher = append(publication.Metadata.Publisher, models.Contributor{Name: models.MultiLanguage{SingleString: pub}})
		}
	}

	if len(book.Opf.Metadata.Source) > 0 {
		publication.Metadata.Source = book.Opf.Metadata.Source[0]
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

	if epubVersion == epub3 {
		findContributorInMeta(&publication, book, epubVersion)
	}

	fillSpineAndResource(&publication, book)
	addRendition(&publication, book)
	addCoverRel(&publication, book)

	fillTOCFromNavDoc(&publication, book)
	if len(publication.TOC) == 0 {
		fillTOCFromNCX(&publication, book)
		fillPageListFromNCX(&publication, book)
	}

	fillCalibreSerieInfo(&publication, book)
	return publication, nil
}

func fillSpineAndResource(publication *models.Publication, book *epub.Book) {

	for _, item := range book.Opf.Spine.Items {
		if item.Linear == "yes" || item.Linear == "" {

			linkItem := findInManifestByID(book, item.IDref)

			if linkItem.Href != "" {
				publication.Spine = append(publication.Spine, linkItem)
			}
		}
	}

	for _, item := range book.Opf.Manifest {
		linkSpine := findInSpineByHref(publication, item.Href)
		if linkSpine.Href == "" {
			linkItem := models.Link{}
			linkItem.TypeLink = item.MediaType
			linkItem.Href = item.Href
			addRelAndPropertiesToLink(&linkItem, &item, book)
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
			addRelAndPropertiesToLink(&linkItem, &item, book)
			addMediaOverlay(&linkItem, &item, book)
			return linkItem
		}
	}
	return models.Link{}
}

func findContributorInMeta(publication *models.Publication, book *epub.Book, epubVersion string) {

	for _, meta := range book.Opf.Metadata.Meta {
		if meta.Property == "dcterms:creator" || meta.Property == "dcterms:contributor" {
			cont := epub.Author{}
			cont.Data = meta.Data
			cont.ID = meta.ID
			addContributor(publication, book, epubVersion, cont)

		}
	}

}

func addContributor(publication *models.Publication, book *epub.Book, epubVersion string, cont epub.Author) {
	var contributor models.Contributor
	var role string

	if epubVersion == "3.0" {
		meta := findMetaByRefineAndProperty(book, cont.ID, "role")
		if meta.Property == "role" {
			role = meta.Data
		}

		metaAlt := findAllMetaByRefineAndProperty(book, cont.ID, "alternate-script")
		if len(metaAlt) > 0 {
			contributor.Name.MultiString = make(map[string]string)
			contributor.Name.MultiString[publication.Metadata.Language[0]] = cont.Data

			for _, m := range metaAlt {
				contributor.Name.MultiString[m.Lang] = m.Data
			}
		} else {
			contributor.Name.SingleString = cont.Data
		}

	} else {
		contributor.Name.SingleString = cont.Data
		role = cont.Role
	}

	switch role {
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
		contributor.Role = role
		publication.Metadata.Contributor = append(publication.Metadata.Contributor, contributor)
	}
}

func addTitle(publication *models.Publication, book *epub.Book, epubVersion string) {

	if epubVersion == "3.0" {
		var mainTitle epub.Title

		if len(book.Opf.Metadata.Title) > 1 {
			for _, titleTag := range book.Opf.Metadata.Title {
				for _, metaTag := range book.Opf.Metadata.Meta {
					if metaTag.Refine == "#"+titleTag.ID {
						if metaTag.Data == "main" {
							mainTitle = titleTag
						}
					}
				}
			}
		} else {
			mainTitle = book.Opf.Metadata.Title[0]
		}

		metaAlt := findAllMetaByRefineAndProperty(book, mainTitle.ID, "alternate-script")
		if len(metaAlt) > 0 {
			publication.Metadata.Title.MultiString = make(map[string]string)
			publication.Metadata.Title.MultiString[strings.ToLower(mainTitle.Lang)] = mainTitle.Data

			for _, m := range metaAlt {
				publication.Metadata.Title.MultiString[strings.ToLower(m.Lang)] = m.Data
			}
		} else {
			publication.Metadata.Title.SingleString = mainTitle.Data
		}

	} else {
		publication.Metadata.Title.SingleString = book.Opf.Metadata.Title[0].Data
	}

	fmt.Println(publication.Metadata.Title)

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

func addRelAndPropertiesToLink(link *models.Link, linkEpub *epub.Manifest, book *epub.Book) {

	if linkEpub.Properties != "" {
		addToLinkFromProperties(link, linkEpub.Properties)
	}
	spineProperties := findPropertiesInSpineForManifest(linkEpub, book)
	if spineProperties != "" {
		addToLinkFromProperties(link, spineProperties)
	}
}

func findPropertiesInSpineForManifest(linkEpub *epub.Manifest, book *epub.Book) string {

	for _, item := range book.Opf.Spine.Items {
		if item.IDref == linkEpub.ID {
			return item.Properties
		}
	}

	return ""
}

func addToLinkFromProperties(link *models.Link, propertiesString string) {
	var properties []string
	var propertiesStruct models.Properties

	properties = strings.Split(propertiesString, " ")

	// vocabulary list can be consulted here https://idpf.github.io/epub-vocabs/rendition/
	for _, p := range properties {
		switch p {
		case "cover-image":
			link.Rel = append(link.Rel, "cover")
		case "nav":
			link.Rel = append(link.Rel, "contents")
		case "scripted":
			propertiesStruct.Contains = append(propertiesStruct.Contains, "js")
		case "mathml":
			propertiesStruct.Contains = append(propertiesStruct.Contains, "mathml")
		case "onix-record":
			propertiesStruct.Contains = append(propertiesStruct.Contains, "onix")
		case "svg":
			propertiesStruct.Contains = append(propertiesStruct.Contains, "svg")
		case "xmp-record":
			propertiesStruct.Contains = append(propertiesStruct.Contains, "xmp")
		case "remote-resources":
			propertiesStruct.Contains = append(propertiesStruct.Contains, "remote-resources")
		case "page-spread-left":
			propertiesStruct.Page = "left"
		case "page-spread-right":
			propertiesStruct.Page = "right"
		case "page-spread-center":
			propertiesStruct.Page = "center"
		case "rendition:spread-none":
			propertiesStruct.Spread = "none"
		case "rendition:spread-auto":
			propertiesStruct.Spread = "auto"
		case "rendition:spread-landscape":
			propertiesStruct.Spread = "landscape"
		case "rendition:spread-portrait":
			propertiesStruct.Spread = "portrait"
		case "rendition:spread-both":
			propertiesStruct.Spread = "both"
		case "rendition:layout-reflowable":
			propertiesStruct.Layout = "reflowable"
		case "rendition:layout-pre-paginated":
			propertiesStruct.Layout = "fixed"
		case "rendition:orientation-auto":
			propertiesStruct.Orientation = "auto"
		case "rendition:orientation-landscape":
			propertiesStruct.Orientation = "landscape"
		case "rendition:orientation-portrait":
			propertiesStruct.Orientation = "portrait"
		case "rendition:flow-auto":
			propertiesStruct.Overflow = "auto"
		case "rendition:flow-paginated":
			propertiesStruct.Overflow = "paginated"
		case "rendition:flow-scrolled-continuous":
			propertiesStruct.Overflow = "scrolled-continuous"
		case "rendition:flow-scrolled-doc":
			propertiesStruct.Overflow = "scrolled"
		}

		if propertiesStruct.Layout != "" || propertiesStruct.Orientation != "" || propertiesStruct.Overflow != "" || propertiesStruct.Page != "" || propertiesStruct.Spread != "" || len(propertiesStruct.Contains) > 0 {
			link.Properties = &propertiesStruct
		}
	}
}

func addRendition(publication *models.Publication, book *epub.Book) {
	var rendition models.Properties

	for _, meta := range book.Opf.Metadata.Meta {
		switch meta.Property {
		case "rendition:layout":
			if meta.Data == "pre-paginated" {
				rendition.Layout = "fixed"
			} else if meta.Data == "reflowable" {
				rendition.Layout = "reflowable"
			}
		case "rendition:orientation":
			rendition.Orientation = meta.Data
		case "rendition:spread":
			rendition.Spread = meta.Data
		case "rendition:flow":
			rendition.Overflow = meta.Data
		}
	}

	if rendition.Layout != "" || rendition.Orientation != "" || rendition.Overflow != "" || rendition.Page != "" || rendition.Spread != "" {
		publication.Metadata.Rendition = &rendition
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

func findAllMetaByRefineAndProperty(book *epub.Book, ID string, property string) []epub.Metafield {
	var metas []epub.Metafield

	for _, metaTag := range book.Opf.Metadata.Meta {
		if metaTag.Refine == "#"+ID && metaTag.Property == property {
			metas = append(metas, metaTag)
		}
	}
	return metas
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
		if typeNav == "toc" {
			olElem := navElem.ChildrenFiltered("ol")
			fillTOCFromNavDocWithOL(olElem, &publication.TOC)
		}
		if typeNav == "page-list" {
			olElem := navElem.ChildrenFiltered("ol")
			fillTOCFromNavDocWithOL(olElem, &publication.PageList)
		}
		if typeNav == "landmarks" {
			olElem := navElem.ChildrenFiltered("ol")
			fillTOCFromNavDocWithOL(olElem, &publication.Landmarks)
		}
	})

}

func fillTOCFromNavDocWithOL(olElem *goquery.Selection, node *[]models.Link) {
	olElem.ChildrenFiltered("li").Each(func(i int, s *goquery.Selection) {
		if s.ChildrenFiltered("span").Text() != "" {
			nextOlElem := s.ChildrenFiltered("ol")
			fillTOCFromNavDocWithOL(nextOlElem, node)
		} else {
			href, _ := s.ChildrenFiltered("a").Attr("href")
			title := s.ChildrenFiltered("a").Text()
			link := models.Link{}
			link.Href = href
			link.Title = title
			nextOlElem := s.ChildrenFiltered("ol")
			if nextOlElem != nil {
				fillTOCFromNavDocWithOL(nextOlElem, &link.Children)
			}
			*node = append(*node, link)
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
			fillTOCFromNavPoint(publication, book, point, &publication.TOC)
		}
	}
}

func fillTOCFromNavPoint(publication *models.Publication, book *epub.Book, point epub.NavPoint, node *[]models.Link) {

	link := models.Link{}
	link.Href = point.Content.Src
	link.Title = point.Text
	if len(point.Points) > 0 {
		for _, p := range point.Points {
			fillTOCFromNavPoint(publication, book, p, &link.Children)
		}
	}
	*node = append(*node, link)

}

func fillCalibreSerieInfo(publication *models.Publication, book *epub.Book) {
	var serie string
	var seriePosition float32

	for _, m := range book.Opf.Metadata.Meta {
		if m.Name == "calibre:series" {
			serie = m.Content
		}
		if m.Name == "calibre:series_index" {
			index, err := strconv.ParseFloat(m.Content, 32)
			if err == nil {
				seriePosition = float32(index)
			}
		}
	}

	if serie != "" {
		collection := models.Collection{Name: serie, Position: seriePosition}
		publication.Metadata.BelongsTo.Series = append(publication.Metadata.BelongsTo.Series, collection)
	}

}
