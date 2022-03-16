package pub

import (
	"encoding/json"
	"path"

	"github.com/jinzhu/copier"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/service"
)

// The Publication shared model is the entrypoint for all the metadata and services related to a Readium publication.
type Publication struct {
	Manifest manifest.Manifest // The manifest holding the publication metadata extracted from the publication file.
	Fetcher  fetcher.Fetcher   // The underlying fetcher used to read publication resources.
	// TODO servicesBuilder
	// TODO positionsFactory
	services []service.Service
}

// Returns whether this publication conforms to the given Readium Web Publication Profile.
func (p Publication) ConformsTo(profile manifest.Profile) bool {
	return p.Manifest.ConformsTo(profile)
}

// Returns the RWPM JSON representation for this [Publication]'s manifest, as a string.
func (p Publication) JSONManifest() (string, error) {
	bin, err := json.Marshal(p.Manifest)
	if err != nil {
		return "", err
	}
	return string(bin), nil
}

// The URL where this publication is served, computed from the [Link] with `self` relation.
func (p Publication) BaseURL() *string {
	lnk := p.Manifest.Links.FirstWithRel("self")
	if lnk == nil {
		return nil
	}
	dir := path.Dir(lnk.Href)
	return &dir
}

// Returns the first existing link matching the given [path].
func (p Publication) Find(path string) *manifest.Link {
	link := p.Manifest.Links.FirstWithHref(path)
	if link == nil {
		link = p.Manifest.ReadingOrder.FirstWithHref(path)
		if link == nil {
			link = p.Manifest.Resources.FirstWithHref(path)
			if link == nil {
				return nil
			}
		}
	}

	link.Href = "/" + link.Href
	return link
}

// Returns the resource targeted by the given non-templated [link].
func (p Publication) Get(link manifest.Link) fetcher.Resource {
	for _, service := range p.services {
		if l, ok := service.Get(link); ok {
			return l
		}
	}
	return p.Fetcher.Get(link)
}

// Free up resources associated with the publication
func (p Publication) Close() {
	p.Fetcher.Close()
	for _, service := range p.services {
		service.Close()
	}
}

func New(m manifest.Manifest, f fetcher.Fetcher, b *service.ServicesBuilder) *Publication {
	if b == nil {
		b = service.NewBuilder(nil, nil, nil, nil, nil)
	}
	services := b.Build(service.NewContext(m, f)) // Build the services
	var newManifest manifest.Manifest
	copier.Copy(&newManifest, &m) // Make a deep copy of the manifest

	// Add links from the services to the manifest links
	for _, v := range services {
		lnks := v.Links()
		if len(lnks) > 0 {
			newManifest.Links = append(newManifest.Links, lnks...)
		}
	}

	return &Publication{
		Manifest: newManifest,
		Fetcher:  f,
		services: services,
	}
}

func NewBuilder(m manifest.Manifest, f fetcher.Fetcher, b *service.ServicesBuilder) *Builder {
	if b == nil {
		b = service.NewBuilder(nil, nil, nil, nil, nil)
	}
	return &Builder{
		manifest:        m,
		fetcher:         f,
		servicesBuilder: *b,
	}
}

type Builder struct {
	manifest        manifest.Manifest
	fetcher         fetcher.Fetcher
	servicesBuilder service.ServicesBuilder
}

func (b Builder) Build() *Publication {
	return New(b.manifest, b.fetcher, &b.servicesBuilder)
}
