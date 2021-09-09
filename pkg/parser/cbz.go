package parser

import (
	"archive/zip"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/readium/go-toolkit/pkg/parser/comicrack"
	"github.com/readium/go-toolkit/pkg/pub"
)

func init() {
	parserList = append(parserList, List{fileExt: "cbz", parser: CbzParser, callback: CbzCallback})
}

// CbzParser TODO add doc
func CbzParser(filePath string) (pub.Manifest, error) {
	var publication pub.Manifest

	publication.Metadata.Identifier = filePath
	publication.Context = append(publication.Context, "https://readium.org/webpub-manifest/context.jsonld")
	publication.Metadata.Type = "http://schema.org/ComicIssue"

	zipReader, err := zip.OpenReader(filePath)
	if err != nil {
		return publication, errors.New("can't open or parse cbz file with err : " + err.Error())
	}

	publication.Internal = append(publication.Internal, pub.Internal{Name: "type", Value: "cbz"})
	publication.Internal = append(publication.Internal, pub.Internal{Name: "cbz", Value: zipReader})

	for _, f := range zipReader.File {
		linkItem := pub.Link{}
		linkItem.Type = getMediaTypeByName(f.Name)
		linkItem.Href = f.Name
		if linkItem.Type != "" {
			publication.ReadingOrder = append(publication.ReadingOrder, linkItem)
		}
		if f.Name == "ComicInfo.xml" {
			fd, _ := f.Open()
			defer fd.Close()
			comicRackMetadata(&publication, fd)
		}
	}

	if publication.Metadata.Title() == "" {
		publication.Metadata.LocalizedTitle.SetDefaultTranslation(filePathToTitle(filePath))
	}

	return publication, nil
}

// CbzCallback empty function to respect interface
func CbzCallback(publication *pub.Manifest) {

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

func comicRackMetadata(publication *pub.Manifest, fd io.ReadCloser) {

	meta := comicrack.Parse(fd)
	if meta.Writer != "" {
		cont := pub.Contributor{LocalizedName: pub.NewLocalizedStringFromString(meta.Writer)}
		publication.Metadata.Authors = append(publication.Metadata.Authors, cont)
	}
	if meta.Penciller != "" {
		cont := pub.Contributor{LocalizedName: pub.NewLocalizedStringFromString(meta.Penciller)}
		publication.Metadata.Pencilers = append(publication.Metadata.Pencilers, cont)
	}
	if meta.Colorist != "" {
		cont := pub.Contributor{LocalizedName: pub.NewLocalizedStringFromString(meta.Colorist)}
		publication.Metadata.Colorists = append(publication.Metadata.Colorists, cont)
	}
	if meta.Inker != "" {
		cont := pub.Contributor{LocalizedName: pub.NewLocalizedStringFromString(meta.Inker)}
		publication.Metadata.Inkers = append(publication.Metadata.Inkers, cont)
	}

	if meta.Title != "" {
		publication.Metadata.LocalizedTitle.SetDefaultTranslation(meta.Title)
	}

	if publication.Metadata.Title() == "" {
		if meta.Series != "" {
			title := meta.Series
			if meta.Number != 0 {
				title = title + " - " + strconv.Itoa(meta.Number)
			}
			publication.Metadata.LocalizedTitle.SetDefaultTranslation(title)
		}
	}

	/*if len(meta.Pages) > 0 {
		for _, p := range meta.Pages {
			l := pub.Link{}
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
	}*/

}
