package manifest

// PublicationCollection is used as an extension points for other collections in a Publication
type PublicationCollection struct {
	Metadata map[string]interface{}
	Links    []Link
	Children map[string][]PublicationCollection
}

type Collection = Contributor
