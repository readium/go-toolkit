package fetcher

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/readium/r2-streamer-go/decoder"
	"github.com/readium/r2-streamer-go/models"
)

func init() {
	fetcherList = append(fetcherList, List{publicationType: "epub", fetcher: FetchEpub})
}

// FetchEpub TODO add doc
func FetchEpub(publication models.Publication, publicationResource string) (io.ReadSeeker, string, error) {
	var mediaType string
	var reader *zip.ReadCloser
	var assetFd io.ReadCloser
	var link models.Link

	for _, data := range publication.Internal {
		if data.Name == "epub" {
			reader = data.Value.(*zip.ReadCloser)
		}
	}

	resourcePath := FilePath(publication, publicationResource)
	for _, f := range reader.File {
		if f.Name == resourcePath {
			assetFd, _ = f.Open()
		}
	}

	for _, linkRes := range publication.Resources {
		if publicationResource == linkRes.Href {
			link = linkRes
		}
	}

	for _, linkRes := range publication.Spine {
		if publicationResource == linkRes.Href {
			link = linkRes
		}
	}

	buff, _ := ioutil.ReadAll(assetFd)
	assetFd.Close()
	readerSeeker := bytes.NewReader(buff)

	if decoder.NeedToDecode(publication, link) {
		readerSeekerDecode, err := decoder.Decode(publication, link, readerSeeker)
		if err != nil {
			fmt.Println(err)
			return nil, "", err
		}
		return readerSeekerDecode, mediaType, nil
	}

	return readerSeeker, mediaType, nil
}
