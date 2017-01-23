package searcher

import (
	"errors"

	"github.com/feedbooks/webpub-streamer/models"
)

// List TODO add doc
type List struct {
	publicationType string
	searcher        (func(models.Publication, string) (models.SearchResults, error))
	indexer         (func(models.Publication))
}

var searcherList []List

// CanBeSearch check if the publication type has a search interface
func CanBeSearch(publication models.Publication) bool {
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
func Search(publication models.Publication, query string) (models.SearchResults, error) {
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

	return models.SearchResults{}, errors.New("can't find searcher")
}

// Index TODO add doc
func Index(publication models.Publication) {
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
