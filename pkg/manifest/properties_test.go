package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertiesParseNilJSON(t *testing.T) {
	props, err := PropertiesFromJSON(nil)
	assert.NoError(t, err)
	assert.Equal(t, Properties{}, props)
}

func TestProperiesUnmarshalMinimalJSON(t *testing.T) {
	var p Properties
	assert.NoError(t, json.Unmarshal([]byte(`{}`), &p))
	assert.Equal(t, Properties{}, p)
}

func TestPropertiesUnmarshalFullJSON(t *testing.T) {
	var p Properties
	assert.NoError(t, json.Unmarshal([]byte(`{
		"other-property1": "value",
		"other-property2": [42]
	}`), &p))

	assert.Equal(t, Properties{
		"other-property1": "value",
		"other-property2": []interface{}{float64(42)},
	}, p)
}

func TestPropertiesAddGiven(t *testing.T) {
	p2 := Properties{
		"other-property1": "value",
		"other-property2": []interface{}{float64(42)},
	}
	assert.Equal(t, Properties{
		"other-property1": "value",
		"other-property2": []interface{}{float64(42)},
		"additional":      "property",
	}, p2.Add(Properties{
		"additional": "property",
	}))
}
