package guidednavigation

import (
	"math"
	nurl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/readium/go-toolkit/pkg/util/url"
)

// Media reference utilities, per the "Media References" section of the
// Guided Navigation specification:
//
//   - Audio/video: temporal media fragments, e.g. "audio.mp3#t=10,20"
//     https://www.w3.org/TR/media-frags/#naming-time
//   - Images/video: rectangular spatial media fragments, e.g. "page.jpg#xywh=percent:10,10,20,20"
//     https://www.w3.org/TR/media-frags/#naming-space
//   - Images: polygonal regions, e.g. "page.jpg#xyn=percent:0,50,50,0,100,50"
//     https://idpf.org/epub/renditions/region-nav/#sec-3.5.1
//   - Text: fragment ids ("chapter.html#par1") and text fragments
//     https://wicg.github.io/scroll-to-text-fragment/
//   - Text: css() selector fragments locating an element of an (X)HTML resource,
//     e.g. "chapter.html#css(body%20%3E%20p:nth-child(3))", as emitted by the
//     converter package's WithTextRefLocators option

// The unit of a spatial fragment's coordinates.
type RegionUnit string

const (
	RegionUnitPixel   RegionUnit = "pixel"
	RegionUnitPercent RegionUnit = "percent"
)

// Clip is the temporal fragment of a media resource (the "t" dimension of a
// media fragment, in Normal Play Time format).
type Clip struct {
	Begin *time.Duration // Offset from the start of the media, or nil for its beginning.
	End   *time.Duration // Offset from the start of the media, or nil for its end.
}

// MediaFragment returns the "t" media fragment dimension denoting the clip,
// e.g. "t=10.5,20", or an empty string for an unbounded clip. It can be used
// as the fragment of a media resource URL.
func (c Clip) MediaFragment() string {
	if c.Begin == nil && c.End == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("t=")
	if c.Begin != nil {
		sb.WriteString(FormatNPTTime(*c.Begin))
	}
	if c.End != nil {
		sb.WriteByte(',')
		sb.WriteString(FormatNPTTime(*c.End))
	}
	return sb.String()
}

// FormatNPTTime formats a duration as a Normal Play Time value in seconds,
// rounded to the millisecond, e.g. "123.4".
// https://www.w3.org/TR/media-frags/#npttimedef
func FormatNPTTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := math.Round(d.Seconds()*1000) / 1000
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

