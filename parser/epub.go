package parser

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/readium/r2-streamer-go/fetcher"
	"github.com/readium/r2-streamer-go/models"
	"github.com/readium/r2-streamer-go/parser/epub"
)

const epub3 = "3.0"
const epub31 = "3.1"
const epub2 = "2.0"
const epub201 = "2.0.1"
const autoMeta = "auto"
const noneMeta = "none"
const reflowableMeta = "reflowable"
const mediaOverlayURL = "media-overlay?resource="

func init() {
	parserList = append(parserList, List{fileExt: "epub", parser: EpubParser, callback: EpubCallback})
}

// EpubParser TODO add doc
func EpubParser(filePath string) (models.Publication, error) {
	var publication models.Publication
	var metaStruct models.Metadata
	var epubVersion string
	var err error
	var book *epub.Epub

	timeNow := time.Now()
	metaStruct.Modified = &timeNow
	publication.Metadata = metaStruct
	publication.Resources = make([]models.Link, 0)

	fileExt := filepath.Ext(filePath)
	if fileExt == "" {
		book, err = epub.OpenDir(filePath)
		if err != nil {
			return models.Publication{}, errors.New("can't open or parse epub file with err : " + err.Error())
		}
		publication.AddToInternal("type", "epub_dir")
		publication.AddToInternal("basepath", filePath)
	} else {
		book, err = epub.OpenEpub(filePath)
		if err != nil {
			return models.Publication{}, errors.New("can't open or parse epub file with err : " + err.Error())
		}
		publication.AddToInternal("type", "epub")
		publication.AddToInternal("epub", book.ZipReader())
	}

	publication.Context = append(publication.Context, "https://readium.org/webpub-manifest/context.jsonld")
	publication.Metadata.RDFType = "http://schema.org/Book"

	epubVersion = getEpubVersion(book)
	_, filename := filepath.Split(filePath)

	publication.AddToInternal("filename", filename)
	publication.AddToInternal("rootfile", book.Container.Rootfile.Path)

	addTitle(&publication, book)
	publication.Metadata.Language = book.Opf.Metadata.Language
	addIdentifier(&publication, book, epubVersion)
	publication.Metadata.Rights = strings.Join(book.Opf.Metadata.Rights, " ")
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
	}

	if len(book.Opf.Metadata.Contributor) > 0 {
		for _, cont := range book.Opf.Metadata.Contributor {
			addContributor(&publication, book, epubVersion, cont, "")
		}
	}
	if len(book.Opf.Metadata.Creator) > 0 {
		for _, cont := range book.Opf.Metadata.Creator {
			addContributor(&publication, book, epubVersion, cont, "aut")
		}
	}

	if isEpub3OrMore(book) {
		findContributorInMeta(&publication, book, epubVersion)
	}

	fillSpineAndResource(&publication, book)
	addPresentation(&publication, book)
	addCoverRel(&publication, book)
	fillEncryptionInfo(&publication, book)

	fillTOCFromNavDoc(&publication, book)
	if len(publication.TOC) == 0 {
		fillTOCFromNCX(&publication, book)
		fillPageListFromNCX(&publication, book)
		fillLandmarksFromGuide(&publication, book)
	}

	fillCalibreSerieInfo(&publication, book)
	fillSubject(&publication, book)
	fillPublicationDate(&publication, book)
	fillMediaOverlay(&publication, book)

	return publication, nil
}

// EpubCallback reparse smil file and more to come
func EpubCallback(publication *models.Publication) {
	fillMediaOverlay(publication, nil)
}

func fillSpineAndResource(publication *models.Publication, book *epub.Epub) {

	for _, item := range book.Opf.Spine.Items {
		if item.Linear == "yes" || item.Linear == "" {

			linkItem := findInManifestByID(book, item.IDref)

			if linkItem.Href != "" {
				publication.ReadingOrder = append(publication.ReadingOrder, linkItem)
			}
		}
	}

	for _, item := range book.Opf.Manifest {
		linkItem := models.Link{}
		linkItem.TypeLink = item.MediaType
		linkItem.AddHrefAbsolute(item.Href, book.Container.Rootfile.Path)

		linkSpine := findInSpineByHref(publication, linkItem.Href)
		if linkSpine.Href == "" {
			addRelAndPropertiesToLink(&linkItem, &item, book)
			addMediaOverlay(&linkItem, &item, book)
			publication.Resources = append(publication.Resources, linkItem)
		}
	}

}

