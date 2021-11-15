package asset

import (
	"os"
	"path/filepath"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

// Represents a publication stored as a file on the local file system.
type FileAsset struct {
	filepath       string
	mediatype      *mediatype.MediaType
	knownMediaType *mediatype.MediaType
	mediaTypeHint  string
}

func File(filepath string) *FileAsset {
	return &FileAsset{
		filepath: filepath,
	}
}

// Creates a [FileAsset] from a [File] and an optional media type, when known.
func FileWithMediaType(filepath string, mediatype *mediatype.MediaType) *FileAsset {
	return &FileAsset{
		filepath:       filepath,
		knownMediaType: mediatype,
	}
}

// Creates a [FileAsset] from a [File] and an optional media type hint.
// Providing a media type hint will improve performances when sniffing the media type.
func FileWithMediaTypeHint(filepath string, mediatypeHint string) *FileAsset {
	return &FileAsset{
		filepath:      filepath,
		mediaTypeHint: mediatypeHint,
	}
}

func (a *FileAsset) Name() string {
	return filepath.Base(a.filepath)
}

func (a *FileAsset) MediaType() mediatype.MediaType {
	if a.mediatype == nil {
		if a.knownMediaType != nil {
			a.mediatype = a.knownMediaType
		} else {
			fil, err := os.Open(a.filepath)
			if err == nil { // No problem opening the file
				defer fil.Close()
				a.mediatype = mediatype.OfFile(fil, []string{a.mediaTypeHint}, nil, mediatype.Sniffers)
			}
			if a.mediatype == nil { // Still nothing found
				a.mediatype = &mediatype.BINARY
			}
		}
	}
	return *a.mediatype
}

func (a *FileAsset) CreateFetcher(dependencies Dependencies, credentials string) (fetcher.Fetcher, error) {
	stat, err := os.Stat(a.filepath)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return fetcher.NewFileFetcher("/", a.filepath), nil
	} else {
		af, err := fetcher.NewArchiveFetcherFromPath(a.filepath)
		if err == nil {
			return af, nil
		}
		return fetcher.NewFileFetcher("/"+a.Name(), a.filepath), nil
	}
}
