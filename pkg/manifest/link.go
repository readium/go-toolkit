package manifest

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util"
)

// Function used to recursively transform the href of a [Link] when parsing its JSON representation.
type LinkHrefNormalizer func(href string) (string, error)

// Default href normalizer for [Link], doing nothing.
func LinkHrefNormalizerIdentity(href string) (string, error) {
	return href, nil
}

// Link
// https://github.com/readium/webpub-manifest/blob/master/README.md#24-the-link-object
// https://github.com/readium/webpub-manifest/blob/master/schema/link.schema.json
type Link struct {
	Href       string     `json:"href"`                 // URI or URI template of the linked resource.
	Type       string     `json:"type,omitempty"`       // MIME type of the linked resource.
	Templated  bool       `json:"templated,omitempty"`  // Indicates that a URI template is used in href.
	Title      string     `json:"title,omitempty"`      // Title of the linked resource.
	Rels       Strings    `json:"rel,omitempty"`        // Relation between the linked resource and its containing collection.
	Properties Properties `json:"properties,omitempty"` // Properties associated to the linked resource.
	Height     uint       `json:"height,omitempty"`     // Height of the linked resource in pixels.
	Width      uint       `json:"width,omitempty"`      // Width of the linked resource in pixels.
	Bitrate    float64    `json:"bitrate,omitempty"`    // Bitrate of the linked resource in kbps.
	Duration   float64    `json:"duration,omitempty"`   // Length of the linked resource in seconds.
	Languages  Strings    `json:"language,omitempty"`   // Expected language of the linked resource (BCP 47 tag).
	Alternates LinkList   `json:"alternate,omitempty"`  // Alternate resources for the linked resource.
	Children   LinkList   `json:"children,omitempty"`   // Resources that are children of the linked resource, in the context of a given collection role.
}

func (l Link) MediaType() mediatype.MediaType {
	mt := mediatype.OfString(l.Type)
	if mt == nil {
		return mediatype.Binary
	}
	return *mt
}

// List of URI template parameter keys, if the [Link] is templated.
func (l Link) TemplateParameters() []string {
	if !l.Templated {
		return nil
	}
	return util.NewURITemplate(l.Href).Parameters()
}

// Expands the HREF by replacing URI template variables by the given parameters.
func (l Link) ExpandTemplate(parameters map[string]string) Link {
	l.Href = util.NewURITemplate(l.Href).Expand(parameters)
	l.Templated = false
	return l
}

// Computes an absolute URL to the link, relative to the given [baseUrl].
// If the link's [href] is already absolute, the [baseUrl] is ignored.
func (l Link) ToURL(baseURL string) string {
	href := strings.TrimPrefix(l.Href, "/")
	if href == "" {
		return ""
	}
	if baseURL == "" {
		baseURL = "/"
	}
	h, _ := util.NewHREF(href, baseURL).PercentEncodedString()
	return h
}

// Creates an [Link] from its RWPM JSON representation.
func LinkFromJSON(rawJson map[string]interface{}, normalizeHref LinkHrefNormalizer) (*Link, error) {
	if rawJson == nil {
		return nil, nil
	}

	href, ok := rawJson["href"].(string)
	if !ok {
		// Warning: [href] is required
		return nil, errors.New("'href' is required in link")
	}

	if normalizeHref == nil {
		normalizeHref = LinkHrefNormalizerIdentity
	}
	href, err := normalizeHref(href)
	if err != nil {
		return nil, err
	}

	link := &Link{
		Href:      href,
		Type:      parseOptString(rawJson["type"]),
		Templated: parseOptBool(rawJson["templated"]),
		Title:     parseOptString(rawJson["title"]),
		Height:    float64ToUint(parseOptFloat64(rawJson["height"])),
		Width:     float64ToUint(parseOptFloat64(rawJson["width"])),
		Bitrate:   float64Positive(parseOptFloat64(rawJson["bitrate"])),
		Duration:  float64Positive(parseOptFloat64(rawJson["duration"])),
	}

	// Properties
	properties, ok := rawJson["properties"].(map[string]interface{})
	if ok {
		link.Properties = properties
	}

	// Rels
	rels, err := parseSliceOrString(rawJson["rel"], true)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'rel'")
	}
	link.Rels = rels

	// Languages
	languages, err := parseSliceOrString(rawJson["language"], false)
	if err != nil {
		return nil, errors.Wrap(err, "failed unmarshalling 'language'")
	}
	link.Languages = languages

	// Alternates
	rawAlternates, ok := rawJson["alternate"].([]interface{})
	if ok {
		alternates, err := LinksFromJSONArray(rawAlternates, normalizeHref)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'alternate'")
		}
		link.Alternates = alternates
	}

	// Children
	rawChildren, ok := rawJson["children"].([]interface{})
	if ok {
		children, err := LinksFromJSONArray(rawChildren, normalizeHref)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'children'")
		}
		link.Children = children
	}

	return link, nil
}

