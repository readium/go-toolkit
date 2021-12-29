package manifest

import (
	"fmt"
	"time"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

func addToSet(s []string, e string) []string {
	if !extensions.Contains(s, e) {
		s = append(s, e)
	}
	return s
}

func parseSliceOrString(value interface{}, deduplicate bool) (result []string, err error) {
	switch v := value.(type) {
	case string:
		result = []string{v} // Just a single item
	case []interface{}:
		result = []string{}
		for i, vv := range v {
			str, ok := vv.(string)
			if !ok {
				err = fmt.Errorf("object at position %d is not a string", i)
				return
			}
			if deduplicate {
				// Deduplicate the slice since it's going to be a set (no unique items)
				result = addToSet(result, str)
			} else {
				result = append(result, str)
			}
		}
	}
	return
}

// TODO replace with generic
func newBool(val bool) *bool {
	b := val
	return &b
}

func firstLinkWithRel(links []Link, rel string) *Link {
	for _, link := range links {
		for _, linkRel := range link.Rels {
			if linkRel == rel {
				return &link
			}
		}
	}
	return nil
}

// Utilities for convenient JSON unmarshalling
// TODO replace a lot of these with generics!

func parseOptTime(raw interface{}) *time.Time {
	rt, ok := raw.(string)
	if !ok {
		return nil
	}
	t := &time.Time{}
	t.UnmarshalText([]byte(rt)) // Ignores errors!
	return t
}

func parseOptString(raw interface{}) string {
	rs, _ := raw.(string)
	return rs
}

func parseOptUInt(raw interface{}) uint {
	ri, _ := raw.(uint)
	return ri
}

func parseOptBool(raw interface{}) bool {
	rb, _ := raw.(bool)
	return rb
}

func parseOptFloat64(raw interface{}) float64 {
	rb, _ := raw.(float64)
	return rb
}
