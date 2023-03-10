package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
)

// A11y holds the accessibility metadata of a Publication.
//
// https://www.w3.org/2021/a11y-discov-vocab/latest/
// https://readium.org/webpub-manifest/schema/a11y.schema.json
type A11y struct {
	ConformsTo            []A11yProfile             `json:"conformsTo,omitempty"`           // An established standard to which the described resource conforms.
	Certification         *A11yCertification        `json:"certification,omitempty"`        // Certification of accessible publications.
	Summary               string                    `json:"summary,omitempty"`              // A human-readable summary of specific accessibility features or deficiencies, consistent with the other accessibility metadata but expressing subtleties such as "short descriptions are present but long descriptions will be needed for non-visual users" or "short descriptions are present and no long descriptions are needed."
	AccessModes           []A11yAccessMode          `json:"accessMode,omitempty"`           // The human sensory perceptual system or cognitive faculty through which a person may process or perceive information.
	AccessModesSufficient [][]A11yPrimaryAccessMode `json:"accessModeSufficient,omitempty"` //  A list of single or combined accessModes that are sufficient to understand all the intellectual content of a resource.
	Features              []A11yFeature             `json:"feature,omitempty"`              // Content features of the resource, such as accessible media, alternatives and supported enhancements for accessibility.
	Hazards               []A11yHazard              `json:"hazard,omitempty"`               // A characteristic of the described resource that is physiologically dangerous to some users.
}

// NewA11y creates a new empty A11y.
func NewA11y() A11y {
	return A11y{
		ConformsTo:            []A11yProfile{},
		AccessModes:           []A11yAccessMode{},
		AccessModesSufficient: [][]A11yPrimaryAccessMode{},
		Features:              []A11yFeature{},
		Hazards:               []A11yHazard{},
	}
}

func (a A11y) IsEmpty() bool {
	return len(a.ConformsTo) == 0 && a.Certification == nil && a.Summary == "" &&
		len(a.AccessModes) == 0 && len(a.AccessModesSufficient) == 0 &&
		len(a.Features) == 0 && len(a.Hazards) == 0
}

// Merge extends or overwrites the current A11y with the given one.
func (a *A11y) Merge(other *A11y) {
	if other == nil || other.IsEmpty() {
		return
	}

	a.ConformsTo = extensions.AppendIfMissing(a.ConformsTo, other.ConformsTo...)

	if other.Certification != nil {
		a.Certification = other.Certification
	}

	if len(other.Summary) > 0 {
		a.Summary = other.Summary
	}

	a.AccessModes = extensions.AppendIfMissing(a.AccessModes, other.AccessModes...)
	a.Features = extensions.AppendIfMissing(a.Features, other.Features...)
	a.Hazards = extensions.AppendIfMissing(a.Hazards, other.Hazards...)

	for _, otherAms := range other.AccessModesSufficient {
		found := false
		for _, ams := range a.AccessModesSufficient {
			if extensions.Equal(otherAms, ams) {
				found = true
				break
			}
		}
		if !found {
			a.AccessModesSufficient = append(a.AccessModesSufficient, otherAms)
		}
	}
}

