package streamer

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/parser"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
)

type Streamer struct {
	parsers        []parser.PublicationParser
	archiveFactory archive.ArchiveFactory
	// TODO pdfFactory
	httpClient *http.Client
	// onCreatePublication
}

type Config struct {
	Parsers              []parser.PublicationParser
	IgnoreDefaultParsers bool
	ArchiveFactory       archive.ArchiveFactory
	HttpClient           *http.Client
}

func New(config Config) Streamer { // TODO contentProtections
	if config.HttpClient == nil {
		config.HttpClient = http.DefaultClient
	}
	if config.ArchiveFactory == nil {
		config.ArchiveFactory = archive.NewArchiveFactory()
	}

	defaultParsers := []parser.PublicationParser{
		epub.NewParser(nil),
		// TODO PDF parser
		parser.NewWebPubParser(config.HttpClient),
		parser.ImageParser{},
		parser.AudioParser{},
	}

	if !config.IgnoreDefaultParsers {
		config.Parsers = append(config.Parsers, defaultParsers...)
	}

	return Streamer{
		parsers:        config.Parsers,
		archiveFactory: config.ArchiveFactory,
		httpClient:     config.HttpClient,
	}
}

// Parses a [Publication] from the given asset.
func (s Streamer) Open(a asset.PublicationAsset, credentials string) (*pub.Publication, error) {
	fetcher, err := a.CreateFetcher(asset.Dependencies{
		ArchiveFactory: s.archiveFactory,
	}, credentials)
	if err != nil {
		return nil, err
	}

	// TODO contentProtections/protectedAsset

	var builder *pub.Builder
	for _, parser := range s.parsers {
		pb, err := parser.Parse(a, fetcher)
		if err != nil {
			fetcher.Close()
			return nil, errors.Wrap(err, "failed parsing asset")
		}
		if pb != nil {
			builder = pb
			break
		}
	}
	if builder == nil {
		fetcher.Close()
		return nil, errors.New("cannot find a parser for this asset")
	}

	// TODO apply onCreatePublication

	// TODO addLegacyProperties

	return builder.Build(), nil
}
