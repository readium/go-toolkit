package searcher

import (
	"errors"

	"github.com/readium/r2-streamer-go/pkg/pub"
)

// List TODO add doc
type List struct {
	publicationType string
	searcher        (func(pub.Publication, string) (pub.SearchResults, error))
	indexer         (func(pub.Publication))
}

var searcherList []List

// CanBeSearch check if the publication type has a search interface
func CanBeSearch(publication pub.Publication) bool {
	var typePublication string

	for _, key := range publication.Internal {
		if key.Name == "type" {
			typePublication = key.Value.(string)
		}
	}

	if typePublication != "" {
		for _, searcherFunc := range searcherList {
			if typePublication == searcherFunc.publicationType {
				return true
			}
		}
	}

	return false
}

// Search TODO add doc
func Search(publication pub.Publication, query string) (pub.SearchResults, error) {
	var typePublication string

	for _, key := range publication.Internal {
		if key.Name == "type" {
			typePublication = key.Value.(string)
		}
	}

	if typePublication != "" {
		for _, searcherFunc := range searcherList {
			if typePublication == searcherFunc.publicationType {
				return searcherFunc.searcher(publication, query)
			}
		}
	}

	return pub.SearchResults{}, errors.New("can't find searcher")
}

// Index TODO add doc
func Index(publication pub.Publication) {
	var typePublication string

	for _, key := range publication.Internal {
		if key.Name == "type" {
			typePublication = key.Value.(string)
		}
	}

	if typePublication != "" {
		for _, indexerFunc := range searcherList {
			if typePublication == indexerFunc.publicationType {
				indexerFunc.indexer(publication)
			}
		}
	}
}