func A11yFromJSON(rawJSON map[string]interface{}) (*A11y, error) {
	if rawJSON == nil {
		return nil, nil
	}

	a := new(A11y)

	conformsTo, err := parseSliceOrString(rawJSON["conformsTo"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'conformsTo'")
	}
	a.ConformsTo = A11yProfilesFromStrings(conformsTo)

	if certJSON, ok := rawJSON["certification"].(map[string]interface{}); ok {
		c := A11yCertification{
			CertifiedBy: parseOptString(certJSON["certifiedBy"]),
			Credential:  parseOptString(certJSON["credential"]),
			Report:      parseOptString(certJSON["report"]),
		}
		a.Certification = &c
	}

	if summary, ok := rawJSON["summary"].(string); ok {
		a.Summary = summary
	}

	accessModes, err := parseSliceOrString(rawJSON["accessMode"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'accessMode'")
	}
	a.AccessModes = A11yAccessModesFromStrings(accessModes)

	ams := [][]A11yPrimaryAccessMode{}
	if amsJSON, ok := rawJSON["accessModeSufficient"].([]interface{}); ok {
		for _, l := range amsJSON {
			strings, err := parseSliceOrString(l, true)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshalling 'accessModeSufficient'")
			}
			if len(strings) > 0 {
				ams = append(ams, A11yPrimaryAccessModesFromStrings(strings))
			}
		}
	}
	a.AccessModesSufficient = ams

	features, err := parseSliceOrString(rawJSON["feature"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'feature'")
	}
	a.Features = A11yFeaturesFromStrings(features)

	hazards, err := parseSliceOrString(rawJSON["hazard"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'hazard'")
	}
	a.Hazards = A11yHazardsFromStrings(hazards)

	return a, nil
}

func (e *A11y) UnmarshalJSON(data []byte) error {
	var d interface{}
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}

	mp, ok := d.(map[string]interface{})
	if !ok {
		return errors.New("accessibility object not a map with string keys")
	}

	fe, err := A11yFromJSON(mp)
	if err != nil {
		return err
	}
	*e = *fe
	return nil
}

// A11yProfile represents an established accessibility standard a publication can conform to.
type A11yProfile string

const (
	// EPUB Accessibility 1.0 - WCAG 2.0 Level A
	EPUBA11y10WCAG20A A11yProfile = "http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a"
	// EPUB Accessibility 1.0 - WCAG 2.0 Level AA
	EPUBA11y10WCAG20AA A11yProfile = "http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aa"
	// EPUB Accessibility 1.0 - WCAG 2.0 Level AAA
	EPUBA11y10WCAG20AAA A11yProfile = "http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aaa"
)

func A11yProfilesFromStrings(strings []string) []A11yProfile {
	return fromStrings(strings, func(str string) A11yProfile {
		return A11yProfile(str)
	})
}

// A11yCertification represents a certification for an accessible publication.
type A11yCertification struct {
	CertifiedBy string `json:"certifiedBy,omitempty"` // Identifies a party responsible for the testing and certification of the accessibility of a Publication.
	Credential  string `json:"credential,omitempty"`  // Identifies a credential or badge that establishes the authority of the party identified in the associated `certifiedBy` property to certify content accessible.
	Report      string `json:"report,omitempty"`      // Provides a link to an accessibility report created by the party identified in the associated `certifiedBy` property.
}

func (c A11yCertification) IsEmpty() bool {
	return c.CertifiedBy == "" && c.Credential == "" && c.Report == ""
}

// A11yAccessMode is a human sensory perceptual system or cognitive faculty through which a person may process or perceive information.
type A11yAccessMode string

const (
	// Indicates that the resource contains information encoded in auditory form.
	A11yAccessModeAuditory A11yAccessMode = "auditory"

	// Indicates that the resource contains charts encoded in visual form.
	A11yAccessModeChartOnVisual A11yAccessMode = "chartOnVisual"

	// Indicates that the resource contains chemical equations encoded in visual form.
	A11yAccessModeChemOnVisual A11yAccessMode = "chemOnVisual"

	// Indicates that the resource contains information encoded such that color perception is necessary.
	A11yAccessModeColorDependent A11yAccessMode = "colorDependent"

	// Indicates that the resource contains diagrams encoded in visual form.
	A11yAccessModeDiagramOnVisual A11yAccessMode = "diagramOnVisual"

	// Indicates that the resource contains mathematical notations encoded in visual form.
	A11yAccessModeMathOnVisual A11yAccessMode = "mathOnVisual"

	// Indicates that the resource contains musical notation encoded in visual form.
	A11yAccessModeMusicOnVisual A11yAccessMode = "musicOnVisual"

	// Indicates that the resource contains information encoded in tactile form.
	//
	// Note that although an indication of a tactile mode often indicates the content is encoded
	// using a braille system, this is not always the case. Tactile perception may also indicate,
	// for example, the use of tactile graphics to convey information.
	A11yAccessModeTactile A11yAccessMode = "tactile"

	// Indicates that the resource contains text encoded in visual form.
	A11yAccessModeTextOnVisual A11yAccessMode = "textOnVisual"

	// Indicates that the resource contains information encoded in textual form.
	A11yAccessModeTextual A11yAccessMode = "textual"

	// Indicates that the resource contains information encoded in visual form.
	A11yAccessModeVisual A11yAccessMode = "visual"
)

func A11yAccessModesFromStrings(strings []string) []A11yAccessMode {
	return fromStrings(strings, func(str string) A11yAccessMode {
		return A11yAccessMode(str)
	})
}

// A11yPrimaryAccessMode is a human primary sensory perceptual system or cognitive faculty through which a person may process or perceive information.
type A11yPrimaryAccessMode string

const (
	// Indicates that auditory perception is necessary to consume the information.
	A11yPrimaryAccessModeAuditory A11yPrimaryAccessMode = "auditory"

	// Indicates that tactile perception is necessary to consume the information.
	A11yPrimaryAccessModeTactile A11yPrimaryAccessMode = "tactile"

	// Indicates that the ability to read textual content is necessary to consume the information.
	//
	// Note that reading textual content does not require visual perception, as textual content
	// can be rendered as audio using a text-to-speech capable device or assistive technology.
	A11yPrimaryAccessModeTextual A11yPrimaryAccessMode = "textual"

	// Indicates that visual perception is necessary to consume the information.
	A11yPrimaryAccessModeVisual A11yPrimaryAccessMode = "visual"
)

func A11yPrimaryAccessModesFromStrings(strings []string) []A11yPrimaryAccessMode {
	return fromStrings(strings, func(str string) A11yPrimaryAccessMode {
		return A11yPrimaryAccessMode(str)
	})
}

// A11yFeature is a content feature of the described resource, such as accessible media, alternatives and supported enhancements for accessibility.
type A11yFeature string

const (
	// The work includes annotations from the author, instructor and/or others.
	A11yFeatureAnnotations A11yFeature = "annotations"

	// Indicates the resource includes ARIA roles to organize and improve the structure and navigation.
	//
	// The use of this value corresponds to the inclusion of Document Structure, Landmark,
	// Live Region, and Window roles [WAI-ARIA].
	A11yFeatureAria A11yFeature = "ARIA"

	// The work includes bookmarks to facilitate navigation to key points.
	A11yFeatureBookmarks A11yFeature = "bookmark"

	// The work includes an index to the content.
	A11yFeatureIndex A11yFeature = "index"

	// The work includes equivalent print page numbers. This setting is most commonly used
	// with ebooks for which there is a print equivalent.
	A11yFeaturePrintPageNumbers A11yFeature = "printPageNumbers"

	// The reading order of the content is clearly defined in the markup
	// (e.g., figures, sidebars and other secondary content has been marked up to allow it
	// to be skipped automatically and/or manually escaped from).
	A11yFeatureReadingOrder A11yFeature = "readingOrder"

	// The use of headings in the work fully and accurately reflects the document hierarchy,
	// allowing navigation by assistive technologies.
	A11yFeatureStructuralNavigation A11yFeature = "structuralNavigation"

	// The work includes a table of contents that provides links to the major sections of the content.
	A11yFeatureTableOfContents A11yFeature = "tableOfContents"

	// The contents of the PDF have been tagged to permit access by assistive technologies.
	A11yFeatureTaggedPDF A11yFeature = "taggedPDF"

	// Alternative text is provided for visual content (e.g., via the HTML `alt` attribute).
	A11yFeatureAlternativeText A11yFeature = "alternativeText"

	// Audio descriptions are available (e.g., via an HTML `track` element with its `kind`
	// attribute set to "descriptions".
	A11yFeatureAudioDescription A11yFeature = "audioDescription"

	// Indicates that synchronized captions are available for audio and video content.
	A11yFeatureCaptions A11yFeature = "captions"

	// Textual descriptions of math equations are included, whether in the alt attribute
	// for image-based equations,
	A11yFeatureDescribedMath A11yFeature = "describeMath"

	// Descriptions are provided for image-based visual content and/or complex structures
	// such as tables, mathematics, diagrams, and charts.
	A11yFeatureLongDescription A11yFeature = "longDescription"

	// Indicates that `ruby` annotations HTML are provided in the content. Ruby annotations
	// are used as pronunciation guides for the logographic characters for languages like
	// Chinese or Japanese. It makes difficult Kanji or CJK ideographic characters more accessible.
	//
	// The absence of rubyAnnotations implies that no CJK ideographic characters have ruby.
	A11yFeatureRubyAnnotations A11yFeature = "rubyAnnotations"

	// Sign language interpretation is available for audio and video content.
	A11yFeatureSignLanguage A11yFeature = "signLanguage"

	// Indicates that a transcript of the audio content is available.
	A11yFeatureTranscript A11yFeature = "transcript"

	// Display properties are controllable by the user. This property can be set, for example,
	// if custom CSS style sheets can be applied to the content to control the appearance.
	// It can also be used to indicate that styling in document formats like Word and PDF
	// can be modified.
	A11yFeatureDisplayTransformability A11yFeature = "displayTransformability"

	// Describes a resource that offers both audio and text, with information that allows them
	// to be rendered simultaneously. The granularity of the synchronization is not specified.
	// This term is not recommended when the only material that is synchronized is
	// the document headings.
	A11yFeatureSynchronizedAudioText A11yFeature = "synchronizedAudioText"

	// For content with timed interaction, this value indicates that the user can control
	// the timing to meet their needs (e.g., pause and reset)
	A11yFeatureTimingControl A11yFeature = "timingControl"

	// No digital rights management or other content restriction protocols have been applied
	// to the resource.
	A11yFeatureUnlocked A11yFeature = "unlocked"

	// Identifies that chemical information is encoded using the ChemML markup language.
	A11yFeatureChemML A11yFeature = "ChemML"

	// Identifies that mathematical equations and formulas are encoded in the LaTeX
	// typesetting system.
	A11yFeatureLatex A11yFeature = "latex"

	// Identifies that mathematical equations and formulas are encoded in MathML.
	A11yFeatureMathML A11yFeature = "MathML"

	// One or more of SSML, Pronunciation-Lexicon, and CSS3-Speech properties has been used
	// to enhance text-to-speech playback quality.
	A11yFeatureTTSMarkup A11yFeature = "ttsMarkup"

	// Audio content with speech in the foreground meets the contrast thresholds set out
	// in WCAG Success Criteria 1.4.7.
	A11yFeatureHighContrastAudio A11yFeature = "highContrastAudio"

	// Content meets the visual contrast threshold set out in WCAG Success Criteria 1.4.6.
	A11yFeatureHighContrastDisplay A11yFeature = "highContrastDisplay"

	// The content has been formatted to meet large print guidelines.
	//
	// The property is not set if the font size can be increased. See DisplayTransformability.
	A11yFeatureLargePrint A11yFeature = "largePrint"

	// The content is in braille format, or alternatives are available in braille.
	A11yFeatureBraille A11yFeature = "braille"

	// When used with creative works such as books, indicates that the resource includes
	// tactile graphics. When used to describe an image resource or physical object,
	// indicates that the resource is a tactile graphic.
	A11yFeatureTactileGraphic A11yFeature = "tactileGraphic"

	// When used with creative works such as books, indicates that the resource includes models
	// to generate tactile 3D objects. When used to describe a physical object,
	// indicates that the resource is a tactile 3D object.
	A11yFeatureTactileObject A11yFeature = "tactileObject"

	// Indicates that the resource does not contain any accessibility features.
	A11yFeatureNone A11yFeature = "none"
)

func A11yFeaturesFromStrings(strings []string) []A11yFeature {
	return fromStrings(strings, func(str string) A11yFeature {
		return A11yFeature(str)
	})
}

// A11yHazard is a characteristic of the described resource that is physiologically dangerous to some users.
type A11yHazard string

const (
	// Indicates that the resource presents a flashing hazard for photosensitive persons.
	A11yHazardFlashing A11yHazard = "flashing"

	// Indicates that the resource does not present a flashing hazard.
	A11yHazardNoFlashingHazard A11yHazard = "noFlashingHazard"

	// Indicates that the resource contains instances of motion simulation that
	// may affect some individuals.
	//
	// Some examples of motion simulation include video games with a first-person perspective
	// and CSS-controlled backgrounds that move when a user scrolls a page.
	A11yHazardMotionSimulation A11yHazard = "motionSimulation"

	// Indicates that the resource does not contain instances of motion simulation.
	A11yHazardNoMotionSimulationHazard A11yHazard = "noMotionSimulationHazard"

	// Indicates that the resource contains auditory sounds that may affect some individuals.
	A11yHazardSound A11yHazard = "sound"

	// Indicates that the resource does not contain auditory hazards.
	A11yHazardNoSoundHazard A11yHazard = "noSoundHazard"

	// Indicates that the author is not able to determine if the resource presents any hazards.
	A11yHazardUnknown A11yHazard = "unknown"

	// Indicates that the resource does not contain any hazards.
	A11yHazardNone A11yHazard = "none"
)

func A11yHazardsFromStrings(strings []string) []A11yHazard {
	return fromStrings(strings, func(str string) A11yHazard {
		return A11yHazard(str)
	})
}

func fromStrings[T any](strings []string, transform func(string) T) []T {
	res := make([]T, 0, len(strings))
	for _, s := range strings {
		res = append(res, transform(s))
	}
	return res
}
