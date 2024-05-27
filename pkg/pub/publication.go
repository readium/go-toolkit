package pub

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// The Publication shared model is the entrypoint for all the metadata and services related to a Readium publication.
type Publication struct {
	Manifest manifest.Manifest // The manifest holding the publication metadata extracted from the publication file.
	Fetcher  fetcher.Fetcher   // The underlying fetcher used to read publication resources.
	// TODO servicesBuilder
	// TODO positionsFactory
	services map[string]Service
}

// Returns whether this publication conforms to the given Readium Web Publication Profile.
func (p Publication) ConformsTo(profile manifest.Profile) bool {
	return p.Manifest.ConformsTo(profile)
}

// Finds the first [Link] with the given href in the publication's links.
// Searches through (in order) the reading order, resources and links recursively following alternate and children links.
// If there's no match, tries again after removing any query parameter and anchor from the given href.
func (p Publication) LinkWithHref(href string) *manifest.Link {
	return p.Manifest.LinkWithHref(href)
}

// Finds the first [Link] having the given [rel] in the publications's links.
func (p Publication) LinkWithRel(rel string) *manifest.Link {
	return p.Manifest.LinkWithRel(rel)
}

// Finds all [Link]s having the given [rel] in the publications's links.
func (p Publication) LinksWithRel(rel string) []manifest.Link {
	return p.Manifest.LinksWithRel(rel)
}

// Creates a new [Locator] object from a [Link] to a resource of this publication.
// Returns nil if the resource is not found in this publication.
func (p Publication) LocatorFromLink(link manifest.Link) *manifest.Locator {
	return p.Manifest.LocatorFromLink(link)
}

// Returns the RWPM JSON representation for this [Publication]'s manifest, as a string.
func (p Publication) JSONManifest() (string, error) {
	bin, err := json.Marshal(p.Manifest)
	if err != nil {
		return "", err
	}
	return string(bin), nil
}

func (p Publication) PositionsFromManifest() []manifest.Locator {
	// TODO just access the service directly and don't marshal and unmarshal JSON?
	data, err := p.Get(PositionsLink).ReadAsJSON()
	if err != nil || data == nil {
		return []manifest.Locator{}
	}
	rawPositions, ok := data["positions"]
	if !ok {
		return []manifest.Locator{}
	}
	positions, ok := rawPositions.([]map[string]interface{})
	locators := make([]manifest.Locator, len(positions))
	for i, rl := range positions {
		locator, _ := manifest.LocatorFromJSON(rl)
		locators[i] = locator
	}
	return locators
}

func (p Publication) PositionsByReadingOrder() [][]manifest.Locator {
	service := p.FindService(PositionsService_Name)
	if service == nil {
		return nil
	}
	return service.(PositionsService).PositionsByReadingOrder()
}

func (p *Publication) Positions() []manifest.Locator {
	service := p.FindService(PositionsService_Name)
	if service == nil {
		return nil
	}
	return service.(PositionsService).Positions()
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

	if !strings.HasPrefix(link.Href, "/") {
		link.Href = "/" + link.Href
	}
	return link
}

func (p Publication) FindService(serviceName string) Service {
	for k, v := range p.services {
		if k != serviceName {
			continue
		}
		return v
	}
	return nil
}

func (p Publication) FindServices(serviceName string) []Service {
	var services []Service
	for k, v := range p.services {
		if k != serviceName {
			continue
		}
		services = append(services, v)
	}
	return services
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

func New(m manifest.Manifest, f fetcher.Fetcher, b *ServicesBuilder) *Publication {
	if b == nil {
		b = NewServicesBuilder(nil)
	}
	newManifest := m                                // Make a copy of the manifest
	services := b.Build(NewContext(newManifest, f)) // Build the services

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

func NewBuilder(m manifest.Manifest, f fetcher.Fetcher, b *ServicesBuilder) *Builder {
	if b == nil {
		b = NewServicesBuilder(nil)
	}
	return &Builder{
		Manifest:        m,
		Fetcher:         f,
		ServicesBuilder: *b,
	}
}

type Builder struct {
	Manifest        manifest.Manifest
	Fetcher         fetcher.Fetcher
	ServicesBuilder ServicesBuilder
}

func (b Builder) Build() *Publication {
	return New(b.Manifest, b.Fetcher, &b.ServicesBuilder)
}
