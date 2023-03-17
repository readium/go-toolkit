package util

type JSONMappable interface {
	// JSONMap returns a JSON object representation of the receiver.
	JSONMap() (map[string]interface{}, error)
}
