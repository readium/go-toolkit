package pub

import (
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// The Publication shared model is the entrypoint for all the metadata and services related to a Readium publication.
type Publication struct {
	manifest manifest.Manifest // The manifest holding the publication metadata extracted from the publication file.
	fetcher  fetcher.Fetcher   // The underlying fetcher used to read publication resources.
	// TODO servicesBuilder
	// TODO positionsFactory
	// TODO services []Service
	_manifest manifest.Manifest
}

func InitPublication() *Publication {
	return &Publication{}
}

func NewBuilder(m manifest.Manifest, f fetcher.Fetcher) *Builder {
	return &Builder{
		manifest: m,
		fetcher:  f,
		// TODO servicesBuilder
	}
}

type Builder struct {
	manifest manifest.Manifest
	fetcher  fetcher.Fetcher
	// TODO servicesBuilder
}

func (b Builder) Build() Publication {
	return Publication{
		manifest: b.manifest,
		fetcher:  b.fetcher,
	}
}
