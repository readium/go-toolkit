package streamer

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/parser"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/parser/pdf"
	"github.com/readium/go-toolkit/pkg/pub"
)

// Streamer opens a `Publication` using a list of parsers.
//
// The `Streamer` is configured to use Readium's default parsers, which you can
// bypass using `Config.IgnoreDefaultParsers`. However, you can provide
// additional `Config.Parsers` which will take precedence over the default
// ones. This can also be used to provide an alternative configuration of a
// default parser.
type Streamer struct {
	parsers           []parser.PublicationParser
	inferA11yMetadata InferA11yMetadata
	inferPageCount    bool
	archiveFactory    archive.ArchiveFactory
	// TODO pdfFactory
	httpClient *http.Client
	// onCreatePublication
}

type Config struct {
	Parsers              []parser.PublicationParser // Parsers used to open a publication, in addition to the default parsers.
	IgnoreDefaultParsers bool                       // When true, only parsers provided in parsers will be used.
	InferA11yMetadata    InferA11yMetadata          // When not empty, additional accessibility metadata will be infered from the manifest.
	InferPageCount       bool                       // When true, will infer `Metadata.NumberOfPages` from the generated position list.
	ArchiveFactory       archive.ArchiveFactory     // Opens an archive (e.g. ZIP, RAR), optionally protected by credentials.
	HttpClient           *http.Client               // Service performing HTTP requests.
}

type InferA11yMetadata uint8

const (
	// No accessibility metadata will be infered.
	InferA11yMetadataNo InferA11yMetadata = 0 + iota
	// Accessibility metadata will be infered from the manifest and merged in
	// the `Accessibility` object.
	InferA11yMetadataMerged
	// Accessibility metadata will be infered from the manifest and added
	// separately in the `InferredAccessibility` object.
	InferA11yMetadataSplit
)

func New(config Config) Streamer { // TODO contentProtections
	if config.HttpClient == nil {
		config.HttpClient = http.DefaultClient
	}
	if config.ArchiveFactory == nil {
		config.ArchiveFactory = archive.NewArchiveFactory()
	}

	defaultParsers := []parser.PublicationParser{
		epub.NewParser(nil), // TODO pass strategy
		pdf.NewParser(),
		parser.NewWebPubParser(config.HttpClient),
		parser.ImageParser{},
		parser.AudioParser{},
	}

	if !config.IgnoreDefaultParsers {
		config.Parsers = append(config.Parsers, defaultParsers...)
	}

	return Streamer{
		parsers:           config.Parsers,
		inferA11yMetadata: config.InferA11yMetadata,
		inferPageCount:    config.InferPageCount,
		archiveFactory:    config.ArchiveFactory,
		httpClient:        config.HttpClient,
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

	pub := builder.Build()

	s.inferA11yMetadataInPublication(pub)

	if s.inferPageCount && pub.Manifest.Metadata.NumberOfPages == nil {
		pageCount := uint(len(pub.Positions()))
		if pageCount > 0 {
			pub.Manifest.Metadata.NumberOfPages = &pageCount
		}
	}

	return pub, nil
}

func (s *Streamer) inferA11yMetadataInPublication(pub *pub.Publication) {
	if s.inferA11yMetadata == InferA11yMetadataNo {
		return
	}
	inferredA11y := inferA11yMetadataFromManifest(pub.Manifest)
	if inferredA11y == nil {
		return
	}

	switch s.inferA11yMetadata {
	case InferA11yMetadataMerged:
		if pub.Manifest.Metadata.Accessibility == nil {
			pub.Manifest.Metadata.Accessibility = inferredA11y
		} else {
			pub.Manifest.Metadata.Accessibility.Merge(inferredA11y)
		}

	case InferA11yMetadataSplit:
		pub.Manifest.Metadata.SetOtherMetadata(manifest.InferredAccessibilityMetadataKey, inferredA11y)

	case InferA11yMetadataNo:
		return
	}
}
