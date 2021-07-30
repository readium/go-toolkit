package pub

import "path"

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

// AddRel add rel information to Link, will check if the
func (link *Link) AddRel(rel string) {
	relAlreadyPresent := false

	for _, r := range link.Rels {
		if r == rel {
			relAlreadyPresent = true
		}
	}

	if !relAlreadyPresent {
		link.Rels = append(link.Rels, rel)
	}
}

// AddHrefAbsolute modify Href field with a calculated path based on a
// referend file
func (link *Link) AddHrefAbsolute(href string, baseFile string) {
	link.Href = path.Join(path.Dir(baseFile), href)
}
