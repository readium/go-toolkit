package manifest

import (
	"encoding/json"
)

/**
 * The Presentation Hints extension defines a number of hints for User Agents about the way content
 * should be presented to the user.
 *
 * https://readium.org/webpub-manifest/modules/presentation.html
 * https://readium.org/webpub-manifest/schema/extensions/presentation/metadata.schema.json
 *
 * These properties are nullable to avoid having default values when it doesn't make sense for a
 * given [Publication]. If a navigator needs a default value when not specified,
 * Presentation.DEFAULT_X and Presentation.X.DEFAULT can be used.
 */
type Presentation struct {
	Clipped     *bool        `json:"clipped,omitempty"`     // Specifies whether or not the parts of a linked resource that flow out of the viewport are clipped.
	Continuous  *bool        `json:"continuous,omitempty"`  // Indicates how the progression between resources from the [readingOrder] should be handled.
	Fit         *Fit         `json:"fit,omitempty"`         // Suggested method for constraining a resource inside the viewport.
	Orientation *Orientation `json:"orientation,omitempty"` // Suggested orientation for the device when displaying the linked resource.
	Overflow    *Overflow    `json:"overflow,omitempty"`    // Suggested method for handling overflow while displaying the linked resource.
	Spread      *Spread      `json:"spread,omitempty"`      // Indicates the condition to be met for the linked resource to be rendered within a synthetic spread.
	Layout      *EPUBLayout  `json:"layout,omitempty"`      // Hints how the layout of the resource should be presented (EPUB extension).
}

const PresentationDefaultClipped = false    // Default value for Presentation.Clipped
const PresentationDefaultContinuous = false // Default value for Presentation.Continuous

type Fit string // Suggested method for constraining a resource inside the viewport.
const (
	FitWidth   Fit = "width"
	FitHeight  Fit = "height"
	FitContain Fit = "contain"
	FitCover   Fit = "cover"
)

type Orientation string // Suggested orientation for the device when displaying the linked resource.
const (
	OrientationAuto      Orientation = "auto"
	OrientationLandscape Orientation = "landscape"
	OrientationPortrait  Orientation = "portrait"
)

type Overflow string // Suggested method for handling overflow while displaying the linked resource.
const (
	OverflowAuto      Overflow = "auto"
	OverflowPaginated Overflow = "paginated"
	OverflowScrolled  Overflow = "scrolled"
)

type Page string // Indicates how the linked resource should be displayed in a reading environment that displays synthetic spreads.
const (
	PageLeft   Page = "left"
	PageRight  Page = "right"
	PageCenter Page = "center"
)

type Spread string // Indicates the condition to be met for the linked resource to be rendered within a synthetic spread.
const (
	SpreadAuto      Spread = "auto"
	SpreadBoth      Spread = "both"
	SpreadNone      Spread = "none"
	SpreadLandscape Spread = "landscape"
)

// Hints how the layout of the resource should be presented.
// https://readium.org/webpub-manifest/schema/extensions/epub/metadata.schema.json
type EPUBLayout string

const (
	EPUBLayoutFixed      EPUBLayout = "fixed"
	EPUBLayoutReflowable EPUBLayout = "reflowable"
)

func (p *Presentation) setDefaults() {
	if p.Fit == nil {
		def := FitContain // Default value for [Fit], if not specified.
		p.Fit = &def
	}

	if p.Orientation == nil {
		def := OrientationAuto
		p.Orientation = &def
	}

	if p.Overflow == nil {
		def := OverflowAuto
		p.Overflow = &def
	}

	if p.Spread == nil {
		def := SpreadAuto
		p.Spread = &def
	}

	if p.Clipped == nil {
		def := PresentationDefaultClipped
		p.Clipped = &def
	}

	if p.Continuous == nil {
		def := PresentationDefaultContinuous
		p.Continuous = &def
	}
}

func NewPresentation() *Presentation {
	p := &Presentation{}
	p.setDefaults()
	return p
}

func (p *Presentation) UnmarshalJSON(data []byte) error {
	type PT Presentation
	if err := json.Unmarshal(data, (*PT)(p)); err != nil {
		return err
	}

	p.setDefaults()

	return nil
}

func (p Presentation) MarshalJSON() ([]byte, error) {
	if nilstrEq((*string)(p.Fit), string(FitContain)) {
		p.Fit = nil
	}

	if nilstrEq((*string)(p.Orientation), string(OrientationAuto)) {
		p.Orientation = nil
	}

	if nilstrEq((*string)(p.Overflow), string(OverflowAuto)) {
		p.Overflow = nil
	}

	if nilstrEq((*string)(p.Spread), string(SpreadAuto)) {
		p.Spread = nil
	}

	if nilboolEq(p.Clipped, PresentationDefaultClipped) {
		p.Clipped = nil
	}

	if nilboolEq(p.Continuous, PresentationDefaultContinuous) {
		p.Continuous = nil
	}

	type alias Presentation
	return json.Marshal(alias(p))
}

// Get the layout of the given resource in this publication. Falls back on REFLOWABLE.
func (p Presentation) LayoutOf(link Link) EPUBLayout {
	if l := link.Properties.Layout(); l != "" {
		return l
	}
	if p.Layout != nil && *p.Layout != "" {
		return *p.Layout
	}
	return EPUBLayoutReflowable
}
