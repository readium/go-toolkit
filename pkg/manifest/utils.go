package manifest

import (
	"fmt"
	"time"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

func parseSliceOrString(value interface{}, deduplicate bool) (result []string, err error) {
	switch v := value.(type) {
	case string:
		result = []string{v} // Just a single item
	case []interface{}:
		result = make([]string, 0, len(v))
		for i, vv := range v {
			str, ok := vv.(string)
			if !ok {
				err = fmt.Errorf("object at position %d is not a string", i)
				return
			}
			if deduplicate {
				// Deduplicate the slice since it's going to be a set (no unique items)
				result = extensions.AddToSet(result, str)
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

// TODO replace with generic
func newString(val string) *string {
	s := val
	return &s
}

func nilstrEq(source *string, val string) bool {
	if source == nil {
		return val == ""
	}
	return *source == val
}

func nilboolEq(source *bool, val bool) bool {
	if source == nil {
		return val == false
	}
	return *source == val
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

func parseOptBool(raw interface{}) bool {
	rb, _ := raw.(bool)
	return rb
}

func parseOptFloat64(raw interface{}) float64 {
	rb, _ := raw.(float64)
	return rb
}

func float64ToUint(f float64) uint {
	if f < 0 {
		return 0
	}
	return uint(f)
}

func float64Positive(f float64) float64 {
	if f < 0 {
		return 0
	}
	return f
}
