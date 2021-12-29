package manifest

import (
	"encoding/json"
	"path"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/util"
)

// Manifest Main structure for a publication
type Manifest struct {
	Context         []string `json:"@context,omitempty"`
	Metadata        Metadata `json:"metadata"`
	Links           LinkList `json:"links"`
	ReadingOrder    LinkList `json:"readingOrder,omitempty"`
	Resources       LinkList `json:"resources,omitempty"` //Replaces the manifest but less redundant
	TableOfContents LinkList `json:"toc,omitempty"`

	Subcollections map[string][]PublicationCollection `json:"-"` //Extension point for collections that shouldn't show up in the manifest
	// Internal       []Internal                         `json:"-"` // TODO remove
}

// Finds the first [Link] with the given relation in the manifest's links.
func (m Manifest) LinkWithRel(rel string) *Link {
	for _, resource := range m.Resources {
		for _, resRel := range resource.Rels {
			if resRel == rel {
				return &resource
			}
		}
	}

	for _, item := range m.ReadingOrder {
		for _, spineRel := range item.Rels {
			if spineRel == rel {
				return &item
			}
		}
	}

	for _, link := range m.Links {
		for _, linkRel := range link.Rels {
			if linkRel == rel {
				return &link
			}
		}
	}

	return nil
}

// TODO linksWithRel: Finds all [Link]s having the given [rel] in the manifest's links.

// Parses a [Manifest] from its RWPM JSON representation.
//
// TODO log [warnings] ?
// https://readium.org/webpub-manifest/
// https://readium.org/webpub-manifest/schema/publication.schema.json
func ManifestFromJSON(rawJson map[string]interface{}, packaged bool) (*Manifest, error) {
	if rawJson == nil {
		return nil, nil
	}

	// Parse links
	rawLinks, ok := rawJson["links"].([]interface{})
	var links []Link
	var err error
	if ok {
		links, err = LinksFromJSONArray(rawLinks, LinkHrefNormalizerIdentity)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'links'")
		}
	}

	baseURL := ""
	if !packaged {
		self := firstLinkWithRel(links, "self")
		if self != nil {
			url := extensions.ToUrlOrNull(self.Href)
			if url != nil {
				url.Path = path.Dir(url.Path)
				baseURL = url.String()
			}
		}
	}

	normalizeHref := func(href string) (string, error) {
		return util.NewHREF(href, baseURL).String()
	}

	manifest := new(Manifest)

	// Context
	rawContext, ok := rawJson["@context"].([]interface{})
	if ok {
		context, err := parseSliceOrString(rawContext, false)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling '@context'")
		}
		manifest.Context = context
	}

	// Metadata
	rmt, ok := rawJson["metadata"].(map[string]interface{})
	if !ok {
		errors.New("'metadata' JSON object is required")
	}
	if rmt == nil {
		return nil, errors.New("'metadata' is required")
	}
	metadata, err := MetadataFromJSON(rmt, normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'metadata'")
	}
	manifest.Metadata = *metadata

	// Links
	links, err = LinksFromJSONArray(rawLinks, normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'links'")
	}
	for _, link := range links {
		if packaged && extensions.Contains(link.Rels, "self") {
			newRels := make([]string, 0, len(link.Rels)) // Same total length as original
			newRels = append(newRels, "alternate")
			for _, rel := range link.Rels {
				if rel == "self" {
					continue
				}
				newRels = append(newRels, rel)
			}
			link.Rels = newRels
		}
	}
	manifest.Links = links

	// ReadingOrder
	readingOrderRaw, ok := rawJson["readingOrder"].([]interface{})
	if !ok {
		// [readingOrder] used to be [spine], so we parse [spine] as a fallback.
		readingOrderRaw, ok = rawJson["spine"].([]interface{})
		if !ok {
			return nil, errors.New("Manifest has no valid 'readingOrder' or 'spine'")
		}
	}
	readingOrder, err := LinksFromJSONArray(readingOrderRaw, normalizeHref)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling reading order")
	}
	manifest.ReadingOrder = make(LinkList, 0, len(readingOrder)) // More links with than without mimetypes
	for _, link := range readingOrder {
		if link.Type == "" {
			continue
		}
		manifest.ReadingOrder = append(manifest.ReadingOrder, link)
	}

	// Resources
	resourcesRaw, ok := rawJson["resources"].([]interface{})
	if ok {
		resources, err := LinksFromJSONArray(resourcesRaw, normalizeHref)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'resources'")
		}
		manifest.Resources = make(LinkList, 0, len(resources)) // More resources with than without mimetypes
		for _, link := range resources {
			if link.Type == "" {
				continue
			}
			manifest.Resources = append(manifest.Resources, link)
		}
	}

	// TOC
	tocRaw, ok := rawJson["toc"].([]interface{})
	if ok {
		toc, err := LinksFromJSONArray(tocRaw, normalizeHref)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'toc'")
		}
		manifest.TableOfContents = toc
	}

	// Parses subcollections from the remaining JSON properties.
	// TODO!

	return manifest, nil
}

func (m *Manifest) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fm, err := ManifestFromJSON(object, false)
	if err != nil {
		return err
	}
	*m = *fm
	return nil
}

/*func (m Manifest) MarshalJSON() ([]byte, error) {

}*/

/*

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

*/