func findInSpineByHref(publication *models.Publication, href string) models.Link {
	for _, l := range publication.ReadingOrder {
		if l.Href == href {
			return l
		}
	}

	return models.Link{}
}

func findInManifestByID(book *epub.Epub, ID string) models.Link {
	for _, item := range book.Opf.Manifest {
		if item.ID == ID {
			linkItem := models.Link{}
			linkItem.TypeLink = item.MediaType
			linkItem.AddHrefAbsolute(item.Href, book.Container.Rootfile.Path)
			addRelAndPropertiesToLink(&linkItem, &item, book)
			addMediaOverlay(&linkItem, &item, book)
			return linkItem
		}
	}
	return models.Link{}
}

func findContributorInMeta(publication *models.Publication, book *epub.Epub, epubVersion string) {

	for _, meta := range book.Opf.Metadata.Meta {
		if meta.Property == "dcterms:creator" || meta.Property == "dcterms:contributor" {
			cont := epub.Author{}
			cont.Data = meta.Data
			cont.ID = meta.ID
			addContributor(publication, book, epubVersion, cont, "")

		}
	}

}

func addContributor(publication *models.Publication, book *epub.Epub, epubVersion string, cont epub.Author, forcedRole string) {
	var contributor models.Contributor
	var role string

	if isEpub3OrMore(book) {
		meta := findMetaByRefineAndProperty(book, cont.ID, "role")
		if meta.Property == "role" {
			role = meta.Data
		}
		if role == "" && forcedRole != "" {
			role = forcedRole
		}

		metaAlt := findAllMetaByRefineAndProperty(book, cont.ID, "alternate-script")
		if len(metaAlt) > 0 {
			contributor.Name.MultiString = make(map[string]string)
			contributor.Name.MultiString[strings.ToLower(publication.Metadata.Language[0])] = cont.Data

			for _, m := range metaAlt {
				contributor.Name.MultiString[strings.ToLower(m.Lang)] = m.Data
			}
		} else {
			contributor.Name.SingleString = cont.Data
		}

	} else {
		contributor.Name.SingleString = cont.Data
		role = cont.Role
		if role == "" && forcedRole != "" {
			role = forcedRole
		}
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

func addTitle(publication *models.Publication, book *epub.Epub) {

	if isEpub3OrMore(book) {
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
		if len(book.Opf.Metadata.Title) > 0 {
			publication.Metadata.Title.SingleString = book.Opf.Metadata.Title[0].Data
		}
	}

}

func addIdentifier(publication *models.Publication, book *epub.Epub, epubVersion string) {
	if len(book.Opf.Metadata.Identifier) > 1 {
		uniqueID := book.Opf.UniqueIdentifier
		for _, iden := range book.Opf.Metadata.Identifier {
			if iden.ID == uniqueID {
				publication.Metadata.Identifier = iden.Data
			}
		}
	} else {
		if len(book.Opf.Metadata.Identifier) > 0 {
			publication.Metadata.Identifier = book.Opf.Metadata.Identifier[0].Data
		}
	}
}

func addRelAndPropertiesToLink(link *models.Link, linkEpub *epub.Manifest, book *epub.Epub) {

	if linkEpub.Properties != "" {
		addToLinkFromProperties(link, linkEpub.Properties)
	}
	spineProperties := findPropertiesInSpineForManifest(linkEpub, book)
	if spineProperties != "" {
		addToLinkFromProperties(link, spineProperties)
	}
}

func findPropertiesInSpineForManifest(linkEpub *epub.Manifest, book *epub.Epub) string {

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
			link.AddRel("cover")
		case "nav":
			link.AddRel("contents")
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
			propertiesStruct.Spread = noneMeta
		case "rendition:spread-auto":
			propertiesStruct.Spread = autoMeta
		case "rendition:spread-landscape":
			propertiesStruct.Spread = "landscape"
		case "rendition:spread-portrait":
			propertiesStruct.Spread = "both"
		case "rendition:spread-both":
			propertiesStruct.Spread = "both"
		case "rendition:layout-reflowable":
			propertiesStruct.Layout = reflowableMeta
		case "rendition:layout-pre-paginated":
			propertiesStruct.Layout = "fixed"
		case "rendition:orientation-auto":
			propertiesStruct.Orientation = "auto"
		case "rendition:orientation-landscape":
			propertiesStruct.Orientation = "landscape"
		case "rendition:orientation-portrait":
			propertiesStruct.Orientation = "portrait"
		case "rendition:flow-auto":
			propertiesStruct.Overflow = autoMeta
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

func addPresentation(publication *models.Publication, book *epub.Epub) {
	var presentation models.Properties

	for _, meta := range book.Opf.Metadata.Meta {
		switch meta.Property {
		case "rendition:layout":
			if meta.Data == "pre-paginated" {
				presentation.Layout = "fixed"
			} else if meta.Data == "reflowable" {
				presentation.Layout = "reflowable"
			}
		case "rendition:orientation":
			presentation.Orientation = meta.Data
		case "rendition:spread":
			presentation.Spread = meta.Data
		case "rendition:flow":
			presentation.Overflow = meta.Data
		}
	}

	if presentation.Layout != "" || presentation.Orientation != "" || presentation.Overflow != "" || presentation.Page != "" || presentation.Spread != "" {
		publication.Metadata.Presentation = &presentation
	}
}

func addCoverRel(publication *models.Publication, book *epub.Epub) {
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
					publication.Resources[i].AddRel("cover")
				}
			}
		}
	}

	// Second method use item manifest properties is done in addRelToLink

}

