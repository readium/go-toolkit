package parser

import (
	"context"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/pub"
)

type PublicationParser interface {
	Parse(ctx context.Context, asset asset.PublicationAsset, f fetcher.Fetcher) (*pub.Builder, error)
}
