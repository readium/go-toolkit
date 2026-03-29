package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropertiesUnmarshalNilJSON(t *testing.T) {
	props, err := PropertiesFromJSON(nil)
	require.NoError(t, err)
	assert.Equal(t, Properties{}, props)
}

func TestProperiesUnmarshalMinimalJSON(t *testing.T) {
	var p Properties
	require.NoError(t, json.Unmarshal([]byte(`{}`), &p))
	assert.Equal(t, Properties{}, p)
}

func TestPropertiesUnmarshalFullJSON(t *testing.T) {
	var p Properties
	require.NoError(t, json.Unmarshal([]byte(`{
		"other-property1": "value",
		"other-property2": [42]
	}`), &p))

	assert.Equal(t, Properties{
		"other-property1": "value",
		"other-property2": []interface{}{float64(42)},
	}, p)
}

/*func TestPropertiesAddGiven(t *testing.T) {
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
}*/

func TestPropertiesPageAvailable(t *testing.T) {
	assert.Equal(t, PageRight, Properties{
		"page": "right",
	}.Page(), "Page right when set to right")
}

func TestPropertiesPageMissing(t *testing.T) {
	// These are the same thing
	assert.Equal(t, PageNone, Properties{}.Page(), "Page empty when missing")
	assert.Empty(t, Properties{}.Page(), "Page empty when missing")
}
