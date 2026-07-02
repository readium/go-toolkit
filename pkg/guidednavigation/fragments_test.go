package guidednavigation

import (
	"testing"
	"time"

	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func objWithAudioRef(t *testing.T, ref string) GuidedNavigationObject {
	t.Helper()
	return GuidedNavigationObject{AudioRef: url.MustURLFromString(ref)}
}

func duration(seconds float64) *time.Duration {
	d := time.Duration(seconds * float64(time.Second))
	return &d
}

func TestAudioClip(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		clip *Clip
	}{
		{"begin and end", "chapter1.mp3#t=0,20", &Clip{Begin: duration(0), End: duration(20)}},
		{"begin only", "chapter1.mp3#t=10", &Clip{Begin: duration(10)}},
		{"end only", "chapter1.mp3#t=,20", &Clip{End: duration(20)}},
		{"npt prefix", "chapter1.mp3#t=npt:10,20", &Clip{Begin: duration(10), End: duration(20)}},
		{"fractional seconds", "chapter1.mp3#t=10.5,21.25", &Clip{Begin: duration(10.5), End: duration(21.25)}},
		{"mm:ss", "chapter1.mp3#t=1:30,2:00", &Clip{Begin: duration(90), End: duration(120)}},
		{"hh:mm:ss with fraction", "chapter1.mp3#t=0:02:00.5,0:02:01", &Clip{Begin: duration(120.5), End: duration(121)}},
		{"end only with colons", "chapter1.mp3#t=,0:02:00", &Clip{End: duration(120)}},
		{"other dimensions around", "chapter1.mp3#u=v&t=5,6&x=y", &Clip{Begin: duration(5), End: duration(6)}},
		{"last occurrence wins", "chapter1.mp3#t=1,2&t=5,6", &Clip{Begin: duration(5), End: duration(6)}},

		{"no fragment", "chapter1.mp3", nil},
		{"no t dimension", "chapter1.mp3#xywh=0,0,1,1", nil},
		{"empty value", "chapter1.mp3#t=", nil},
		{"end before begin", "chapter1.mp3#t=20,10", nil},
		{"zero length", "chapter1.mp3#t=5,5", nil},
		{"smpte unsupported", "chapter1.mp3#t=smpte:00:01:00,00:02:00", nil},
		{"clock unsupported", "chapter1.mp3#t=clock:2026-07-01T00:00:00Z", nil},
		{"garbage", "chapter1.mp3#t=abc", nil},
		{"negative", "chapter1.mp3#t=-5,10", nil},
		{"seconds out of range in mm:ss", "chapter1.mp3#t=1:75", nil},
		{"minutes out of range in hh:mm:ss", "chapter1.mp3#t=1:75:00", nil},
		{"minutes out of range in mm:ss", "chapter1.mp3#t=75:00", nil},
		{"trailing comma", "chapter1.mp3#t=10,", nil},

		// The NPT grammar allows nothing but digits and a fraction
		{"nan", "chapter1.mp3#t=nan,10", nil},
		{"inf", "chapter1.mp3#t=inf", nil},
		{"exponent", "chapter1.mp3#t=1e3", nil},
		{"hex float", "chapter1.mp3#t=0x1p4", nil},
		{"explicit sign", "chapter1.mp3#t=+5", nil},
		{"bare fraction", "chapter1.mp3#t=.5", nil},
		{"duration overflow", "chapter1.mp3#t=10000000000", nil},
		{"zero-length end-only", "chapter1.mp3#t=,0", nil},

		// Percent-encoded dimensions are decoded before parsing
		{"encoded npt prefix", "chapter1.mp3#t=npt%3A10,20", &Clip{Begin: duration(10), End: duration(20)}},
		{"encoded comma", "chapter1.mp3#t=10%2C20", &Clip{Begin: duration(10), End: duration(20)}},
		// Only the last VALID occurrence of a dimension counts
		{"invalid last occurrence ignored", "chapter1.mp3#t=10,20&t=abc", &Clip{Begin: duration(10), End: duration(20)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.clip, objWithAudioRef(t, tt.ref).AudioClip())
		})
	}
}

func TestClipMediaFragment(t *testing.T) {
	assert.Equal(t, "", Clip{}.MediaFragment())
	assert.Equal(t, "t=10", Clip{Begin: duration(10)}.MediaFragment())
	assert.Equal(t, "t=,20", Clip{End: duration(20)}.MediaFragment())
	assert.Equal(t, "t=10.5,20", Clip{Begin: duration(10.5), End: duration(20)}.MediaFragment())
	// Rounded to the millisecond, without trailing zeros
	assert.Equal(t, "t=62.011,1647.2", Clip{Begin: duration(62.0109), End: duration(1647.2)}.MediaFragment())

	// A formatted clip parses back to the same value
	clips := []Clip{
		{Begin: duration(10)},
		{End: duration(20)},
		{Begin: duration(0), End: duration(20)},
		{Begin: duration(90.25), End: duration(120.5)},
	}
	for _, clip := range clips {
		ref := url.MustURLFromString("chapter1.mp3#" + clip.MediaFragment())
		parsed := GuidedNavigationObject{AudioRef: ref}.AudioClip()
		require.NotNil(t, parsed, clip.MediaFragment())
		assert.Equal(t, &clip, parsed, clip.MediaFragment())
	}
}

