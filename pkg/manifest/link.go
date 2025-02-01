package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Link
// https://github.com/readium/webpub-manifest/blob/master/README.md#24-the-link-object
// https://github.com/readium/webpub-manifest/blob/master/schema/link.schema.json
type Link struct {
	Href       HREF                 `json:"href"`                 // URI or URI template of the linked resource.
	MediaType  *mediatype.MediaType `json:"type,omitempty"`       // MIME type of the linked resource.
	Title      string               `json:"title,omitempty"`      // Title of the linked resource.
	Rels       Strings              `json:"rel,omitempty"`        // Relation between the linked resource and its containing collection.
	Properties Properties           `json:"properties,omitempty"` // Properties associated to the linked resource.
	Height     uint                 `json:"height,omitempty"`     // Height of the linked resource in pixels.
	Width      uint                 `json:"width,omitempty"`      // Width of the linked resource in pixels.
	Bitrate    float64              `json:"bitrate,omitempty"`    // Bitrate of the linked resource in kbps.
	Duration   float64              `json:"duration,omitempty"`   // Length of the linked resource in seconds.
	Languages  Strings              `json:"language,omitempty"`   // Expected language of the linked resource (BCP 47 tag).
	Alternates LinkList             `json:"alternate,omitempty"`  // Alternate resources for the linked resource.
	Children   LinkList             `json:"children,omitempty"`   // Resources that are children of the linked resource, in the context of a given collection role.
}

// Returns the URL represented by this link's HREF, resolved to the given [base] URL.
// If the HREF is a template, the [parameters] are used to expand it according to RFC 6570.
func (l Link) URL(base url.URL, parameters map[string]string) url.URL {
	return l.Href.Resolve(base, parameters)
}

// Creates an [Link] from its RWPM JSON representation.
func LinkFromJSON(rawJson map[string]interface{}) (*Link, error) {
	if rawJson == nil {
		return nil, nil
	}

	rawHref, ok := rawJson["href"].(string)
	if !ok {
		// Warning: [href] is required
		return nil, errors.New("'href' is required in link")
	}

	templated := parseOptBool(rawJson["templated"])
	var href HREF
	var err error
	if templated {
		href, err = NewHREFFromString(rawHref, templated)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'href' as URL template")
		}
	} else {
		u, err := url.URLFromString(rawHref)
		if err != nil {
			u, err = url.URLFromDecodedPath(rawHref)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshalling 'href' as URL")
			}
		}
		href = NewHREF(u)
	}

	link := &Link{
		Href:     href,
		Title:    parseOptString(rawJson["title"]),
		Height:   float64ToUint(parseOptFloat64(rawJson["height"])),
		Width:    float64ToUint(parseOptFloat64(rawJson["width"])),
		Bitrate:  float64Positive(parseOptFloat64(rawJson["bitrate"])),
		Duration: float64Positive(parseOptFloat64(rawJson["duration"])),
	}

	// Media Type
	rawType := parseOptString(rawJson["type"])
	if rawType != "" {
		mediaType, err := mediatype.NewOfString(rawType)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'type' as valid mimetype")
		}
		link.MediaType = &mediaType
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
		alternates, err := LinksFromJSONArray(rawAlternates)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'alternate'")
		}
		link.Alternates = alternates
	}

	// Children
	rawChildren, ok := rawJson["children"].([]interface{})
	if ok {
		children, err := LinksFromJSONArray(rawChildren)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling 'children'")
		}
		link.Children = children
	}

	return link, nil
}

func LinksFromJSONArray(rawJsonArray []interface{}) ([]Link, error) {
	links := make([]Link, 0, len(rawJsonArray))
	for i, entry := range rawJsonArray {
		entry, ok := entry.(map[string]interface{})
		if !ok {
			// TODO: Should this be a "warning", an error, or completely ignored?
			continue
		}
		rl, err := LinkFromJSON(entry)
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
	fl, err := LinkFromJSON(object)
	if err != nil {
		return err
	}
	*l = *fl
	return nil
}

func (l Link) MarshalJSON() ([]byte, error) {
	res := make(map[string]interface{})
	res["href"] = l.Href.String()
	if l.MediaType != nil {
		res["type"] = l.MediaType.String()
	}
	if l.Href.IsTemplated() {
		res["templated"] = true
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
func (ll LinkList) IndexOfFirstWithHref(href url.URL) int {
	for i, link := range ll {
		if link.URL(nil, nil).Equivalent(href) {
			return i
		}
	}
	return -1
}

// Finds the first link matching the given HREF.
func (ll LinkList) FirstWithHref(href url.URL) *Link {
	for _, link := range ll {
		if link.URL(nil, nil).Equivalent(href) {
			return &link
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
		if link.MediaType.Matches(mt) {
			return &link
		}
	}
	return nil
}

// Finds all the links matching any of the given media types.
func (ll LinkList) FilterByMediaType(mt ...*mediatype.MediaType) LinkList {
	flinks := make([]Link, 0)
	for _, link := range ll {
		if link.MediaType.Matches(mt...) {
			flinks = append(flinks, link)
		}
	}
	return flinks
}

// Returns whether all the resources in the collection are bitmaps.
func (ll LinkList) AllAreBitmap() bool {
	for _, link := range ll {
		if !link.MediaType.IsBitmap() {
			return false
		}
	}
	return true
}

// Returns whether all the resources in the collection are audio clips.
func (ll LinkList) AllAreAudio() bool {
	for _, link := range ll {
		if !link.MediaType.IsAudio() {
			return false
		}
	}
	return true
}

// Returns whether all the resources in the collection are video clips.
func (ll LinkList) AllAreVideo() bool {
	for _, link := range ll {
		if !link.MediaType.IsVideo() {
			return false
		}
	}
	return true
}

// Returns whether all the resources in the collection are bitmaps or video clips.
func (ll LinkList) AllAreVisual() bool {
	for _, link := range ll {
		if !link.MediaType.IsBitmap() && !link.MediaType.IsVideo() {
			return false
		}
	}
	return true
}

// Returns whether all the resources in the collection are HTML documents.
func (ll LinkList) AllAreHTML() bool {
	for _, link := range ll {
		if !link.MediaType.IsHTML() {
			return false
		}
	}
	return true
}

// Returns whether all the resources in the collection are matching the given media type.
func (ll LinkList) AllMatchMediaType(mt ...*mediatype.MediaType) bool {
	for _, link := range ll {
		if !link.MediaType.Matches(mt...) {
			return false
		}
	}
	return true
}

// Returns a list of `Link` after flattening the `children` and `alternates` links of the receiver.
func (ll LinkList) Flatten() LinkList {
	links := make(LinkList, 0, len(ll))
	for _, link := range ll {
		links = append(links, link)
		links = append(links, link.Alternates.Flatten()...)
		links = append(links, link.Children.Flatten()...)
	}
	return links
}
