package pub

// The Publication shared model is the entrypoint for all the metadata and services related to a Readium publication.
type Publication struct {
	manifest Manifest // The manifest holding the publication metadata extracted from the publication file.
	// fetcher  fetcher.Fetcher // The underlying fetcher used to read publication resources.
	// TODO servicesBuilder
	// TODO positionsFactory
	// TODO services []Service
	_manifest Manifest
}

func InitPublication() *Publication {
	return &Publication{}
}

// PublicationCollection is used as an extension points for other collections in a Publication
type PublicationCollection struct {
	Role     string
	Metadata map[string]interface{}
	Links    []Link
	Children []PublicationCollection
}
