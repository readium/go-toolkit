package searcher

import (
	"errors"

	"github.com/readium/r2-streamer-go/pkg/pub"
)

func init() {
	searcherList = append(searcherList, List{publicationType: "epub", searcher: searchEpub, indexer: indexEpub})
}

// FetchEpub TODO add doc
func searchEpub(publication pub.Manifest, searchTerm string) (SearchResults, error) {
	// var bleveIndex bleve.Index
	// var bleveIndexFile string
	// var err error
	//
	// for _, internal := range publication.Internal {
	// 	if internal.Name == "filename" {
	// 		bleveIndexFile = "/tmp/" + internal.Value.(string) + ".bleve"
	// 	}
	// }
	//
	// if bleveIndexFile != "" {
	// 	bleveIndex, err = bleve.Open(bleveIndexFile)
	// 	if err != nil {
	// 		return pub.SearchResults{}, errors.New("can't find results")
	// 	}
	//
	// 	query := bleve.NewMatchQuery(searchTerm)
	// 	search := bleve.NewSearchRequest(query)
	// 	search.IncludeLocations = true
	// 	searchResults, _ := bleveIndex.Search(search)
	//
	// 	searchReturn := pub.SearchResults{Query: searchTerm, TotalResults: int(searchResults.Total)}
	//
	// 	for _, r := range searchResults.Hits {
	// 		returnResult := pub.SearchResult{Resource: r.ID}
	// 		for _, l := range r.Locations {
	// 			for k, v := range l {
	// 				returnResult.Match = k
	// 				for _, l2 := range v {
	// 					locator := pub.Locator{}
	// 					locator.Position = l2.Start
	// 					returnResult.Locators = locator
	// 				}
	// 			}
	// 			searchReturn.Results = append(searchReturn.Results, returnResult)
	// 		}
	// 	}
	//
	// 	return searchReturn, nil
	// }
	return SearchResults{}, errors.New("can't find results")
}

func indexEpub(publication pub.Manifest) {
	// var err error
	// var bleveIndexFile string
	// var bleveIndex bleve.Index
	//
	// for _, internal := range publication.Internal {
	// 	if internal.Name == "filename" {
	// 		bleveIndexFile = "/tmp/" + internal.Value.(string) + ".bleve"
	// 	}
	// }
	//
	// if bleveIndexFile != "" {
	// 	bleveIndex, err = bleve.Open(bleveIndexFile)
	// 	if err == bleve.ErrorIndexPathDoesNotExist {
	// 		indexMapping := bleve.NewIndexMapping()
	//
	// 		bleveIndex, _ = bleve.New(bleveIndexFile, indexMapping)
	//
	// 		for _, s := range publication.ReadingOrder {
	// 			reader, _, _ := fetcher.Fetch(publication, s.Href)
	// 			buff, _ := ioutil.ReadAll(reader)
	// 			fmt.Println("indexing " + s.Href)
	// 			bleveIndex.Index(s.Href, string(buff))
	// 		}
	// 	}
	// 	fmt.Println("finish indexing")
	// }
}
