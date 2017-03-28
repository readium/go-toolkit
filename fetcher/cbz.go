package fetcher

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"

	"github.com/readium/r2-streamer-go/models"
)

func init() {
	fetcherList = append(fetcherList, List{publicationType: "cbz", fetcher: FetchCbz})
}

// FetchCbz TODO add doc
func FetchCbz(publication *models.Publication, publicationResource string) (io.ReadSeeker, string, error) {
	var mediaType string
	var reader *zip.ReadCloser
	var assetFd io.ReadCloser

	for _, data := range publication.Internal {
		if data.Name == "cbz" {
			reader = data.Value.(*zip.ReadCloser)
		}
	}

	resourcePath := FilePath(publication, publicationResource)
	for _, f := range reader.File {
		if f.Name == resourcePath {
			assetFd, _ = f.Open()
		}
	}

	buff, _ := ioutil.ReadAll(assetFd)
	assetFd.Close()
	readerSeeker := bytes.NewReader(buff)

	return readerSeeker, mediaType, nil
}
