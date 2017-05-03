package opds2

import (
	"time"

	"github.com/readium/r2-streamer-go/models"
)

// OPDSFeed is a collection as defined in Readium Web Publication Manifest
type OPDSFeed struct {
	Context      []string             `json:"@context"`
	Metadata     OPDSMetadata         `json:"metadata"`
	Links        []models.Link        `json:"links"`
	Publications []models.Publication `json:"publications,omitempty"`
	Navigation   []models.Link        `json:"navigation,omitempty"`
	Facets       []OPDSFacet          `json:"facets,omitempty"`
	Groups       []OPDSGroup          `json:"groups,omitempty"`
}

// OPDSMetadata has a limited subset of metadata compared to a publication
type OPDSMetadata struct {
	RDFType       string     `json:"@type,omitempty"`
	Title         string     `json:"title"`
	NumberOfItems int        `json:"numberOfItems,omitempty"`
	Modified      *time.Time `json:"modified,omitempty"`
}

// OPDSFacet is a collection that contains a facet group
type OPDSFacet struct {
	Metadata OPDSMetadata  `json:"metadata"`
	Links    []models.Link `json:"links"`
}

// OPDSGroup is a group collection that must contain publications
type OPDSGroup struct {
	Metadata     OPDSMetadata         `json:"metadata"`
	Publications []models.Publication `json:"publications"`
	Links        []models.Link        `json:"links"`
}
