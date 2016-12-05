package fetcher

import (
	"bufio"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/feedbooks/webpub-streamer/models"
	"github.com/kapmahc/epub"
)

func init() {
	fetcherList = append(fetcherList, List{publicationType: "epub", fetcher: FetchEpub})
}

// FetchEpub TODO add doc
func FetchEpub(publication models.Publication, assetName string) (string, string) {
	var buff string
	var mediaType string
	var book *epub.Book

	cssInject := ""
	jsInject := ""

	for _, data := range publication.Internal {
		if data.Name == "epub" {
			book = data.Value.(*epub.Book)
		}
	}

	extension := filepath.Ext(assetName)
	if extension == ".css" {
		mediaType = "text/css"
	}
	if extension == ".xml" {
		mediaType = "application/xhtml+xml"
	}
	if extension == ".js" {
		mediaType = "text/javascript"
	}

	assetFd, _ := book.Open(assetName)
	buffByte, _ := ioutil.ReadAll(assetFd)
	buff = string(buffByte)
	buffReader := strings.NewReader(buff)

	finalBuff := ""
	if cssInject != "" || jsInject != "" {
		scanner := bufio.NewScanner(buffReader)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "</head>") {
				headBuff := ""
				if jsInject != "" {
					headBuff += strings.Replace(scanner.Text(), "</head>", "<script src='/"+jsInject+"'></script></head>", 1)
				}
				if cssInject != "" {
					if headBuff == "" {
						headBuff += strings.Replace(scanner.Text(), "</head>", "<link rel='stylesheet' type='text/css' href='/"+cssInject+"'></script></head>", 1)
					} else {
						headBuff = strings.Replace(headBuff, "</head>", "<link rel='stylesheet' type='text/css' href='/"+cssInject+"'></head>", 1)
					}
				}
				if headBuff == "" {
					headBuff = scanner.Text()
				}
				finalBuff += headBuff + "\n"
			} else {
				finalBuff += scanner.Text() + "\n"
			}
		}
	} else {
		finalBuff = buff
	}

	return finalBuff, mediaType
}
