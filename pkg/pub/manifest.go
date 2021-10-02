package pub

import (
	"errors"
	"strings"
)

// Manifest Main structure for a publication
type Manifest struct {
	Context         []string `json:"@context,omitempty"`
	Metadata        Metadata `json:"metadata"`
	Links           []Link   `json:"links"`
	ReadingOrder    []Link   `json:"readingOrder,omitempty"`
	Resources       []Link   `json:"resources,omitempty"` //Replaces the manifest but less redundant
	TableOfContents []Link   `json:"toc,omitempty"`

	Subcollections map[string][]PublicationCollection `json:"-"` //Extension point for collections that shouldn't show up in the manifest
	Internal       []Internal                         `json:"-"` // TODO remove
}

// Internal TODO
type Internal struct {
	Name  string
	Value interface{}
}

// GetCover return the link for the cover
func (publication *Manifest) GetCover() (Link, error) {
	return publication.searchLinkByRel("cover")
}

// GetNavDoc return the link for the navigation document
func (publication *Manifest) GetNavDoc() (Link, error) {
	return publication.searchLinkByRel("contents")
}

func (publication *Manifest) searchLinkByRel(rel string) (Link, error) {
	for _, resource := range publication.Resources {
		for _, resRel := range resource.Rels {
			if resRel == rel {
				return resource, nil
			}
		}
	}

	for _, item := range publication.ReadingOrder {
		for _, spineRel := range item.Rels {
			if spineRel == rel {
				return item, nil
			}
		}
	}

	for _, link := range publication.Links {
		for _, linkRel := range link.Rels {
			if linkRel == rel {
				return link, nil
			}
		}
	}

	return Link{}, errors.New("Can't find " + rel + " in publication")
}

// AddLink Add link in publication link self or search
func (publication *Manifest) AddLink(typeLink string, rel []string, url string, templated bool) {
	link := Link{
		Href: url,
		Type: typeLink,
	}
	if len(rel) > 0 {
		link.Rels = rel
	}

	if templated {
		link.Templated = true
	}

	publication.Links = append(publication.Links, link)
}

// AddLCPPassphrase function to add internal metadata for decrypting LCP resources
func (publication *Manifest) AddLCPPassphrase(passphrase string) {
	publication.Internal = append(publication.Internal, Internal{Name: "lcp_passphrase", Value: passphrase})
}

// AddLCPHash function to add internal metadata for decrypting LCP resources
func (publication *Manifest) AddLCPHash(token []byte) {
	publication.AddToInternal("lcp_hash_passphrase", token)
}

func (publication *Manifest) findFromInternal(key string) Internal {
	for _, data := range publication.Internal {
		if data.Name == key {
			return data
		}
	}
	return Internal{}
}

// GetStringFromInternal get data store in internal struct in string
func (publication *Manifest) GetStringFromInternal(key string) string {

	data := publication.findFromInternal(key)
	if data.Name != "" {
		return data.Value.(string)
	}
	return ""
}

// GetBytesFromInternal get data store in internal structure in byte
func (publication *Manifest) GetBytesFromInternal(key string) []byte {

	data := publication.findFromInternal(key)
	if data.Name != "" {
		return data.Value.([]byte)
	}
	return []byte("")
}

// AddToInternal push data to internal struct in publication
func (publication *Manifest) AddToInternal(key string, value interface{}) {
	publication.Internal = append(publication.Internal, Internal{Name: key, Value: value})
}

// GetLCPJSON return the raw lcp license json from META-INF/license.lcpl
// if the data is present else return emtpy string
func (publication *Manifest) GetLCPJSON() []byte {
	data := publication.GetBytesFromInternal("lcpl")

	return data
}

// GetPreFetchResources select resources that match media type we want to
// prefetch with the manifest
func (publication *Manifest) GetPreFetchResources() []Link {
	var resources []Link

	mediaTypes := []string{"text/css", "application/vnd.ms-opentype", "text/javascript"}

	for _, l := range publication.Resources {
		for _, m := range mediaTypes {
			if l.Type == m {
				resources = append(resources, l)
			}
		}
	}

	return resources
}

//TransformLinkToFullURL concatenate a base url to all links
func (publication *Manifest) TransformLinkToFullURL(baseURL string) {

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

	for i := range publication.TableOfContents {
		if !(strings.Contains(publication.TableOfContents[i].Href, "http://") || strings.Contains(publication.TableOfContents[i].Href, "https://")) {
			publication.TableOfContents[i].Href = baseURL + publication.TableOfContents[i].Href
		}
	}
}
