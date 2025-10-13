package pub

import (
	"context"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

type ServiceName string

const (
	ContentProtectionService_Name ServiceName = "ContentProtectionService"
	CoverService_Name             ServiceName = "CoverService"
	LocatorService_Name           ServiceName = "LocatorService"
	PositionsService_Name         ServiceName = "PositionsService"
	SearchService_Name            ServiceName = "SearchService"
	ContentService_Name           ServiceName = "ContentService"
	GuidedNavigationService_Name  ServiceName = "GuidedNavigationService"
)

// Base interface to be implemented by all publication services.
type Service interface {
	Links() manifest.LinkList                                             // Links to be added to the publication
	Get(ctx context.Context, link manifest.Link) (fetcher.Resource, bool) // A service can return a Resource that supplements, replaces or compensates for other links
	Close()                                                               // Closes any opened file handles, removes temporary files, etc.
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

type ServiceFactory func(context Context, public bool) Service

// Builds a list of [Service] from a collection of service factories.
// Provides helpers to manipulate the list of services of a [pub.Publication].
type ServicesBuilder struct {
	serviceFactories map[ServiceName]ServiceFactory
	publicFlags      map[ServiceName]bool
}

/*
contentProtection ServiceFactory,
	cover ServiceFactory,
	search ServiceFactory,
*/

func NewServicesBuilder(fcs map[ServiceName]ServiceFactory) *ServicesBuilder {
	if fcs == nil {
		fcs = map[ServiceName]ServiceFactory{}
	}

	// TODO DefaultLocatorService(it.manifest.readingOrder, it.publication) if LocatorService_Name doesn't exist

	return &ServicesBuilder{
		serviceFactories: fcs,
		publicFlags:      map[ServiceName]bool{},
	}
}

// Builds the actual list of publication services to use in a Publication.
func (s *ServicesBuilder) Build(context Context) map[ServiceName]Service {
	services := make(map[ServiceName]Service, len(s.serviceFactories))
	for k, v := range s.serviceFactories {
		// Allow service factories to be nil
		if v != nil {
			public := s.publicFlags[k]

			// Allow service factories to return nil
			if service := v(context, public); service != nil {
				services[k] = service
			}
		}
	}
	return services
}

// Gets the names of all services currently in the builder
func (s *ServicesBuilder) Services() []ServiceName {
	keys := make([]ServiceName, 0, len(s.serviceFactories))
	for k := range s.serviceFactories {
		keys = append(keys, k)
	}
	return keys
}

// Gets the publication service factory for the given service type.
func (s *ServicesBuilder) Get(name ServiceName) *ServiceFactory {
	if v, ok := s.serviceFactories[name]; ok {
		return &v
	}
	return nil
}

// Sets the publication service factory for the given service type.
func (s *ServicesBuilder) Set(name ServiceName, factory *ServiceFactory) {
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
func (s *ServicesBuilder) Remove(name ServiceName) {
	if name == "" {
		return
	}
	delete(s.serviceFactories, name)
}

// Removes all service factories except the ones producing the given kinds of services, if any.
// If no services are given, all service factories are removed.
func (s *ServicesBuilder) RemoveExcept(name ...ServiceName) {
	if len(name) == 0 {
		clear(s.serviceFactories)
	}

	whitelist := make(map[ServiceName]struct{}, len(name))
	for _, n := range name {
		whitelist[n] = struct{}{}
	}
	for k := range s.serviceFactories {
		if _, ok := whitelist[k]; !ok {
			delete(s.serviceFactories, k)
		}
	}
}

// Replaces the service factory associated with the given service type with the result of [transform]
func (s *ServicesBuilder) Decorate(name ServiceName, transform func(*ServiceFactory) ServiceFactory) {
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

func (s *ServicesBuilder) ExposeLinks(name ServiceName) {
	s.publicFlags[name] = true
}

func (s *ServicesBuilder) HideLinks(name ServiceName) {
	delete(s.publicFlags, name)
}
