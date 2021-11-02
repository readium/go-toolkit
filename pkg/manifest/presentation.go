package manifest

import "encoding/json"

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
	Layout      *EpubLayout  `json:"layout,omitempty"`      // Hints how the layout of the resource should be presented (EPUB extension).
}

const PRESENTATION_DEFAULT_CLIPPED = false    // Default value for Presentation.Clipped
const PRESENTATION_DEFAULT_CONTINUOUS = false // Default value for Presentation.Continuous

type Fit string // Suggested method for constraining a resource inside the viewport.
const (
	FIT_WIDTH   Fit = "width"
	FIT_HEIGTH  Fit = "height"
	FIT_CONTAIN Fit = "contain"
	FIT_COVER   Fit = "cover"
)

type Orientation string // Suggested orientation for the device when displaying the linked resource.
const (
	ORIENTATION_AUTO      Orientation = "auto"
	ORIENTATION_LANDSCAPE Orientation = "landscape"
	ORIENTATION_PORTRAIT  Orientation = "portrait"
)

type Overflow string // Suggested method for handling overflow while displaying the linked resource.
const (
	OVERFLOW_AUTO      Overflow = "auto"
	OVERFLOW_PAGINATED Overflow = "paginated"
	OVERFLOW_SCROLLED  Overflow = "scrolled"
)

type Page string // Indicates how the linked resource should be displayed in a reading environment that displays synthetic spreads.
const (
	PAGE_LEFT   Page = "left"
	PAGE_RIGHT  Page = "right"
	PAGE_CENTER Page = "center"
)

type Spread string // Indicates the condition to be met for the linked resource to be rendered within a synthetic spread.
const (
	SPREAD_AUTO      Spread = "auto"
	SPREAD_BOTH      Spread = "both"
	SPREAD_NONE      Spread = "none"
	SPREAD_LANDSCAPE Spread = "landscape"
)

// Hints how the layout of the resource should be presented.
// https://readium.org/webpub-manifest/schema/extensions/epub/metadata.schema.json
type EpubLayout string

const (
	EPUB_LAYOUT_FIXED      EpubLayout = "fixed"
	EPUB_LAYOUT_REFLOWABLE EpubLayout = "reflowable"
)

func (p *Presentation) UnmarshalJSON(data []byte) error {
	type PT Presentation
	if err := json.Unmarshal(data, (*PT)(p)); err != nil {
		return err
	}

	if p.Fit == nil {
		def := FIT_CONTAIN // Default value for [Fit], if not specified.
		p.Fit = &def
	}

	if p.Orientation == nil {
		def := ORIENTATION_AUTO
		p.Orientation = &def
	}

	if p.Overflow == nil {
		def := OVERFLOW_AUTO
		p.Overflow = &def
	}

	if p.Spread == nil {
		def := SPREAD_AUTO
		p.Spread = &def
	}

	if p.Clipped == nil {
		def := PRESENTATION_DEFAULT_CLIPPED
		p.Clipped = &def
	}

	if p.Continuous == nil {
		def := PRESENTATION_DEFAULT_CONTINUOUS
		p.Continuous = &def
	}

	return nil
}

// Get the layout of the given resource in this publication. Falls back on REFLOWABLE.
func LayoutOf(link Link) EpubLayout {
	if l := link.Properties.Layout(); l != "" {
		return l
	}
	return EPUB_LAYOUT_REFLOWABLE
}