func findMetaByRefineAndProperty(book *epub.Epub, ID string, property string) epub.Metafield {
	for _, metaTag := range book.Opf.Metadata.Meta {
		if metaTag.Refine == "#"+ID && metaTag.Property == property {
			return metaTag
		}
	}
	return epub.Metafield{}
}

func findAllMetaByRefineAndProperty(book *epub.Epub, ID string, property string) []epub.Metafield {
	var metas []epub.Metafield

	for _, metaTag := range book.Opf.Metadata.Meta {
		if metaTag.Refine == "#"+ID && metaTag.Property == property {
			metas = append(metas, metaTag)
		}
	}
	return metas
}

func addMediaOverlay(link *models.Link, linkEpub *epub.Manifest, book *epub.Epub) {
	if linkEpub.MediaOverlay != "" {
		meta := findMetaByRefineAndProperty(book, linkEpub.MediaOverlay, "media:duration")

		link.Duration = smilTimeToSeconds(meta.Data)
	}

}

func fillTOCFromNavDoc(publication *models.Publication, book *epub.Epub) {

	navLink, err := publication.GetNavDoc()
	if err != nil {
		return
	}
	navReader, err := book.RawOpen(navLink.Href)
	if err != nil {
		fmt.Println("can't open nav doc file : " + err.Error())
		return
	}
	defer navReader.Close()
	doc, err := goquery.NewDocumentFromReader(navReader)
	if err != nil {
		fmt.Println("can't parse navdoc : " + err.Error())
		return
	}

	doc.Find("nav").Each(func(j int, navElem *goquery.Selection) {
		typeNav, _ := navElem.Attr("epub:type")
		olElem := navElem.ChildrenFiltered("ol")
		switch typeNav {
		case "toc":
			fillTOCFromNavDocWithOL(olElem, &publication.TOC, navLink.Href)
		case "page-list":
			fillTOCFromNavDocWithOL(olElem, &publication.PageList, navLink.Href)
		case "landmarks":
			fillTOCFromNavDocWithOL(olElem, &publication.Landmarks, navLink.Href)
		case "lot":
			fillTOCFromNavDocWithOL(olElem, &publication.LOT, navLink.Href)
		case "loa":
			fillTOCFromNavDocWithOL(olElem, &publication.LOA, navLink.Href)
		case "loi":
			fillTOCFromNavDocWithOL(olElem, &publication.LOI, navLink.Href)
		case "lov":
			fillTOCFromNavDocWithOL(olElem, &publication.LOV, navLink.Href)
		}
	})

}

func fillTOCFromNavDocWithOL(olElem *goquery.Selection, node *[]models.Link, navDocURL string) {
	olElem.ChildrenFiltered("li").Each(func(i int, s *goquery.Selection) {
		if s.ChildrenFiltered("span").Text() != "" {
			nextOlElem := s.ChildrenFiltered("ol")
			fillTOCFromNavDocWithOL(nextOlElem, node, navDocURL)
		} else {
			href, _ := s.ChildrenFiltered("a").Attr("href")
			if href[0] == '#' {
				href = navDocURL + href
			}
			title := s.ChildrenFiltered("a").Text()
			link := models.Link{}
			link.AddHrefAbsolute(href, navDocURL)
			link.Title = title
			nextOlElem := s.ChildrenFiltered("ol")
			if nextOlElem != nil {
				fillTOCFromNavDocWithOL(nextOlElem, &link.Children, navDocURL)
			}
			*node = append(*node, link)
		}
	})
}

