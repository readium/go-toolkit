package pub

import (
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

const (
	ContentProtectionService_Name = "ContentProtectionService"
	CoverService_Name             = "CoverService"
	LocatorService_Name           = "LocatorService"
	PositionsService_Name         = "PositionsService"
	SearchService_Name            = "SearchService"
	ContentService_Name           = "ContentService"
)

// Base interface to be implemented by all publication services.
type Service interface {
	Links() manifest.LinkList                        // Links to be added to the publication
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

/*
contentProtection ServiceFactory,
	cover ServiceFactory,
	locator ServiceFactory,
	positions ServiceFactory,
	search ServiceFactory,
*/

func NewServicesBuilder(fcs map[string]ServiceFactory) *ServicesBuilder {
	if fcs == nil {
		fcs = map[string]ServiceFactory{}
	}

	// TODO DefaultLocatorService(it.manifest.readingOrder, it.publication) if LocatorService_Name doesn't exist

	return &ServicesBuilder{
		serviceFactories: fcs,
	}
}

// Builds the actual list of publication services to use in a Publication.
func (s *ServicesBuilder) Build(context Context) map[string]Service {
	services := make(map[string]Service, len(s.serviceFactories))
	for k, v := range s.serviceFactories {
		if v != nil {
			services[k] = v(context)
		}
	}
	return services
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
