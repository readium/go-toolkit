package manifest

import (
	"path"

	"github.com/readium/go-toolkit/pkg/mediatype"
)

// Link
// https://github.com/readium/webpub-manifest/blob/master/README.md#24-the-link-object
// https://github.com/readium/webpub-manifest/blob/master/schema/link.schema.json
type Link struct {
	Href       string     `json:"href"`                 // URI or URI template of the linked resource.
	Type       string     `json:"type,omitempty"`       // MIME type of the linked resource.
	Templated  bool       `json:"templated,omitempty"`  // Indicates that a URI template is used in href.
	Title      string     `json:"title,omitempty"`      // Title of the linked resource.
	Rels       []string   `json:"rel,omitempty"`        // Relation between the linked resource and its containing collection.
	Properties Properties `json:"properties,omitempty"` // Properties associated to the linked resource.
	Height     int        `json:"height,omitempty"`     // Height of the linked resource in pixels.
	Width      int        `json:"width,omitempty"`      // Width of the linked resource in pixels.
	Bitrate    float64    `json:"bitrate,omitempty"`    // Bitrate of the linked resource in kbps.
	Duration   float64    `json:"duration,omitempty"`   // Length of the linked resource in seconds.
	Languages  []string   `json:"language,omitempty"`   // Expected language of the linked resource (BCP 47 tag).
	Alternates []Link     `json:"alternate,omitempty"`  // Alternate resources for the linked resource.
	Children   []Link     `json:"children,omitempty"`   // Resources that are children of the linked resource, in the context of a given collection role.
}

func (l Link) MediaType() mediatype.MediaType {
	mt := mediatype.OfString(l.Type)
	if mt == nil {
		return mediatype.BINARY
	}
	return *mt
}

/// OLD ///

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
