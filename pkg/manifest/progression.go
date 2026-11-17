package manifest

import "strings"

type ReadingProgression string

const (
	None ReadingProgression = ""
	LTR  ReadingProgression = "ltr"
	RTL  ReadingProgression = "rtl"
)

// isRTLLanguage reports whether a primary language subtag (case-insensitive)
// belongs to a language written right to left.
// https://github.com/readium/readium-css/blob/develop/docs/CSS16-internationalization.md#missing-page-progression-direction
func isRTLLanguage(primary string) bool {
	switch strings.ToLower(primary) {
	case "ar", "fa", "per", "fas", "he", "iw", "yi", "ur", "ps", "sd", "ckb", "ug", "dv", "syr":
		return true
	}
	return false
}

func (r ReadingProgression) correct() ReadingProgression {
	switch r {
	case LTR, RTL:
		return r
	default:
		return None // Default to None if the value is not recognized
	}
}
