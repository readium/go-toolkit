package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPresentationMarshalMinimalJSON(t *testing.T) {
	var p Presentation
	assert.NoError(t, json.Unmarshal([]byte(`{}`), &p))
	assert.Equal(t, &p, NewPresentation(), "new Presentation should be equal to empty JSON object")
}

func TestPresentationMarshalFullJSON(t *testing.T) {
	var p Presentation
	assert.NoError(t, json.Unmarshal([]byte(`{
		"clipped": true,
		"continuous": false,
		"fit": "cover",
		"orientation": "landscape",
		"overflow": "paginated",
		"spread": "both",
		"layout": "fixed"
	}`), &p))
	assert.Equal(t, Presentation{
		Clipped:     newBool(true),
		Continuous:  newBool(false),
		Fit:         (*Fit)(newString("cover")),
		Orientation: (*Orientation)(newString("landscape")),
		Overflow:    (*Overflow)(newString("paginated")),
		Spread:      (*Spread)(newString("both")),
		Layout:      (*EPUBLayout)(newString("fixed")),
	}, p, "Presentation should be equal to given JSON")
}

func TestPresentationMinimalJSON(t *testing.T) {
	p, err := json.Marshal(NewPresentation())
	assert.NoError(t, err)
	assert.Equal(t, "{}", string(p), "JSON of default Presentation should be equal to JSON representation")
}

func TestPresentationFullJSON(t *testing.T) {
	p, err := json.Marshal(&Presentation{
		Clipped:     newBool(true),
		Continuous:  newBool(false),
		Fit:         (*Fit)(newString("cover")),
		Orientation: (*Orientation)(newString("landscape")),
		Overflow:    (*Overflow)(newString("paginated")),
		Spread:      (*Spread)(newString("both")),
		Layout:      (*EPUBLayout)(newString("fixed")),
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"clipped": true,
		"fit": "cover",
		"orientation": "landscape",
		"overflow": "paginated",
		"spread": "both",
		"layout": "fixed"
	}`, string(p), "JSON of Presentation should be equal to JSON representation")
}