func fillPageListFromNCX(publication *models.Publication, book *epub.Epub) {
	if len(book.Ncx.PageList.PageTarget) > 0 {
		for _, pageTarget := range book.Ncx.PageList.PageTarget {
			link := models.Link{}
			link.AddHrefAbsolute(pageTarget.Content.Src, book.NcxPath)
			link.Title = pageTarget.Text
			publication.PageList = append(publication.PageList, link)
		}
	}
}

func fillTOCFromNCX(publication *models.Publication, book *epub.Epub) {
	if len(book.Ncx.Points) > 0 {
		for _, point := range book.Ncx.Points {
			fillTOCFromNavPoint(publication, book, point, &publication.TOC)
		}
	}
}

func fillLandmarksFromGuide(publication *models.Publication, book *epub.Epub) {
	if len(book.Opf.Guide) > 0 {
		for _, ref := range book.Opf.Guide {
			if ref.Href != "" {
				link := models.Link{}
				link.AddHrefAbsolute(ref.Href, book.Container.Rootfile.Path)
				link.Title = ref.Title
				publication.Landmarks = append(publication.Landmarks, link)
			}
		}
	}
}

func fillTOCFromNavPoint(publication *models.Publication, book *epub.Epub, point epub.NavPoint, node *[]models.Link) {

	link := models.Link{}
	link.AddHrefAbsolute(point.Content.Src, book.NcxPath)
	link.Title = point.Text
	if len(point.Points) > 0 {
		for _, p := range point.Points {
			fillTOCFromNavPoint(publication, book, p, &link.Children)
		}
	}
	*node = append(*node, link)

}

func fillCalibreSerieInfo(publication *models.Publication, book *epub.Epub) {
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
		if publication.Metadata.BelongsTo == nil {
			publication.Metadata.BelongsTo = &models.BelongsTo{}
		}
		publication.Metadata.BelongsTo.Series = append(publication.Metadata.BelongsTo.Series, collection)
	}

}

func fillEncryptionInfo(publication *models.Publication, book *epub.Epub) {

	for _, encInfo := range book.Encryption.EncryptedData {
		encrypted := models.Encrypted{}
		encrypted.Algorithm = encInfo.EncryptionMethod.Algorithm
		if book.LCP.ID != "" {
			encrypted.Profile = book.LCP.Encryption.Profile
			encrypted.Scheme = "http://readium.org/2014/01/lcp"
		}
		if len(encInfo.EncryptionProperties) > 0 {
			for _, prop := range encInfo.EncryptionProperties {
				if prop.Compression.OriginalLength != "" {
					encrypted.OriginalLength, _ = strconv.Atoi(prop.Compression.OriginalLength)
					if prop.Compression.Method == "8" {
						encrypted.Compression = "deflate"
					} else {
						encrypted.Compression = "none"
					}
				}
			}
		}
		resURI := encInfo.CipherData.CipherReference.URI

		for i, l := range publication.Resources {
			if resURI == l.Href {
				if l.Properties == nil {
					publication.Resources[i].Properties = &models.Properties{}
				}
				publication.Resources[i].Properties.Encrypted = &encrypted
			}
		}
		for i, l := range publication.ReadingOrder {
			if resURI == l.Href {
				if l.Properties == nil {
					publication.ReadingOrder[i].Properties = &models.Properties{}
				}
				publication.ReadingOrder[i].Properties.Encrypted = &encrypted
			}
		}
	}

	if book.LCP.ID != "" {
		decodedKeyCheck, _ := base64.StdEncoding.DecodeString(book.LCP.Encryption.UserKey.KeyCheck)
		decodedContentKey, _ := base64.StdEncoding.DecodeString(book.LCP.Encryption.ContentKey.EncryptedValue)
		publication.LCP = book.LCP

		lcpData, errLcp := book.GetData("META-INF/license.lcpl")
		if errLcp == nil {
			publication.AddToInternal("lcpl", lcpData)
		}
		publication.AddToInternal("lcp_id", book.LCP.ID)
		publication.AddToInternal("lcp_content_key", decodedContentKey)
		publication.AddToInternal("lcp_content_key_algorithm", book.LCP.Encryption.ContentKey.Algorithm)
		publication.AddToInternal("lcp_user_hint", book.LCP.Encryption.UserKey.TextHint)
		publication.AddToInternal("lcp_user_key_check", decodedKeyCheck)
		publication.AddLink("application/vnd.readium.lcp.license-1.0+json", []string{"license"}, "license.lcpl", false)

	}

}

