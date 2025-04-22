package asset

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Represents a publication stored as a file on the local file system.
type FileAsset struct {
	uri            url.URL
	mediatype      *mediatype.MediaType
	knownMediaType *mediatype.MediaType
	mediaTypeHint  string
}

func File(uri url.URL) *FileAsset {
	return &FileAsset{
		uri: uri,
	}
}

// Creates a [FileAsset] from a [File] and an optional media type, when known.
func FileWithMediaType(uri url.URL, mediatype *mediatype.MediaType) *FileAsset {
	return &FileAsset{
		uri:            uri,
		knownMediaType: mediatype,
	}
}

// Creates a [FileAsset] from a [File] and an optional media type hint.
// Providing a media type hint will improve performances when sniffing the media type.
func FileWithMediaTypeHint(uri url.URL, mediatypeHint string) *FileAsset {
	return &FileAsset{
		uri:           uri,
		mediaTypeHint: mediatypeHint,
	}
}

// Name implements PublicationAsset
func (a *FileAsset) Name() string {
	return a.uri.Filename()
}

func (a *FileAsset) realPath() string {
	return filepath.ToSlash(a.uri.Path())
}

// MediaType implements PublicationAsset
func (a *FileAsset) MediaType(ctx context.Context) mediatype.MediaType {
	if a.mediatype == nil {
		if a.knownMediaType != nil {
			a.mediatype = a.knownMediaType
		} else {
			fil, err := os.Open(a.realPath())
			if err == nil { // No problem opening the file
				defer fil.Close()
				a.mediatype = mediatype.OfFile(ctx, fil, []string{a.mediaTypeHint}, nil, mediatype.Sniffers)
			}
			if a.mediatype == nil { // Still nothing found
				a.mediatype = &mediatype.Binary
			}
		}
	}
	return *a.mediatype
}

// CreateFetcher implements PublicationAsset
func (a *FileAsset) CreateFetcher(ctx context.Context, dependencies Dependencies, credentials string) (fetcher.Fetcher, error) {
	if u, ok := a.uri.(url.AbsoluteURL); ok && !u.IsFile() {
		return nil, errors.New("file asset with absolute URL must have file:/// scheme")
	}

	rfp := a.realPath()
	stat, err := os.Stat(rfp)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return fetcher.NewFileFetcher("", rfp), nil
	} else {
		factory, ok := dependencies.ArchiveFactory.(archive.SchemeSpecificArchiveFactory)
		if ok {
			// If we can narrow down the schemes the factory supports, we enforce it
			if !factory.CanOpen(url.SchemeFile) {
				return nil, errors.New("provided ArchiveFactory does not support file scheme")
			}
		}

		af, err := fetcher.NewArchiveFetcherFromPathWithFactory(ctx, rfp, dependencies.ArchiveFactory)
		if err == nil {
			return af, nil
		}

		return fetcher.NewFileFetcher(a.Name(), rfp), nil
	}
}
