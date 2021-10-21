package manifest

// PublicationCollection is used as an extension points for other collections in a Publication
type PublicationCollection struct {
	Role     string
	Metadata map[string]interface{}
	Links    []Link
	Children []PublicationCollection
}
