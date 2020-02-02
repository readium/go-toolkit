package models

import (
	"errors"
	"path"
	"strings"

	"github.com/readium/r2-streamer-go/parser/epub"
)

// Publication Main structure for a publication
type Publication struct {
	Context      []string `json:"@context,omitempty"`
	Metadata     Metadata `json:"metadata"`
	Links        []Link   `json:"links"`
	ReadingOrder []Link   `json:"readingOrder,omitempty"`
	Resources    []Link   `json:"resources,omitempty"` //Replaces the manifest but less redundant
	TOC          []Link   `json:"toc,omitempty"`
	PageList     []Link   `json:"page-list,omitempty"`
	Landmarks    []Link   `json:"landmarks,omitempty"`
	LOI          []Link   `json:"loi,omitempty"` //List of illustrations
	LOA          []Link   `json:"loa,omitempty"` //List of audio files
	LOV          []Link   `json:"lov,omitempty"` //List of videos
	LOT          []Link   `json:"lot,omitempty"` //List of tables

	OtherLinks       []Link                  `json:"-"` //Extension point for links that shouldn't show up in the manifest
	OtherCollections []PublicationCollection `json:"-"` //Extension point for collections that shouldn't show up in the manifest
	Internal         []Internal              `json:"-"`
	LCP              epub.LCP                `json:"-"`
}

// Internal TODO
type Internal struct {
	Name  string
	Value interface{}
}

// Link object used in collections and links
type Link struct {
	Href          string             `json:"href"`
	TypeLink      string             `json:"type,omitempty"`
	Rel           []string           `json:"rel,omitempty"`
	Height        int                `json:"height,omitempty"`
	Width         int                `json:"width,omitempty"`
	Title         string             `json:"title,omitempty"`
	Properties    *Properties        `json:"properties,omitempty"`
	Duration      string             `json:"duration,omitempty"`
	Templated     bool               `json:"templated,omitempty"`
	Children      []Link             `json:"children,omitempty"`
	Bitrate       int                `json:"bitrate,omitempty"`
	MediaOverlays []MediaOverlayNode `json:"-"`
}

// PublicationCollection is used as an extension points for other collections in a Publication
type PublicationCollection struct {
	Role     string
	Metadata []Meta
	Links    []Link
	Children []PublicationCollection
}

// LCPHandler struct to generate json to return to the navigator for the lcp information
type LCPHandler struct {
	Identifier string `json:"identifier,omitempty"`
	Profile    string `json:"profile,omitempty"`
	Key        struct {
		Ready bool   `json:"ready,omitempty"`
		Check string `json:"check,omitempty"`
	} `json:"key,omitempty"`
	Hint struct {
		Text string `json:"text,omitempty"`
		URL  string `json:"url,omitempty"`
	} `json:"hint,omitempty"`
	Support struct {
		Mail string `json:"mail,omitempty"`
		URL  string `json:"url,omitempty"`
		Tel  string `json:"tel,omitempty"`
	} `json:"support"`
}

// LCPHandlerPost struct to unmarshal hash send for decrypting lcp
type LCPHandlerPost struct {
	Key struct {
		Hash string `json:"hash"`
	} `json:"key"`
}

// GetCover return the link for the cover
func (publication *Publication) GetCover() (Link, error) {
	return publication.searchLinkByRel("cover")
}

// GetNavDoc return the link for the navigation document
func (publication *Publication) GetNavDoc() (Link, error) {
	return publication.searchLinkByRel("contents")
}

func (publication *Publication) searchLinkByRel(rel string) (Link, error) {
	for _, resource := range publication.Resources {
		for _, resRel := range resource.Rel {
			if resRel == rel {
				return resource, nil
			}
		}
	}

	for _, item := range publication.ReadingOrder {
		for _, spineRel := range item.Rel {
			if spineRel == rel {
				return item, nil
			}
		}
	}

	for _, link := range publication.Links {
		for _, linkRel := range link.Rel {
			if linkRel == rel {
				return link, nil
			}
		}
	}

	return Link{}, errors.New("Can't find " + rel + " in publication")
}

// AddLink Add link in publication link self or search
func (publication *Publication) AddLink(typeLink string, rel []string, url string, templated bool) {
	link := Link{
		Href:     url,
		TypeLink: typeLink,
	}
	if len(rel) > 0 {
		link.Rel = rel
	}

	if templated == true {
		link.Templated = true
	}

	publication.Links = append(publication.Links, link)
}

// FindAllMediaOverlay return all media overlay structure from struct
func (publication *Publication) FindAllMediaOverlay() []MediaOverlayNode {
	var overlay []MediaOverlayNode

	for _, l := range publication.ReadingOrder {
		if len(l.MediaOverlays) > 0 {
			for _, ov := range l.MediaOverlays {
				overlay = append(overlay, ov)
			}
		}
	}

	return overlay
}

