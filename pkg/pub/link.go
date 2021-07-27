package pub

// Link object used in collections and links
type Link struct {
	Href       string      `json:"href"`
	Type       string      `json:"type,omitempty"`
	Rels       []string    `json:"rel,omitempty"`
	Height     int         `json:"height,omitempty"`
	Width      int         `json:"width,omitempty"`
	Title      string      `json:"title,omitempty"`
	Properties *Properties `json:"properties,omitempty"`
	Duration   string      `json:"duration,omitempty"`
	Templated  bool        `json:"templated,omitempty"`
	Children   []Link      `json:"children,omitempty"`
	Bitrate    int         `json:"bitrate,omitempty"`
}