func LinksFromJSONArray(rawJsonArray []interface{}, normalizeHref LinkHrefNormalizer) ([]Link, error) {
	links := make([]Link, 0, len(rawJsonArray))
	for i, entry := range rawJsonArray {
		entry, ok := entry.(map[string]interface{})
		if !ok {
			// TODO: Should this be a "warning", an error, or completely ignored?
			continue
		}
		rl, err := LinkFromJSON(entry, normalizeHref)
		if err != nil {
			return nil, errors.Wrapf(err, "failed unmarshalling Link at position %d", i)
		}
		if rl == nil {
			continue
		}
		links = append(links, *rl)
	}
	return links, nil
}

func (l *Link) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fl, err := LinkFromJSON(object, LinkHrefNormalizerIdentity)
	if err != nil {
		return err
	}
	*l = *fl
	return nil
}

func (l Link) MarshalJSON() ([]byte, error) {
	res := make(map[string]interface{})
	res["href"] = l.Href
	if l.Type != "" {
		res["type"] = l.Type
	}
	if l.Templated {
		res["templated"] = l.Templated
	}
	if l.Title != "" {
		res["title"] = l.Title
	}
	if len(l.Rels) > 0 {
		res["rel"] = l.Rels
	}
	if len(l.Properties) > 0 {
		res["properties"] = l.Properties
	}
	if l.Height > 0 {
		res["height"] = l.Height
	}
	if l.Width > 0 {
		res["width"] = l.Width
	}
	if l.Bitrate > 0 {
		res["bitrate"] = l.Bitrate
	}
	if l.Duration > 0 {
		res["duration"] = l.Duration
	}
	if len(l.Languages) > 0 {
		res["language"] = l.Languages
	}
	if len(l.Alternates) > 0 {
		res["alternate"] = l.Alternates
	}
	if len(l.Children) > 0 {
		res["children"] = l.Children
	}
	return json.Marshal(res)
}

// Slice of links
type LinkList []Link

// Returns the first [Link] with the given [href], or null if not found.
func (ll LinkList) IndexOfFirstWithHref(href string) int {
	for i, link := range ll {
		if link.Href == href {
			return i
		}
		for _, alt := range link.Alternates {
			if alt.Href == href {
				return i
			}
		}
	}
	return -1
}

// Finds the first link matching the given HREF.
func (ll LinkList) FirstWithHref(href string) *Link {
	for _, link := range ll {
		if link.Href == href {
			return &link
		}
		for _, alt := range link.Alternates {
			if alt.Href == href {
				return &alt
			}
		}
	}
	return nil
}

// Finds the first link with the given relation.
func (ll LinkList) FirstWithRel(rel string) *Link {
	for _, link := range ll {
		for _, r := range link.Rels {
			if r == rel {
				return &link
			}
		}
	}
	return nil
}

// Finds all the links with the given relation.
func (ll LinkList) FilterByRel(rel string) LinkList {
	flinks := make([]Link, 0)
	for _, link := range ll {
		// TODO should we check alternates?
		for _, r := range link.Rels {
			if r == rel {
				flinks = append(flinks, link)
			}
		}
	}
	return flinks
}

// Finds the first link matching the given media type.
func (ll LinkList) FirstWithMediaType(mt *mediatype.MediaType) *Link {
	for _, link := range ll {
		if link.MediaType().Matches(mt) {
			return &link
		}
		// TODO should we check alternates?
	}
	return nil
}

// Finds all the links matching any of the given media types.
func (ll LinkList) FilterByMediaType(mt ...*mediatype.MediaType) LinkList {
	flinks := make([]Link, 0)
	for _, link := range ll {
		if link.MediaType().Matches(mt...) {
			flinks = append(flinks, link)
		}
		// TODO should we check alternates?
	}
	return flinks
}

// Returns whether all the resources in the collection are bitmaps.
func (ll LinkList) AllAreBitmap() bool {
	for _, link := range ll {
		if !link.MediaType().IsBitmap() {
			return false
		}
		// TODO should we check alternates?
	}
	return true
}

// Returns whether all the resources in the collection are audio clips.
func (ll LinkList) AllAreAudio() bool {
	for _, link := range ll {
		if !link.MediaType().IsAudio() {
			return false
		}
		// TODO should we check alternates?
	}
	return true
}

// Returns whether all the resources in the collection are video clips.
func (ll LinkList) AllAreVideo() bool {
	for _, link := range ll {
		if !link.MediaType().IsVideo() {
			return false
		}
		// TODO should we check alternates?
	}
	return true
}

// Returns whether all the resources in the collection are bitmaps or video clips.
func (ll LinkList) AllAreVisual() bool {
	for _, link := range ll {
		mt := link.MediaType()
		if !mt.IsBitmap() && !mt.IsVideo() {
			return false
		}
		// TODO should we check alternates?
	}
	return true
}

// Returns whether all the resources in the collection are HTML documents.
func (ll LinkList) AllAreHTML() bool {
	for _, link := range ll {
		if !link.MediaType().IsHTML() {
			return false
		}
		// TODO should we check alternates?
	}
	return true
}

// Returns whether all the resources in the collection are matching the given media type.
func (ll LinkList) AllMatchMediaType(mt ...*mediatype.MediaType) bool {
	for _, link := range ll {
		if !link.MediaType().Matches(mt...) {
			return false
		}
		// TODO should we check alternates?
	}
	return true
}
