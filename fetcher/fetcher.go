package fetcher

import "github.com/feedbooks/webpub-streamer/models"

// List TODO add doc
type List struct {
	publicationType string
	fetcher         (func(models.Publication, string) (string, string))
}

var fetcherList []List

// Fetch TODO add doc
func Fetch(publication models.Publication, assetName string) (string, string) {

	for _, fetcherFunc := range fetcherList {
		// if fileExt == parserFunc.fileExt {
		return fetcherFunc.fetcher(publication, assetName)
		// }
	}

	return "", ""
}