// FilePath return the complete path for the ressource
func FilePath(publication models.Publication, publicationResource string) string {
	var rootFile string

	for _, data := range publication.Internal {
		if data.Name == "rootfile" {
			rootFile = data.Value.(string)
		}
	}

	return path.Join(path.Dir(rootFile), publicationResource)
}

func fillSubject(publication *models.Publication, book *epub.Epub) {
	for _, s := range book.Opf.Metadata.Subject {
		sub := models.Subject{Name: s.Data, Code: s.Term, Scheme: s.Authority}
		publication.Metadata.Subject = append(publication.Metadata.Subject, sub)
	}

}

func fillMediaOverlay(publication *models.Publication, book *epub.Epub) {
	var smil epub.SMIL

	for _, item := range publication.Resources {
		if item.TypeLink == "application/smil+xml" {
			mo := models.MediaOverlayNode{}
			if book == nil {
				fd, _, _ := fetcher.Fetch(publication, item.Href)
				dec := xml.NewDecoder(fd)
				dec.Decode(&smil)
			} else {
				smil = book.GetSMIL(item.Href)
			}
			mo.Role = append(mo.Role, "section")
			mo.AddHrefAbsolute(smil.Body.TextRef, item.Href)
			if len(smil.Body.Par) > 0 {
				for _, par := range smil.Body.Par {
					p := models.MediaOverlayNode{}
					p.AddHrefAbsolute(par.Text.Src, item.Href)
					p.AddAudioAbsolute(par.Audio.Src, item.Href)
					mo.Children = append(mo.Children, p)
				}
			}

			if len(smil.Body.Seq) > 0 {
				for _, s := range smil.Body.Seq {
					addSeqToMediaOverlay(publication, &mo.Children, s, mo.Text, item.Href)
				}
			}

			baseHref := strings.Split(mo.Text, "#")[0]
			link := findLinKByHref(publication, baseHref, item.Href)
			link.MediaOverlays = append(link.MediaOverlays, mo)
			if link.Properties == nil {
				link.Properties = &models.Properties{MediaOverlay: mediaOverlayURL + link.Href}
			} else {
				link.Properties.MediaOverlay = mediaOverlayURL + link.Href
			}
		}
	}
}

func addSeqToMediaOverlay(publication *models.Publication, mo *[]models.MediaOverlayNode, seq epub.Seq, href string, smilHref string) {

	moc := models.MediaOverlayNode{}
	moc.Role = append(moc.Role, "section")
	moc.AddHrefAbsolute(seq.TextRef, smilHref)

	if len(seq.Par) > 0 {
		for _, par := range seq.Par {
			p := models.MediaOverlayNode{}
			p.AddHrefAbsolute(par.Text.Src, smilHref)
			p.AddAudioAbsolute(par.Audio.Src, smilHref)
			if par.Audio.ClipBegin != "" && par.Audio.ClipEnd != "" {
				p.Audio += "#t="
				p.Audio += smilTimeToSeconds(par.Audio.ClipBegin)
				p.Audio += ","
				p.Audio += smilTimeToSeconds(par.Audio.ClipEnd)
			}
			moc.Children = append(moc.Children, p)
		}
	}

	if len(seq.Seq) > 0 {
		for _, s := range seq.Seq {
			addSeqToMediaOverlay(publication, &moc.Children, s, moc.Text, smilHref)
		}
	}
	baseHref := strings.Split(moc.Text, "#")[0]
	baseHrefParent := strings.Split(href, "#")[0]
	if baseHref == baseHrefParent {
		*mo = append(*mo, moc)
	} else {
		link := findLinKByHref(publication, baseHref, smilHref)
		link.MediaOverlays = append(link.MediaOverlays, moc)
		if link.Properties == nil {
			link.Properties = &models.Properties{MediaOverlay: mediaOverlayURL + link.Href}
		} else {
			link.Properties.MediaOverlay = mediaOverlayURL + link.Href
		}
	}

}

