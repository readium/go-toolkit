package fetcher

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/feedbooks/r2-streamer-go/models"
)

func init() {
	fetcherList = append(fetcherList, List{publicationType: "epub_dir", fetcher: FetchEpubDir})
}

// FetchEpubDir TODO add doc
func FetchEpubDir(publication models.Publication, publicationResource string) (io.ReadSeeker, string, error) {
	var mediaType string
	var basePath string
	var rootFile string

	for _, data := range publication.Internal {
		if data.Name == "basepath" {
			basePath = data.Value.(string)
		}
		if data.Name == "rootfile" {
			rootFile = data.Value.(string)
		}
	}

	filePath := path.Join(path.Join(basePath+"/", path.Dir(rootFile)), publicationResource)
	fd, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
	}

	return fd, mediaType, nil
}
