package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var emptyA11y = A11y{
	ConformsTo:            []A11yProfile{},
	AccessModes:           []A11yAccessMode{},
	AccessModesSufficient: [][]A11yPrimaryAccessMode{},
	Features:              []A11yFeature{},
	Hazards:               []A11yHazard{},
}

func TestA11yUnmarshalMinimalJSON(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte("{}"), &m))
	assert.Equal(t, emptyA11y, m, "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yUnmarshalFullJSON(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte(`{
		"conformsTo": ["https://profile1", "https://profile2"],
		"certification": {
			"certifiedBy": "company1",
			"credential": "credential1",
			"report": "https://report1"
		},
		"summary": "Summary",
		"accessMode": ["auditory", "chartOnVisual"],
		"accessModeSufficient": [["visual", "tactile"]],
		"feature": ["readingOrder", "alternativeText"],
		"hazard": ["flashing", "motionSimulation"]
	}`), &m))
	assert.Equal(t, A11y{
		ConformsTo: []A11yProfile{
			"https://profile1",
			"https://profile2",
		},
		Certification: &A11yCertification{
			CertifiedBy: "company1",
			Credential:  "credential1",
			Report:      "https://report1",
		},
		Summary: "Summary",
		AccessModes: []A11yAccessMode{
			A11yAccessModeAuditory,
			A11yAccessModeChartOnVisual,
		},
		AccessModesSufficient: [][]A11yPrimaryAccessMode{
			{
				A11yPrimaryAccessModeVisual,
				A11yPrimaryAccessModeTactile,
			},
		},
		Features: []A11yFeature{
			A11yFeatureReadingOrder,
			A11yFeatureAlternativeText,
		},
		Hazards: []A11yHazard{
			A11yHazardFlashing,
			A11yHazardMotionSimulation,
		},
	}, m, "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yUnmarshalInvalidSummaryIsIgnored(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte(`{"summary": ["sum1", "sum2"]}`), &m))
	assert.Equal(t, emptyA11y, m, "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yMarshalMinimalJSON(t *testing.T) {
	m := A11y{
		ConformsTo:            []A11yProfile{},
		AccessModes:           []A11yAccessMode{},
		AccessModesSufficient: [][]A11yPrimaryAccessMode{},
		Features:              []A11yFeature{},
		Hazards:               []A11yHazard{},
	}
	data, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte(`{}`), "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yMarshalFullJSON(t *testing.T) {
	m := A11y{
		ConformsTo: []A11yProfile{
			"http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a",
			"https://profile2",
		},
		Certification: &A11yCertification{
			CertifiedBy: "company1",
			Credential:  "credential1",
			Report:      "https://report1",
		},
		Summary: "Summary",
		AccessModes: []A11yAccessMode{
			A11yAccessModeAuditory,
			A11yAccessModeChartOnVisual,
		},
		AccessModesSufficient: [][]A11yPrimaryAccessMode{
			{A11yPrimaryAccessModeAuditory},
			{
				A11yPrimaryAccessModeVisual,
				A11yPrimaryAccessModeTactile,
			},
			{A11yPrimaryAccessModeVisual},
		},
		Features: []A11yFeature{
			A11yFeatureReadingOrder,
			A11yFeatureAlternativeText,
		},
		Hazards: []A11yHazard{
			A11yHazardFlashing,
			A11yHazardMotionSimulation,
		},
	}
	data, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(
		t,
		data,
		[]byte(`{"conformsTo":["http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a","https://profile2"],"certification":{"certifiedBy":"company1","credential":"credential1","report":"https://report1"},"summary":"Summary","accessMode":["auditory","chartOnVisual"],"accessModeSufficient":[["auditory"],["visual","tactile"],["visual"]],"feature":["readingOrder","alternativeText"],"hazard":["flashing","motionSimulation"]}`),
	)
}
