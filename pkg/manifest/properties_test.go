package manifest

import (
	"encoding/json"
	"sync"
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
		properties: map[string]interface{}{
			"other-property1": "value",
			"other-property2": []interface{}{float64(42)},
		},
		mutext: &sync.RWMutex{},
	}, p)
}

func TestPropertiesMarshalFullJSON(t *testing.T) {
	p := Properties{
		properties: map[string]interface{}{
			"other-property1": "value",
			"other-property2": []interface{}{float64(42)},
		},
		mutext: &sync.RWMutex{},
	}
	b, err := json.Marshal(p)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"other-property1":"value","other-property2":[42]}`, string(b))
}

func TestPropertiesAddGiven(t *testing.T) {
	p2 := Properties{
		properties: map[string]interface{}{
			"other-property1": "value",
			"other-property2": []interface{}{float64(42)},
		},
		mutext: &sync.RWMutex{},
	}
	assert.Equal(t, Properties{
		properties: map[string]interface{}{
			"other-property1": "value",
			"other-property2": []interface{}{float64(42)},
			"additional":      "property",
		},
		mutext: &sync.RWMutex{},
	}, p2.Add(map[string]interface{}{
		"additional": "property",
	}))
}

// Presentation-specific properties

func TestPropertiesClippedAvailable(t *testing.T) {
	assert.Equal(t, true, *Properties{
		properties: map[string]interface{}{
			"clipped": true,
		},
		mutext: &sync.RWMutex{},
	}.Clipped(), "Clipped true when set to true")
}

func TestPropertiesClippedMissing(t *testing.T) {
	assert.Nil(t, Properties{
		properties: map[string]interface{}{},
		mutext:     &sync.RWMutex{},
	}.Clipped(), "Clipped nil when missing")
}

func TestPropertiesFitAvailable(t *testing.T) {
	assert.Equal(t, FitCover, Properties{
		properties: map[string]interface{}{
			"fit": "cover",
		},
		mutext: &sync.RWMutex{},
	}.Fit(), "Fit cover when set to cover")
}

func TestPropertiesFitMissing(t *testing.T) {
	assert.Empty(t, Properties{
		properties: map[string]interface{}{},
		mutext:     &sync.RWMutex{},
	}.Clipped(), "Fit empty when missing")
}

func TestPropertiesOrientationAvailable(t *testing.T) {
	assert.Equal(t, OrientationLandscape, Properties{
		properties: map[string]interface{}{
			"orientation": "landscape",
		},
		mutext: &sync.RWMutex{},
	}.Orientation(), "Orientation landscape when set to landscape")
}

func TestPropertiesOrientationMissing(t *testing.T) {
	assert.Empty(t, Properties{
		properties: map[string]interface{}{},
		mutext:     &sync.RWMutex{},
	}.Orientation(), "Orientation empty when missing")
}

func TestPropertiesOverflowAvailable(t *testing.T) {
	assert.Equal(t, OverflowScrolled, Properties{
		properties: map[string]interface{}{
			"overflow": "scrolled",
		},
		mutext: &sync.RWMutex{},
	}.Overflow(), "Overflow scrolled when set to scrolled")
}

func TestPropertiesOverflowMissing(t *testing.T) {
	assert.Empty(t, Properties{
		properties: map[string]interface{}{},
		mutext:     &sync.RWMutex{},
	}.Overflow(), "Overflow empty when missing")
}

func TestPropertiesPageAvailable(t *testing.T) {
	assert.Equal(t, PageRight, Properties{
		properties: map[string]interface{}{
			"page": "right",
		},
		mutext: &sync.RWMutex{},
	}.Page(), "Page right when set to right")
}

func TestPropertiesPageMissing(t *testing.T) {
	assert.Empty(t, Properties{
		properties: map[string]interface{}{},
		mutext:     &sync.RWMutex{},
	}.Page(), "Page empty when missing")
}

func TestPropertiesSpreadAvailable(t *testing.T) {
	assert.Equal(t, SpreadBoth, Properties{
		properties: map[string]interface{}{
			"spread": "both",
		},
		mutext: &sync.RWMutex{},
	}.Spread(), "Spread both when set to both")
}

func TestPropertiesSpreadMissing(t *testing.T) {
	assert.Empty(t, Properties{
		properties: map[string]interface{}{},
		mutext:     &sync.RWMutex{},
	}.Spread(), "Spread empty when missing")
}
