package models

// MediaOverlayNode represents a single node of a Media Overlay
type MediaOverlayNode struct {
	Text       string                  `json:"text,omitempty"`
	Audio      string                  `json:"audio,omitempty"`
	Role       []string                `json:"role,omitempty"`
	Children   []MediaOverlayNode      `json:"children,omitempty"`
}