// FindMediaOverlayByHref search in media overlay structure for url that match
func (publication *Publication) FindMediaOverlayByHref(href string) []MediaOverlayNode {
	var overlay []MediaOverlayNode

	for _, l := range publication.ReadingOrder {
		if strings.Contains(l.Href, href) {
			if len(l.MediaOverlays) > 0 {
				for _, ov := range l.MediaOverlays {
					overlay = append(overlay, ov)
				}
			}
		}
	}

	return overlay
}

// AddLCPPassphrase function to add internal metadata for decrypting LCP resources
func (publication *Publication) AddLCPPassphrase(passphrase string) {
	publication.Internal = append(publication.Internal, Internal{Name: "lcp_passphrase", Value: passphrase})
}

// AddLCPHash function to add internal metadata for decrypting LCP resources
func (publication *Publication) AddLCPHash(token []byte) {
	publication.AddToInternal("lcp_hash_passphrase", token)
}

func (publication *Publication) findFromInternal(key string) Internal {
	for _, data := range publication.Internal {
		if data.Name == key {
			return data
		}
	}
	return Internal{}
}

// GetStringFromInternal get data store in internal struct in string
func (publication *Publication) GetStringFromInternal(key string) string {

	data := publication.findFromInternal(key)
	if data.Name != "" {
		return data.Value.(string)
	}
	return ""
}

// GetBytesFromInternal get data store in internal structure in byte
func (publication *Publication) GetBytesFromInternal(key string) []byte {

	data := publication.findFromInternal(key)
	if data.Name != "" {
		return data.Value.([]byte)
	}
	return []byte("")
}

// AddToInternal push data to internal struct in publication
func (publication *Publication) AddToInternal(key string, value interface{}) {
	publication.Internal = append(publication.Internal, Internal{Name: key, Value: value})
}

// GetLCPJSON return the raw lcp license json from META-INF/license.lcpl
// if the data is present else return emtpy string
func (publication *Publication) GetLCPJSON() []byte {
	data := publication.GetBytesFromInternal("lcpl")

	return data
}

// GetLCPHandlerInfo return the lcp handler struct for marshalling
func (publication *Publication) GetLCPHandlerInfo() (LCPHandler, error) {
	var info LCPHandler

	if publication.LCP.ID != "" {
		info.Identifier = publication.LCP.ID
		info.Hint.Text = publication.LCP.Encryption.UserKey.TextHint
		info.Key.Check = publication.LCP.Encryption.UserKey.KeyCheck
		info.Key.Ready = false
		info.Profile = publication.LCP.Encryption.Profile
		for _, l := range publication.LCP.Links {
			if l.Rel == "hint" {
				info.Hint.URL = l.Href
			}
		}

		return info, nil
	}

	return info, errors.New("no LCP information")
}

// GetPreFetchResources select resources that match media type we want to
// prefetch with the manifest
func (publication *Publication) GetPreFetchResources() []Link {
	var resources []Link

	mediaTypes := []string{"text/css", "application/vnd.ms-opentype", "text/javascript"}

	for _, l := range publication.Resources {
		for _, m := range mediaTypes {
			if l.TypeLink == m {
				resources = append(resources, l)
			}
		}
	}

	return resources
}

// AddRel add rel information to Link, will check if the
func (link *Link) AddRel(rel string) {
	relAlreadyPresent := false

	for _, r := range link.Rel {
		if r == rel {
			relAlreadyPresent = true
		}
	}

	if relAlreadyPresent == false {
		link.Rel = append(link.Rel, rel)
	}
}

// AddHrefAbsolute modify Href field with a calculated path based on a
// referend file
func (link *Link) AddHrefAbsolute(href string, baseFile string) {
	link.Href = path.Join(path.Dir(baseFile), href)
}

//TransformLinkToFullURL concatenate a base url to all links
func (publication *Publication) TransformLinkToFullURL(baseURL string) {

	for i := range publication.ReadingOrder {
		if !(strings.Contains(publication.ReadingOrder[i].Href, "http://") || strings.Contains(publication.ReadingOrder[i].Href, "https://")) {
			publication.ReadingOrder[i].Href = baseURL + publication.ReadingOrder[i].Href
		}
	}

	for i := range publication.Resources {
		if !(strings.Contains(publication.Resources[i].Href, "http://") || strings.Contains(publication.Resources[i].Href, "https://")) {
			publication.Resources[i].Href = baseURL + publication.Resources[i].Href
		}
	}

	for i := range publication.TOC {
		if !(strings.Contains(publication.TOC[i].Href, "http://") || strings.Contains(publication.TOC[i].Href, "https://")) {
			publication.TOC[i].Href = baseURL + publication.TOC[i].Href
		}
	}

	for i := range publication.Landmarks {
		if !(strings.Contains(publication.Landmarks[i].Href, "http://") || strings.Contains(publication.Landmarks[i].Href, "https://")) {
			publication.Landmarks[i].Href = baseURL + publication.Landmarks[i].Href
		}
	}
}