// Region is a rectangular spatial fragment of a resource
// (the "xywh" dimension of a media fragment).
type Region struct {
	Unit   RegionUnit
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// A point of a polygonal region.
type Point struct {
	X float64
	Y float64
}

// Polygon is a polygonal region of a resource, as defined by EPUB Region-Based
// Navigation (the "xyn" fragment parameter).
type Polygon struct {
	Unit   RegionUnit
	Points []Point // At least three points.
}

// TextFragment is a text fragment of a textual resource, as defined by the
// URL Fragment Text Directives specification (":~:text=" fragments).
type TextFragment struct {
	Prefix    string // Text immediately before the target, or empty.
	TextStart string // Start of the target text.
	TextEnd   string // End of the target text when it's a range, or empty.
	Suffix    string // Text immediately after the target, or empty.
}

// AudioFile returns the audio resource referenced by the object, without its media fragment.
func (o GuidedNavigationObject) AudioFile() url.URL {
	return refFile(o.AudioRef)
}

// AudioClip returns the temporal fragment of the object's audio reference,
// or nil when the whole resource is referenced.
func (o GuidedNavigationObject) AudioClip() *Clip {
	return refClip(o.AudioRef)
}

// VideoFile returns the video resource referenced by the object, without its media fragment.
func (o GuidedNavigationObject) VideoFile() url.URL {
	return refFile(o.VideoRef)
}

// VideoClip returns the temporal fragment of the object's video reference,
// or nil when no time range is referenced.
func (o GuidedNavigationObject) VideoClip() *Clip {
	return refClip(o.VideoRef)
}

// VideoRegion returns the rectangular fragment of the object's video reference,
// or nil when no region is referenced.
func (o GuidedNavigationObject) VideoRegion() *Region {
	return refRegion(o.VideoRef)
}

// ImageFile returns the image referenced by the object, without its media fragment.
func (o GuidedNavigationObject) ImageFile() url.URL {
	return refFile(o.ImgRef)
}

// ImageRegion returns the rectangular fragment of the object's image reference,
// or nil when no rectangular region is referenced.
func (o GuidedNavigationObject) ImageRegion() *Region {
	return refRegion(o.ImgRef)
}

// ImagePolygon returns the polygonal fragment of the object's image reference,
// or nil when no polygonal region is referenced.
func (o GuidedNavigationObject) ImagePolygon() *Polygon {
	return refPolygon(o.ImgRef)
}

// TextFile returns the textual resource referenced by the object, without its fragment.
func (o GuidedNavigationObject) TextFile() url.URL {
	return refFile(o.TextRef)
}

// TextFragmentID returns the fragment id of the object's text reference
// (e.g. "par1" for "chapter.html#par1"), or an empty string when there is none.
func (o GuidedNavigationObject) TextFragmentID() string {
	return refFragmentID(o.TextRef)
}

// TextFragments returns the text fragments of the object's text reference
// (e.g. "chapter.html#:~:text=some,text"), or nil when there are none.
func (o GuidedNavigationObject) TextFragments() []TextFragment {
	return refTextFragments(o.TextRef)
}

// TextCSSSelector returns the CSS selector of the object's text reference
// (e.g. "body > p:nth-child(3)" for "chapter.html#css(body%20%3E%20p:nth-child(3))"),
// or an empty string when the reference has no css() fragment.
func (o GuidedNavigationObject) TextCSSSelector() string {
	return refCSSSelector(o.TextRef)
}

// AudioFile returns the audio resource referenced by the description, without its media fragment.
func (d GuidedNavigationDescription) AudioFile() url.URL {
	return refFile(d.AudioRef)
}

// AudioClip returns the temporal fragment of the description's audio reference,
// or nil when the whole resource is referenced.
func (d GuidedNavigationDescription) AudioClip() *Clip {
	return refClip(d.AudioRef)
}

// VideoFile returns the video resource referenced by the description, without its media fragment.
func (d GuidedNavigationDescription) VideoFile() url.URL {
	return refFile(d.VideoRef)
}

// VideoClip returns the temporal fragment of the description's video reference,
// or nil when no time range is referenced.
func (d GuidedNavigationDescription) VideoClip() *Clip {
	return refClip(d.VideoRef)
}

// VideoRegion returns the rectangular fragment of the description's video reference,
// or nil when no region is referenced.
func (d GuidedNavigationDescription) VideoRegion() *Region {
	return refRegion(d.VideoRef)
}

// ImageFile returns the image referenced by the description, without its media fragment.
func (d GuidedNavigationDescription) ImageFile() url.URL {
	return refFile(d.ImgRef)
}

// ImageRegion returns the rectangular fragment of the description's image reference,
// or nil when no rectangular region is referenced.
func (d GuidedNavigationDescription) ImageRegion() *Region {
	return refRegion(d.ImgRef)
}

// ImagePolygon returns the polygonal fragment of the description's image reference,
// or nil when no polygonal region is referenced.
func (d GuidedNavigationDescription) ImagePolygon() *Polygon {
	return refPolygon(d.ImgRef)
}

// TextFile returns the textual resource referenced by the description, without its fragment.
func (d GuidedNavigationDescription) TextFile() url.URL {
	return refFile(d.TextRef)
}

// TextFragmentID returns the fragment id of the description's text reference,
// or an empty string when there is none.
func (d GuidedNavigationDescription) TextFragmentID() string {
	return refFragmentID(d.TextRef)
}

// TextFragments returns the text fragments of the description's text reference,
// or nil when there are none.
func (d GuidedNavigationDescription) TextFragments() []TextFragment {
	return refTextFragments(d.TextRef)
}

// TextCSSSelector returns the CSS selector of the description's text reference,
// or an empty string when the reference has no css() fragment.
func (d GuidedNavigationDescription) TextCSSSelector() string {
	return refCSSSelector(d.TextRef)
}

// The reference without its fragment.
func refFile(ref url.URL) url.URL {
	if ref == nil {
		return nil
	}
	return ref.RemoveFragment()
}

type fragmentDimension struct {
	name  string
	value string
}

// The name-value dimensions of a media fragment, e.g. "t=10,20&xywh=0,0,5,5",
// percent-decoded after splitting, in document order.
// https://www.w3.org/TR/media-frags/#processing-name-value-lists
func fragmentDimensions(ref url.URL) []fragmentDimension {
	if ref == nil {
		return nil
	}
	fragment := ref.Raw().EscapedFragment()
	if fragment == "" {
		return nil
	}
	var res []fragmentDimension
	for pair := range strings.SplitSeq(fragment, "&") {
		name, value, found := strings.Cut(pair, "=")
		if !found || name == "" {
			continue
		}
		var err error
		if name, err = nurl.PathUnescape(name); err != nil {
			continue
		}
		if value, err = nurl.PathUnescape(value); err != nil {
			continue
		}
		res = append(res, fragmentDimension{name: name, value: value})
	}
	return res
}

// The parsed values of a fragment dimension: only the last valid occurrence
// of a dimension is interpreted, the others are ignored.
// https://www.w3.org/TR/media-frags/#error-uri-general
func lastValidDimension[T any](ref url.URL, name string, parse func(string) *T) *T {
	dims := fragmentDimensions(ref)
	for i := len(dims) - 1; i >= 0; i-- {
		if dims[i].name != name {
			continue
		}
		if res := parse(dims[i].value); res != nil {
			return res
		}
	}
	return nil
}

func refClip(ref url.URL) *Clip {
	return lastValidDimension(ref, "t", parseClipValue)
}

func parseClipValue(value string) *Clip {
	// Only the Normal Play Time format is supported, which is also the default:
	// https://www.w3.org/TR/media-frags/#naming-time
	if prefix, rest, found := strings.Cut(value, ":"); found {
		if prefix == "npt" {
			value = rest
		} else if isAlpha(prefix) {
			// A time format like "smpte" or "clock" (unsupported). Anything else
			// is a colon belonging to an hh:mm:ss time value.
			return nil
		}
	}

	begin, end, hasEnd := strings.Cut(value, ",")
	clip := &Clip{}
	if begin != "" {
		d, ok := parseNPTTime(begin)
		if !ok {
			return nil
		}
		clip.Begin = &d
	}
	if hasEnd {
		if end == "" {
			return nil
		}
		d, ok := parseNPTTime(end)
		if !ok {
			return nil
		}
		clip.End = &d
	}
	if clip.Begin == nil && clip.End == nil {
		return nil
	}
	// The begin time (0 when omitted) must precede the end time
	if clip.End != nil {
		var begin time.Duration
		if clip.Begin != nil {
			begin = *clip.Begin
		}
		if *clip.End <= begin {
			return nil
		}
	}
	return clip
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// Parses a value of the form 1*DIGIT ["." *DIGIT], the only number format the
// media fragment grammars allow. strconv.ParseFloat alone would also accept
// signs, exponents, hexadecimal floats, "inf" and "nan".
func parseDecimal(s string) (float64, bool) {
	dot := false
	for i, r := range s {
		if r == '.' {
			if dot || i == 0 {
				return 0, false
			}
			dot = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	if s == "" || s == "." {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// The largest amount of seconds representable as a time.Duration.
const maxClipSeconds = float64(int64(^uint64(0)>>1) / int64(time.Second))

// Parses a Normal Play Time value: seconds ("123.45"), "mm:ss" or "hh:mm:ss",
// with an optional fraction. https://www.w3.org/TR/media-frags/#npttimedef
func parseNPTTime(s string) (time.Duration, bool) {
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, false
	}

	// The last part holds the seconds, with an optional fraction
	seconds, ok := parseDecimal(parts[len(parts)-1])
	if !ok {
		return 0, false
	}
	if len(parts) > 1 && seconds >= 60 {
		return 0, false
	}

	for i, mul := range []float64{60, 3600} {
		idx := len(parts) - 2 - i
		if idx < 0 {
			break
		}
		v, err := strconv.ParseUint(parts[idx], 10, 32)
		if err != nil {
			return 0, false
		}
		if i == 0 && len(parts) > 1 && v >= 60 {
			// Minutes are bound to 0-59
			return 0, false
		}
		seconds += float64(v) * mul
	}

	if seconds > maxClipSeconds {
		return 0, false
	}
	return time.Duration(seconds * float64(time.Second)), true
}

// Parses the unit prefix of a spatial fragment value, defaulting to pixels.
func parseRegionUnit(value string) (RegionUnit, string, bool) {
	if prefix, rest, found := strings.Cut(value, ":"); found {
		switch RegionUnit(prefix) {
		case RegionUnitPixel:
			return RegionUnitPixel, rest, true
		case RegionUnitPercent:
			return RegionUnitPercent, rest, true
		default:
			return "", "", false
		}
	}
	return RegionUnitPixel, value, true
}

// Parses a comma-separated list of non-negative decimal values.
func parseCoordinates(value string, count int) ([]float64, bool) {
	parts := strings.Split(value, ",")
	if count > 0 && len(parts) != count {
		return nil, false
	}
	res := make([]float64, len(parts))
	for i, part := range parts {
		v, ok := parseDecimal(part)
		if !ok {
			return nil, false
		}
		res[i] = v
	}
	return res, true
}

func refRegion(ref url.URL) *Region {
	return lastValidDimension(ref, "xywh", parseRegionValue)
}

func parseRegionValue(value string) *Region {
	unit, value, ok := parseRegionUnit(value)
	if !ok {
		return nil
	}
	coords, ok := parseCoordinates(value, 4)
	if !ok {
		return nil
	}
	return &Region{
		Unit:   unit,
		X:      coords[0],
		Y:      coords[1],
		Width:  coords[2],
		Height: coords[3],
	}
}

func refPolygon(ref url.URL) *Polygon {
	return lastValidDimension(ref, "xyn", parsePolygonValue)
}

func parsePolygonValue(value string) *Polygon {
	unit, value, ok := parseRegionUnit(value)
	if !ok {
		return nil
	}
	coords, ok := parseCoordinates(value, -1)
	if !ok || len(coords) < 6 || len(coords)%2 != 0 {
		// A polygon needs at least three x,y pairs
		return nil
	}
	res := &Polygon{
		Unit:   unit,
		Points: make([]Point, len(coords)/2),
	}
	for i := range res.Points {
		res.Points[i] = Point{X: coords[i*2], Y: coords[i*2+1]}
	}
	return res
}

// The delimiter of a URL fragment text directive.
const textDirectiveDelimiter = ":~:"

func refFragmentID(ref url.URL) string {
	if ref == nil {
		return ""
	}
	fragment := ref.Raw().EscapedFragment()
	// A fragment directive doesn't belong to the fragment id preceding it
	fragment, _, _ = strings.Cut(fragment, textDirectiveDelimiter)
	if decoded, err := nurl.PathUnescape(fragment); err == nil {
		fragment = decoded
	}
	if _, ok := cssSelectorFragment(fragment); ok {
		// A css() fragment is a selector, not an id
		return ""
	}
	return fragment
}

// The selector of a css() fragment, e.g. "css(body > p)" -> "body > p".
// The fragment is expected in decoded form.
func cssSelectorFragment(fragment string) (string, bool) {
	inner, ok := strings.CutPrefix(fragment, "css(")
	if !ok {
		return "", false
	}
	inner, ok = strings.CutSuffix(inner, ")")
	if !ok {
		return "", false
	}
	return inner, true
}

func refCSSSelector(ref url.URL) string {
	if ref == nil {
		return ""
	}
	fragment := ref.Fragment()
	// A fragment directive doesn't belong to the fragment preceding it
	fragment, _, _ = strings.Cut(fragment, textDirectiveDelimiter)
	sel, _ := cssSelectorFragment(fragment)
	return sel
}

func refTextFragments(ref url.URL) []TextFragment {
	if ref == nil {
		return nil
	}
	// The directive must be parsed in percent-encoded form: its delimiters
	// (",", "-", "&") are only syntax when they appear unencoded
	_, directive, found := strings.Cut(ref.Raw().EscapedFragment(), textDirectiveDelimiter)
	if !found {
		return nil
	}

	var res []TextFragment
	for part := range strings.SplitSeq(directive, "&") {
		value, found := strings.CutPrefix(part, "text=")
		if !found {
			continue
		}
		if fragment, ok := parseTextFragment(value); ok {
			res = append(res, fragment)
		}
	}
	return res
}

// Parses a text directive value: [prefix-,]textStart[,textEnd][,-suffix]
// https://wicg.github.io/scroll-to-text-fragment/#syntax
func parseTextFragment(value string) (fragment TextFragment, ok bool) {
	parts := strings.Split(value, ",")

	// Prefix and suffix tokens are extracted unconditionally: an invalid
	// remainder rejects the whole directive
	if prefix, found := strings.CutSuffix(parts[0], "-"); found {
		if !validTextDirectiveTerm(prefix) {
			return TextFragment{}, false
		}
		fragment.Prefix = decodeTextDirectiveTerm(prefix)
		parts = parts[1:]
	}
	if len(parts) > 0 {
		if suffix, found := strings.CutPrefix(parts[len(parts)-1], "-"); found {
			if !validTextDirectiveTerm(suffix) {
				return TextFragment{}, false
			}
			fragment.Suffix = decodeTextDirectiveTerm(suffix)
			parts = parts[:len(parts)-1]
		}
	}

	switch len(parts) {
	case 1:
		fragment.TextStart = decodeTextDirectiveTerm(parts[0])
	case 2:
		fragment.TextStart = decodeTextDirectiveTerm(parts[0])
		fragment.TextEnd = decodeTextDirectiveTerm(parts[1])
		if !validTextDirectiveTerm(parts[1]) {
			return TextFragment{}, false
		}
	default:
		return TextFragment{}, false
	}
	if !validTextDirectiveTerm(parts[0]) {
		return TextFragment{}, false
	}
	return fragment, true
}

// A text directive term must be non-empty, with any dash it contains
// percent-encoded to keep it distinguishable from the prefix/suffix markers.
func validTextDirectiveTerm(s string) bool {
	return s != "" && !strings.Contains(s, "-")
}

func decodeTextDirectiveTerm(s string) string {
	if decoded, err := nurl.PathUnescape(s); err == nil {
		return decoded
	}
	return s
}