func TestFormatNPTTime(t *testing.T) {
	assert.Equal(t, "0", FormatNPTTime(0))
	assert.Equal(t, "0", FormatNPTTime(-5*time.Second))
	assert.Equal(t, "90.5", FormatNPTTime(90*time.Second+500*time.Millisecond))
	assert.Equal(t, "0.001", FormatNPTTime(1400*time.Microsecond))
}

func TestClipOfVideoRef(t *testing.T) {
	obj := GuidedNavigationObject{VideoRef: url.MustURLFromString("movie.mp4#t=30,60")}
	assert.Equal(t, &Clip{Begin: duration(30), End: duration(60)}, obj.VideoClip())
	assert.Nil(t, obj.AudioClip())
}

func TestAudioFile(t *testing.T) {
	obj := objWithAudioRef(t, "audio/chapter1.mp3#t=0,20")
	assert.Equal(t, "audio/chapter1.mp3", obj.AudioFile().String())
	// The original reference must not lose its fragment
	assert.Equal(t, "audio/chapter1.mp3#t=0,20", obj.AudioRef.String())

	// Without a fragment the reference is returned as-is
	obj = objWithAudioRef(t, "audio/chapter1.mp3")
	assert.Equal(t, "audio/chapter1.mp3", obj.AudioFile().String())

	assert.Nil(t, GuidedNavigationObject{}.AudioFile())
}

func TestImageRegion(t *testing.T) {
	tests := []struct {
		name   string
		ref    string
		region *Region
	}{
		// Example from the specification's README
		{"percent", "page10.jpg#xywh=percent:10,10,20,20",
			&Region{Unit: RegionUnitPercent, X: 10, Y: 10, Width: 20, Height: 20}},
		// The comics example uses decimal percentages
		{"decimal percent", "page1.jpg#xywh=percent:4.1,50.3,91.8,21.5",
			&Region{Unit: RegionUnitPercent, X: 4.1, Y: 50.3, Width: 91.8, Height: 21.5}},
		{"explicit pixel", "page.jpg#xywh=pixel:10,20,30,40",
			&Region{Unit: RegionUnitPixel, X: 10, Y: 20, Width: 30, Height: 40}},
		{"default pixel", "page.jpg#xywh=10,20,30,40",
			&Region{Unit: RegionUnitPixel, X: 10, Y: 20, Width: 30, Height: 40}},

		{"encoded unit prefix", "page.jpg#xywh=percent%3A10,10,20,20",
			&Region{Unit: RegionUnitPercent, X: 10, Y: 10, Width: 20, Height: 20}},

		{"no fragment", "page.jpg", nil},
		{"wrong dimension", "page.jpg#t=1,2", nil},
		{"bad unit", "page.jpg#xywh=inch:1,2,3,4", nil},
		{"too few values", "page.jpg#xywh=1,2,3", nil},
		{"too many values", "page.jpg#xywh=1,2,3,4,5", nil},
		{"negative value", "page.jpg#xywh=-1,2,3,4", nil},
		{"garbage", "page.jpg#xywh=a,b,c,d", nil},
		{"nan", "page.jpg#xywh=nan,0,1,1", nil},
		{"inf", "page.jpg#xywh=inf,0,10,10", nil},
		{"exponent", "page.jpg#xywh=0,0,1e2,50", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := GuidedNavigationObject{ImgRef: url.MustURLFromString(tt.ref)}
			assert.Equal(t, tt.region, obj.ImageRegion())
		})
	}
}

func TestImagePolygon(t *testing.T) {
	// Example from EPUB Region-Based Navigation
	obj := GuidedNavigationObject{ImgRef: url.MustURLFromString("page05.xhtml#xyn=percent:0,50,50,0,100,50")}
	require.NotNil(t, obj.ImagePolygon())
	assert.Equal(t, &Polygon{
		Unit:   RegionUnitPercent,
		Points: []Point{{X: 0, Y: 50}, {X: 50, Y: 0}, {X: 100, Y: 50}},
	}, obj.ImagePolygon())

	// Pixels by default
	obj = GuidedNavigationObject{ImgRef: url.MustURLFromString("page.jpg#xyn=0,0,10,0,10,10,0,10")}
	polygon := obj.ImagePolygon()
	require.NotNil(t, polygon)
	assert.Equal(t, RegionUnitPixel, polygon.Unit)
	assert.Len(t, polygon.Points, 4)

	for name, ref := range map[string]string{
		"odd number of values": "page.jpg#xyn=percent:0,50,50",
		"fewer than 3 points":  "page.jpg#xyn=percent:0,50,50,0",
		"no fragment":          "page.jpg",
		"garbage":              "page.jpg#xyn=percent:a,b,c,d,e,f",
	} {
		t.Run(name, func(t *testing.T) {
			obj := GuidedNavigationObject{ImgRef: url.MustURLFromString(ref)}
			assert.Nil(t, obj.ImagePolygon())
		})
	}
}

