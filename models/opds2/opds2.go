package opds2

import (
	"time"

	"github.com/readium/r2-streamer-go/models"
)

// OPDSFeed represent the root opds2 feed
type OPDSFeed struct {
	Context      []string             `json:"@context"`
	Metadata     OPDSMetadata         `json:"metadata"`
	Links        []models.Link        `json:"links"`
	Publications []models.Publication `json:"publications,omitempty"`
	Navigation   []models.Link        `json:"navigation,omitempty"`
	Facets       []OPDSFacet          `json:"facets,omitempty"`
	Groups       []OPDSCollection     `json:"groups,omitempty"`
	Modified     *time.Time           `json:"modified,omitempty"`
}

// OPDSMetadata information for the feed, facets or collections
type OPDSMetadata struct {
	RDFType       string `json:"@type,omitempty"`
	Title         string `json:"title"`
	NumberOfItems int    `json:"numberOfItems,omitempty"`
}

// OPDSFacet facet information for filtering
type OPDSFacet struct {
	Metadata OPDSMetadata  `json:"metadata"`
	Links    []models.Link `json:"links"`
}

// OPDSCollection collection of publication
type OPDSCollection struct {
	Metadata     OPDSMetadata         `json:"metadata"`
	Publications []models.Publication `json:"publications"`
	Links        []models.Link        `json:"links"`
}
