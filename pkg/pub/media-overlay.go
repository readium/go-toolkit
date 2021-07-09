package pub

import "path"

// MediaOverlayNode represents a single node of a Media Overlay
type MediaOverlayNode struct {
	Text     string             `json:"text,omitempty"`
	Audio    string             `json:"audio,omitempty"`
	Role     []string           `json:"role,omitempty"`
	Children []MediaOverlayNode `json:"children,omitempty"`
}

// AddHrefAbsolute modify Href field with a calculated path based on a
// referend file
func (mo *MediaOverlayNode) AddHrefAbsolute(href string, baseFile string) {
	mo.Text = path.Join(path.Dir(baseFile), href)
}

// AddaudioAbsolute modify Audio field with a calculated path based on a
// referend file
func (mo *MediaOverlayNode) AddAudioAbsolute(href string, baseFile string) {
	mo.Audio = path.Join(path.Dir(baseFile), href)
}
