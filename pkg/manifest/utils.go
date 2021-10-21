package manifest

import "fmt"

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func addToSet(s []string, e string) []string {
	if !contains(s, e) {
		s = append(s, e)
	}
	return s
}

func parseSetOrString(value interface{}) (result []string, err error) {
	switch v := value.(type) {
	case string:
		result = []string{v} // Just a single item
	case []interface{}:
		// Deduplicate the slice since it's a set (no unique items)
		result = []string{}
		for i, vv := range v {
			role, ok := vv.(string)
			if !ok {
				err = fmt.Errorf("object at position %d is not a string", i)
				return
			}
			result = addToSet(result, role)
		}
	}
	return
}

func newTrue() *bool {
	b := true
	return &b
}
