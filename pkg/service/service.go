package service

import (
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// Base interface to be implemented by all publication services.
type Service interface {
	Links() []manifest.Link                          // Links to be added to the publication
	Get(link manifest.Link) (fetcher.Resource, bool) // A service can return a Resource that supplements, replaces or compensates for other links
	Close()                                          // Closes any opened file handles, removes temporary files, etc.
}

// Container for the context from which a service is created.
type Context struct {
	Manifest manifest.Manifest
	Fetcher  fetcher.Fetcher
}

func NewContext(manifest manifest.Manifest, fetcher fetcher.Fetcher) Context {
	return Context{
		Manifest: manifest,
		Fetcher:  fetcher,
	}
}

type ServiceFactory func(context Context) Service

// Builds a list of [Service] from a collection of service factories.
// Provides helpers to manipulate the list of services of a [pub.Publication].
type ServicesBuilder struct {
	serviceFactories map[string]ServiceFactory
}

func NewBuilder(
	contentProtection ServiceFactory,
	cover ServiceFactory,
	locator ServiceFactory,
	positions ServiceFactory,
	search ServiceFactory,
) *ServicesBuilder {
	fcs := map[string]ServiceFactory{}
	if contentProtection != nil {
		fcs[ContentProtectionService_Name] = contentProtection
	}
	if cover != nil {
		fcs[CoverService_Name] = cover
	}
	if locator != nil {
		fcs[LocatorService_Name] = locator
	} else {
		// TODO somehow DefaultLocatorService(it.manifest.readingOrder, it.publication)
	}
	if positions != nil {
		fcs[PositionsService_Name] = positions
	}
	if search != nil {
		fcs[SearchService_Name] = search
	}

	return &ServicesBuilder{
		serviceFactories: fcs,
	}
}

// Builds the actual list of publication services to use in a Publication.
func (s *ServicesBuilder) Build(context Context) []Service {
	list := make([]Service, 0, len(s.serviceFactories))
	for _, v := range s.serviceFactories {
		if v != nil {
			list = append(list, v(context))
		}
	}
	return list
}

// Gets the publication service factory for the given service type.
func (s *ServicesBuilder) Get(name string) *ServiceFactory {
	if v, ok := s.serviceFactories[name]; ok {
		return &v
	}
	return nil
}

// Sets the publication service factory for the given service type.
func (s *ServicesBuilder) Set(name string, factory *ServiceFactory) {
	if name == "" {
		return
	}
	if factory == nil {
		delete(s.serviceFactories, name)
	} else {
		s.serviceFactories[name] = *factory
	}
}

// Removes the service factory producing the given kind of service, if any.
func (s *ServicesBuilder) Remove(name string, factory *ServiceFactory) {
	if name == "" {
		return
	}
	delete(s.serviceFactories, name)
}

// Replaces the service factory associated with the given service type with the result of [transform]
func (s *ServicesBuilder) Decorate(name string, transform func(*ServiceFactory) ServiceFactory) {
	if name == "" {
		return
	}
	v, ok := s.serviceFactories[name]
	if ok {
		s.serviceFactories[name] = transform(&v)
	} else {
		s.serviceFactories[name] = transform(nil)
	}
}
