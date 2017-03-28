package fetcher

import (
	"errors"
	"io"
	"path"

	"github.com/readium/r2-streamer-go/models"
)

// List TODO add doc
type List struct {
	publicationType string
	fetcher         (func(*models.Publication, string) (io.ReadSeeker, string, error))
}

var fetcherList []List

// Fetch TODO add doc
func Fetch(publication *models.Publication, publicationRessource string) (io.ReadSeeker, string, error) {
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

	return nil, "", errors.New("can't find fetcher")
}

// FilePath return the complete path for the ressource
func FilePath(publication *models.Publication, publicationResource string) string {
	var rootFile string

	for _, data := range publication.Internal {
		if data.Name == "rootfile" {
			rootFile = data.Value.(string)
		}
	}

	return path.Join(path.Dir(rootFile), publicationResource)
}
