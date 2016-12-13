package fetcher

import (
	"io"
	"path"

	"github.com/feedbooks/webpub-streamer/models"
)

// List TODO add doc
type List struct {
	publicationType string
	fetcher         (func(models.Publication, string) (io.ReadSeeker, string))
}

var fetcherList []List

// Fetch TODO add doc
func Fetch(publication models.Publication, publicationRessource string) (io.ReadSeeker, string) {
	var typePublication string

	for _, key := range publication.Internal {
		if key.Name == "type" {
			typePublication = key.Value.(string)
		}
	}

	if typePublication != "" {
		for _, fetcherFunc := range fetcherList {
			if typePublication == fetcherFunc.publicationType {
				return fetcherFunc.fetcher(publication, publicationRessource)
			}
		}
	}

	return nil, ""
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
