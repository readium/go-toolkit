package protection

//go:generate stringer -type Scheme

type Scheme int

const (
	NoDRM Scheme = iota
	Generic
	LCP
	Adept
	BarnesAndNoble
	Fairplay
	Kobo
)

// URI returns the canonical namespace URI for the DRM scheme, suitable for
// populating [manifest.Encryption]'s Scheme field. Returns the empty string for
// [NoDRM] and [Generic] (no recognized DRM identifier).
func (s Scheme) URI() string {
	switch s {
	case LCP:
		return SchemeLCP
	case Adept, BarnesAndNoble:
		return SchemeAdept
	case Fairplay:
		return SchemeFairPlay
	default:
		return ""
	}
}
