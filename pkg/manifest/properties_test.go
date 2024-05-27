package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertiesUnmarshalNilJSON(t *testing.T) {
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

// Presentation-specific properties

func TestPropertiesClippedAvailable(t *testing.T) {
	assert.Equal(t, true, *Properties{
		"clipped": true,
	}.Clipped(), "Clipped true when set to true")
}

func TestPropertiesClippedMissing(t *testing.T) {
	assert.Nil(t, Properties{}.Clipped(), "Clipped nil when missing")
}

func TestPropertiesFitAvailable(t *testing.T) {
	assert.Equal(t, FitCover, Properties{
		"fit": "cover",
	}.Fit(), "Fit cover when set to cover")
}

func TestPropertiesFitMissing(t *testing.T) {
	assert.Empty(t, Properties{}.Clipped(), "Fit empty when missing")
}

func TestPropertiesOrientationAvailable(t *testing.T) {
	assert.Equal(t, OrientationLandscape, Properties{
		"orientation": "landscape",
	}.Orientation(), "Orientation landscape when set to landscape")
}

func TestPropertiesOrientationMissing(t *testing.T) {
	assert.Empty(t, Properties{}.Orientation(), "Orientation empty when missing")
}

func TestPropertiesOverflowAvailable(t *testing.T) {
	assert.Equal(t, OverflowScrolled, Properties{
		"overflow": "scrolled",
	}.Overflow(), "Overflow scrolled when set to scrolled")
}

func TestPropertiesOverflowMissing(t *testing.T) {
	assert.Empty(t, Properties{}.Overflow(), "Overflow empty when missing")
}

func TestPropertiesPageAvailable(t *testing.T) {
	assert.Equal(t, PageRight, Properties{
		"page": "right",
	}.Page(), "Page right when set to right")
}

func TestPropertiesPageMissing(t *testing.T) {
	assert.Empty(t, Properties{}.Page(), "Page empty when missing")
}

func TestPropertiesSpreadAvailable(t *testing.T) {
	assert.Equal(t, SpreadBoth, Properties{
		"spread": "both",
	}.Spread(), "Spread both when set to both")
}

func TestPropertiesSpreadMissing(t *testing.T) {
	assert.Empty(t, Properties{}.Spread(), "Spread empty when missing")
}
