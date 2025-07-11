package pdf

import (
	"context"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/validate"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

type Parser struct {
}

func NewParser() Parser {
	return Parser{}
}

func init() {
	// Disable this feature of pdfcpu
	model.ConfigPath = "disable"
}

// Parse implements PublicationParser
func (p Parser) Parse(ctx context.Context, asset asset.PublicationAsset, f fetcher.Fetcher) (*pub.Builder, error) {
	fallbackTitle := asset.Name()

	if !asset.MediaType(ctx).Equal(&mediatype.PDF) {
		return nil, nil
	}

	links, err := f.Links(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch links")
	}
	if len(links) == 0 {
		return nil, errors.New("unable to find PDF file: links empty")
	}
	link := links.FirstWithMediaType(&mediatype.PDF)
	if link == nil {
		return nil, errors.New("unable to find PDF file: no matching link found")
	}

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	c, err := pdfcpu.Read(fetcher.NewResourceReadSeeker(f.Get(ctx, *link)), conf)
	if err != nil {
		return nil, errors.Wrap(err, "failed opening PDF")
	}

	// Clean up and prepare document
	validate.XRefTable(c)
	pdfcpu.OptimizeXRefTable(c)
	c.EnsurePageCount()

	m, err := ParseMetadata(c, link)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing PDF metadata")
	}

	// Fallback title
	if m.Metadata.Title() == "" {
		m.Metadata.LocalizedTitle = manifest.NewLocalizedStringFromString(fallbackTitle)
	}

	// Finalize
	builder := pub.NewServicesBuilder(map[pub.ServiceName]pub.ServiceFactory{
		pub.PositionsService_Name: PositionsServiceFactory(),
	})
	return pub.NewBuilder(m, f, builder), nil
}
