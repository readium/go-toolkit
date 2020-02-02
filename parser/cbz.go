package parser

import (
	"archive/zip"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/readium/r2-streamer-go/models"
	"github.com/readium/r2-streamer-go/parser/comicrack"
)

func init() {
	parserList = append(parserList, List{fileExt: "cbz", parser: CbzParser, callback: CbzCallback})
}

// CbzParser TODO add doc
func CbzParser(filePath string) (models.Publication, error) {
	var publication models.Publication

	publication.Metadata.Identifier = filePath
	publication.Context = append(publication.Context, "https://readium.org/webpub-manifest/context.jsonld")
	publication.Metadata.RDFType = "http://schema.org/ComicIssue"

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
			publication.ReadingOrder = append(publication.ReadingOrder, linkItem)
		}
		if f.Name == "ComicInfo.xml" {
			fd, _ := f.Open()
			defer fd.Close()
			comicRackMetadata(&publication, fd)
		}
	}

	if publication.Metadata.Title.String() == "" {
		publication.Metadata.Title.SingleString = filePathToTitle(filePath)
	}

	return publication, nil
}

// CbzCallback empty function to respect interface
func CbzCallback(publication *models.Publication) {

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

func comicRackMetadata(publication *models.Publication, fd io.ReadCloser) {

	meta := comicrack.Parse(fd)
	if meta.Writer != "" {
		cont := models.Contributor{Name: models.MultiLanguage{SingleString: meta.Writer}}
		publication.Metadata.Author = append(publication.Metadata.Author, cont)
	}
	if meta.Penciller != "" {
		cont := models.Contributor{Name: models.MultiLanguage{SingleString: meta.Writer}}
		publication.Metadata.Penciler = append(publication.Metadata.Penciler, cont)
	}
	if meta.Colorist != "" {
		cont := models.Contributor{Name: models.MultiLanguage{SingleString: meta.Writer}}
		publication.Metadata.Colorist = append(publication.Metadata.Colorist, cont)
	}
	if meta.Inker != "" {
		cont := models.Contributor{Name: models.MultiLanguage{SingleString: meta.Writer}}
		publication.Metadata.Inker = append(publication.Metadata.Inker, cont)
	}

	if meta.Title != "" {
		publication.Metadata.Title.SingleString = meta.Title
	}

	if publication.Metadata.Title.String() == "" {
		if meta.Series != "" {
			title := meta.Series
			if meta.Number != 0 {
				title = title + " - " + strconv.Itoa(meta.Number)
			}
			publication.Metadata.Title.SingleString = title
		}
	}

	if len(meta.Pages) > 0 {
		for _, p := range meta.Pages {
			l := models.Link{}
			if p.Type == "FrontCover" {
				l.AddRel("cover")
			}
			l.Href = publication.ReadingOrder[p.Image].Href
			if p.ImageHeight != 0 {
				l.Height = p.ImageHeight
			}
			if p.ImageWidth != 0 {
				l.Width = p.ImageWidth
			}
			if p.Bookmark != "" {
				l.Title = p.Bookmark
			}
			publication.TOC = append(publication.TOC, l)

		}
	}

}