func TestTextFragmentID(t *testing.T) {
	tests := []struct {
		ref string
		id  string
	}{
		{"chapter1.html#start", "start"},
		{"chapter1.html#par%201", "par 1"},
		{"chapter1.html#id:~:text=highlighted", "id"},
		{"chapter1.html#:~:text=highlighted", ""},
		{"chapter1.html", ""},
	}
	for _, tt := range tests {
		obj := GuidedNavigationObject{TextRef: url.MustURLFromString(tt.ref)}
		assert.Equal(t, tt.id, obj.TextFragmentID(), tt.ref)
	}
	assert.Empty(t, GuidedNavigationObject{}.TextFragmentID())
}

func TestTextFragments(t *testing.T) {
	// Example from the specification's README
	obj := GuidedNavigationObject{TextRef: url.MustURLFromString(
		"chapter1.html#:~:text=It%20is%20a%20truth,want%20of%20a%20wife")}
	assert.Equal(t, []TextFragment{
		{TextStart: "It is a truth", TextEnd: "want of a wife"},
	}, obj.TextFragments())

	// All four parts
	obj = GuidedNavigationObject{TextRef: url.MustURLFromString(
		"chapter1.html#:~:text=before-,start%20text,end%20text,-after")}
	assert.Equal(t, []TextFragment{
		{Prefix: "before", TextStart: "start text", TextEnd: "end text", Suffix: "after"},
	}, obj.TextFragments())

	// Start only, with a percent-encoded comma and dash kept as content
	obj = GuidedNavigationObject{TextRef: url.MustURLFromString(
		"chapter1.html#:~:text=one%2C%20two%2Dthree")}
	assert.Equal(t, []TextFragment{
		{TextStart: "one, two-three"},
	}, obj.TextFragments())

	// Multiple directives
	obj = GuidedNavigationObject{TextRef: url.MustURLFromString(
		"chapter1.html#:~:text=first&text=second")}
	assert.Equal(t, []TextFragment{
		{TextStart: "first"},
		{TextStart: "second"},
	}, obj.TextFragments())

	// Not text fragments
	for _, ref := range []string{
		"chapter1.html#start",
		"chapter1.html",
		"chapter1.html#:~:unknown=x",
	} {
		obj := GuidedNavigationObject{TextRef: url.MustURLFromString(ref)}
		assert.Nil(t, obj.TextFragments(), ref)
	}

	// Invalid directives, per the WICG parsing algorithm
	for _, ref := range []string{
		"chapter1.html#:~:text=foo-",      // Prefix without a start
		"chapter1.html#:~:text=-bar",      // Suffix without a start
		"chapter1.html#:~:text=a-,-b",     // Prefix and suffix without a start
		"chapter1.html#:~:text=foo,",      // Empty end
		"chapter1.html#:~:text=two-three", // Unencoded dash in the start
		"chapter1.html#:~:text=a,b,c,d,e", // Too many terms
		"chapter1.html#:~:text=-,foo",     // Empty prefix
	} {
		obj := GuidedNavigationObject{TextRef: url.MustURLFromString(ref)}
		assert.Nil(t, obj.TextFragments(), ref)
	}
}

func TestDescriptionMediaReferences(t *testing.T) {
	// From the comics example of the specification
	d := GuidedNavigationDescription{
		AudioRef: url.MustURLFromString("audio/page1-panel1-description.mp3#t=0,5"),
		ImgRef:   url.MustURLFromString("page1.jpg#xywh=percent:4.1,4.1,91.8,44.5"),
	}
	assert.Equal(t, "audio/page1-panel1-description.mp3", d.AudioFile().String())
	assert.Equal(t, &Clip{Begin: duration(0), End: duration(5)}, d.AudioClip())
	assert.Equal(t, &Region{Unit: RegionUnitPercent, X: 4.1, Y: 4.1, Width: 91.8, Height: 44.5}, d.ImageRegion())
	assert.Equal(t, "page1.jpg", d.ImageFile().String())
	assert.Nil(t, d.TextFile())
	assert.Empty(t, d.TextFragmentID())

	d = GuidedNavigationDescription{VideoRef: url.MustURLFromString("clip.mp4#xywh=0,0,100,100")}
	assert.Equal(t, &Region{Unit: RegionUnitPixel, X: 0, Y: 0, Width: 100, Height: 100}, d.VideoRegion())
}