func smilTimeToSeconds(smilTime string) string {

	if strings.Contains(smilTime, "h") {
		hArr := strings.Split(strings.Replace(smilTime, "h", "", 1), ".")
		timeCount := 0
		hour, _ := strconv.Atoi(hArr[0])
		timeCount += hour * 60 * 60
		min, _ := strconv.Atoi(hArr[1])
		minConv := min * 60 / 100
		timeCount += minConv * 60
		return strconv.Itoa(timeCount)
	} else if strings.Contains(smilTime, "ms") {
		ms, _ := strconv.Atoi(strings.Replace(smilTime, "ms", "", 1))
		if ms < 1000 {
			return "0." + strings.Replace(smilTime, "ms", "", 1)
		}
		res := strconv.FormatFloat(float64(ms)/1000, 'f', -1, 32)
		return res
	} else if strings.Contains(smilTime, "s") {
		return strings.Replace(smilTime, "s", "", 1)
	}

	tArr := strings.Split(smilTime, ":")
	switch len(tArr) {
	case 1:
		return smilTime
	case 2:
		sArr := strings.Split(tArr[1], ".")
		if len(sArr) > 1 {
			timeCount := 0
			min, _ := strconv.Atoi(tArr[0])
			timeCount += min * 60
			sec, _ := strconv.Atoi(sArr[0])
			timeCount += sec
			return strconv.Itoa(timeCount) + "." + sArr[1]
		}
		timeCount := 0
		min, _ := strconv.Atoi(tArr[0])
		timeCount += min * 60
		sec, _ := strconv.Atoi(tArr[1])
		timeCount += sec
		return strconv.Itoa(timeCount)
	case 3:
		sArr := strings.Split(tArr[2], ".")
		if len(sArr) > 1 {
			timeCount := 0
			hour, _ := strconv.Atoi(tArr[0])
			timeCount += hour * 60 * 60
			min, _ := strconv.Atoi(tArr[1])
			timeCount += min * 60
			sec, _ := strconv.Atoi(sArr[0])
			timeCount += sec
			return strconv.Itoa(timeCount) + "." + sArr[1]
		}
		timeCount := 0
		hour, _ := strconv.Atoi(tArr[0])
		timeCount += hour * 60 * 60
		min, _ := strconv.Atoi(tArr[1])
		timeCount += min * 60
		sec, _ := strconv.Atoi(sArr[0])
		timeCount += sec
		return strconv.Itoa(timeCount)
	}

	return ""
}

func fillPublicationDate(publication *models.Publication, book *epub.Epub) {
	var date time.Time
	var err error

	if len(book.Opf.Metadata.Date) > 0 {

		if isEpub3OrMore(book) {
			if strings.Contains(book.Opf.Metadata.Date[0].Data, "T") {
				date, err = time.Parse(time.RFC3339, book.Opf.Metadata.Date[0].Data)
			} else {
				date, err = time.Parse("2006-01-02", book.Opf.Metadata.Date[0].Data)
			}
			if err == nil {
				publication.Metadata.PublicationDate = &date
				return
			}
		}
		for _, da := range book.Opf.Metadata.Date {
			if strings.Contains(da.Event, "publication") {
				count := strings.Count(da.Data, "-")
				switch count {
				case 0:
					date, err = time.Parse("2006", da.Data)
				case 1:
					date, err = time.Parse("2006-01", da.Data)
				case 2:
					date, err = time.Parse("2006-01-02", da.Data)
				}
				if err == nil {
					publication.Metadata.PublicationDate = &date
					return
				}
			}
		}

	}
}

func getEpubVersion(book *epub.Epub) string {

	if book.Container.Rootfile.Version != "" {
		return book.Container.Rootfile.Version
	} else if book.Opf.Version != "" {
		return book.Opf.Version
	}

	return ""
}

func isEpub3OrMore(book *epub.Epub) bool {

	version := getEpubVersion(book)
	if version == epub3 || version == epub31 {
		return true
	}

	return false
}

func findLinKByHref(publication *models.Publication, href string, rootFile string) *models.Link {
	if href == "" {
		return &models.Link{}
	}

	for i, l := range publication.ReadingOrder {
		if l.Href == href {
			return &publication.ReadingOrder[i]
		}
	}

	return &models.Link{}
}
