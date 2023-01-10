package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestA11yUnmarshalMinimalJSON(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte("{}"), &m))
	assert.Equal(t, NewA11y(), m, "unmarshalled JSON object should be equal to A11y object")
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
	assert.Equal(t, NewA11y(), m, "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yUnmarshalConformsToString(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte(`{"conformsTo": "http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a"}`), &m))
	var e A11y = NewA11y()
	e.ConformsTo = []A11yProfile{EPUBA11y10WCAG20A}
	assert.Equal(t, e, m, "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yUnmarshalConformsToArray(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte(`{"conformsTo": ["http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a", "https://profile2"]}`), &m))
	var e A11y = NewA11y()
	e.ConformsTo = []A11yProfile{EPUBA11y10WCAG20A, "https://profile2"}
	assert.Equal(t, e, m, "unmarshalled JSON object should be equal to A11y object")
}

func TestA11yUnmarshalAccessModeSufficientContainingBothStringsAndArrays(t *testing.T) {
	var m A11y
	assert.NoError(t, json.Unmarshal([]byte(`{"accessModeSufficient": ["auditory", ["visual", "tactile"], [], "visual"]}`), &m))
	var e A11y = NewA11y()
	e.AccessModesSufficient = [][]A11yPrimaryAccessMode{
		{A11yPrimaryAccessModeAuditory},
		{A11yPrimaryAccessModeVisual, A11yPrimaryAccessModeTactile},
		{A11yPrimaryAccessModeVisual},
	}
	assert.Equal(t, e, m, "unmarshalled JSON object should be equal to A11y object")
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
