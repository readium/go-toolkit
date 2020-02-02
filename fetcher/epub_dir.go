package fetcher

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/readium/r2-streamer-go/decoder"
	"github.com/readium/r2-streamer-go/models"
)

func init() {
	fetcherList = append(fetcherList, List{publicationType: "epub_dir", fetcher: FetchEpubDir})
}

// FetchEpubDir TODO add doc
func FetchEpubDir(publication *models.Publication, publicationResource string) (io.ReadSeeker, string, error) {
	var mediaType string
	var basePath string
	var rootFile string
	var link models.Link

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

	for _, linkRes := range publication.Resources {
		if publicationResource == linkRes.Href {
			link = linkRes
		}
	}

	for _, linkRes := range publication.ReadingOrder {
		if publicationResource == linkRes.Href {
			link = linkRes
		}
	}

	if decoder.NeedToDecode(publication, link) {
		readerSeekerDecode, err := decoder.Decode(publication, link, fd)
		if err != nil {
			fmt.Println(err)
			return nil, "", err
		}
		return readerSeekerDecode, mediaType, nil
	}
	return fd, mediaType, nil
}
