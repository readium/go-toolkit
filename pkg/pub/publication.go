package pub

import (
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// The Publication shared model is the entrypoint for all the metadata and services related to a Readium publication.
type Publication struct {
	Manifest manifest.Manifest // The manifest holding the publication metadata extracted from the publication file.
	Fetcher  fetcher.Fetcher   // The underlying fetcher used to read publication resources.
	// TODO servicesBuilder
	// TODO positionsFactory
	// TODO services []Service
	_manifest manifest.Manifest
}

// Returns whether this publication conforms to the given Readium Web Publication Profile.
func (p Publication) ConformsTo(profile manifest.Profile) bool {
	return p.Manifest.ConformsTo(profile)
}

func (p Publication) Close() {
	p.Fetcher.Close()
}

func New() *Publication {
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

func (b Builder) Build() *Publication {
	return &Publication{
		Manifest: b.manifest,
		Fetcher:  b.fetcher,
	}
}
