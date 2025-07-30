package manifest

import "slices"

type Layout string

const (
	LayoutNone       Layout = ""           // No layout specified, reading systems should use their default layout.
	LayoutReflowable Layout = "reflowable" // Reading systems are free to adapt text and layout entirely based on user preferences.
	LayoutFixed      Layout = "fixed"      // Each resource is a "page" where both dimensions are usually contained in the device's viewport. Based on user preferences, the reading system may also display two resources side by side in a spread.
	LayoutScrolled   Layout = "scrolled"   // Resources are displayed in a continuous scroll, usually by filling the width of the viewport, without any visible gap between between spine items.
)

// Correct the layout value based on the provided profiles.
func (l Layout) correct(profiles Profiles) Layout {
	if len(profiles) == 0 {
		return l
	}

	// Make sure layout has a valid value, otherwise ignore it
	switch l {
	case LayoutNone, LayoutReflowable, LayoutFixed, LayoutScrolled:
	default:
		return LayoutNone
	}

	if slices.ContainsFunc(profiles, func(p Profile) bool {
		return p == ProfilePDF || p == ProfileAudiobook
	}) {
		// For the audiobook and PDF profiles, we ignore the layout
		return LayoutNone
	}

	if slices.Contains(profiles, ProfileDivina) && l == LayoutReflowable {
		// We ignore the value if layout is set to reflowable on a Divina
		return LayoutNone
	}

	return l
}

// Determines the actual layout value based on the provided profiles.
func (l Layout) EffectiveValue(profiles Profiles) Layout {
	l = l.correct(profiles)

	if l == LayoutNone {
		// Divina profile defaults to fixed if layout is not present
		if slices.Contains(profiles, ProfileDivina) {
			return LayoutFixed
		}

		// EPUB profile defaults to reflowable if layout is not present
		if slices.Contains(profiles, ProfileEPUB) {
			return LayoutReflowable
		}
	}

	return l
}

// Determines the minimal layout value based on the provided profiles.
func (l Layout) minimalValue(profiles Profiles) Layout {
	l = l.correct(profiles)

	// Divina profile defaults to fixed if layout is not present
	if slices.Contains(profiles, ProfileDivina) && l == LayoutFixed {
		return LayoutNone
	}

	// EPUB profile defaults to reflowable if layout is not present
	if slices.Contains(profiles, ProfileEPUB) && l == LayoutReflowable {
		return LayoutNone
	}

	return l
}
