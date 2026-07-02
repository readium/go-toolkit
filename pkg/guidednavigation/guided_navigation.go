package guidednavigation

import (
	"encoding/json"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Readium Guided Navigation Document
// https://readium.org/guided-navigation/schema/document.schema.json
type GuidedNavigationDocument struct {
	Links  manifest.LinkList        `json:"links,omitempty"` // References to other resources that are related to the current Guided Navigation Document.
	Guided []GuidedNavigationObject `json:"guided"`          // A sequence of resources and/or media fragments into these resources, meant to be presented sequentially to the user.
}

// Readium Guided Navigation Object
// https://readium.org/guided-navigation/schema/object.schema.json
type GuidedNavigationObject struct {
	ID          string                      `json:"id,omitempty"`          // Identifier for the object, referenced from the SSML representation of a sibling text (e.g. <readium:image id="..."/>).
	AudioRef    url.URL                     `json:"audioref,omitempty"`    // References an audio resource or a fragment of it.
	ImgRef      url.URL                     `json:"imgref,omitempty"`      // References an image or a fragment of it.
	TextRef     url.URL                     `json:"textref,omitempty"`     // References a textual resource or a fragment of it.
	VideoRef    url.URL                     `json:"videoref,omitempty"`    // References a video resource or a fragment of it.
	Text        GuidedNavigationText        `json:"text,omitempty"`        // Textual equivalent of the resources or fragment of the resources referenced by the current Guided Navigation Object.
	Role        []GuidedNavigationRole      `json:"role,omitempty"`        // Convey the structural semantics of a publication
	Children    []GuidedNavigationObject    `json:"children,omitempty"`    // Items that are children of the containing Guided Navigation Object.
	Description GuidedNavigationDescription `json:"description,omitempty"` // Text, audio or image description for the current Guided Navigation Object.
}

func (o GuidedNavigationObject) Empty() bool {
	return o.TextRef == nil && o.ImgRef == nil && o.AudioRef == nil && o.VideoRef == nil &&
		o.Text.Empty() && len(o.Children) == 0 && o.Description.Empty()
}

// ChildrenOnly reports whether the object carries no information of its own besides its children.
// Such objects are transparent wrappers (e.g. from <div> or <span>) whose children can be
// spliced into the parent's children.
func (o GuidedNavigationObject) ChildrenOnly() bool {
	return o.TextRef == nil && o.ImgRef == nil && o.AudioRef == nil && o.VideoRef == nil &&
		o.Text.Empty() && len(o.Children) > 0 && o.Description.Empty() && len(o.Role) == 0 && o.ID == ""
}

func (o GuidedNavigationObject) TextOnly() bool {
	return o.TextRef == nil && o.ImgRef == nil && o.AudioRef == nil && o.VideoRef == nil &&
		!o.Text.Empty() && len(o.Children) == 0 && o.Description.Empty() && len(o.Role) == 0 && o.ID == ""
}

func (o GuidedNavigationObject) MarshalJSON() ([]byte, error) {
	res := make(map[string]interface{})
	if o.Empty() {
		return json.Marshal(res)
	}

	if o.ID != "" {
		res["id"] = o.ID
	}
	if o.TextRef != nil {
		if s := o.TextRef.String(); s != "" {
			res["textref"] = s
		}
	}
	if o.ImgRef != nil {
		if s := o.ImgRef.String(); s != "" {
			res["imgref"] = s
		}
	}
	if o.AudioRef != nil {
		if s := o.AudioRef.String(); s != "" {
			res["audioref"] = s
		}
	}
	if o.VideoRef != nil {
		if s := o.VideoRef.String(); s != "" {
			res["videoref"] = s
		}
	}

	if !o.Text.Empty() {
		res["text"] = o.Text
	}
	if len(o.Role) > 0 {
		res["role"] = o.Role
	}
	if len(o.Children) > 0 {
		res["children"] = o.Children
	}
	if !o.Description.Empty() {
		res["description"] = o.Description
	}

	return json.Marshal(res)
}

type GuidedNavigationText struct {
	Plain    string `json:"plain,omitempty"`    // Plain text
	SSML     string `json:"ssml,omitempty"`     // SSML markup
	Language string `json:"language,omitempty"` // BCP-47 language tag
}

func (t GuidedNavigationText) Empty() bool {
	return t.Plain == "" && t.SSML == "" && t.Language == ""
}

func (t *GuidedNavigationText) UnmarshalJSON(data []byte) error {
	// Just plain text
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		t.Plain = plain
		return nil
	}

	type alias GuidedNavigationText
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*t = GuidedNavigationText(obj)
	return nil
}

func (t GuidedNavigationText) MarshalJSON() ([]byte, error) {
	res := make(map[string]interface{})

	if t.SSML == "" && t.Language == "" {
		return json.Marshal(t.Plain)
	}

	if t.Plain != "" {
		res["plain"] = t.Plain
	}
	if t.SSML != "" {
		res["ssml"] = t.SSML
	}
	if t.Language != "" {
		res["language"] = t.Language
	}
	return json.Marshal(res)
}

// Readium Guided Navigation Description Object
// https://readium.org/guided-navigation/schema/description.schema.json
// Serialized as a plain string when it only carries plain text.
type GuidedNavigationDescription struct {
	AudioRef url.URL              `json:"audioref,omitempty"` // References an audio resource or a fragment of it.
	ImgRef   url.URL              `json:"imgref,omitempty"`   // References an image or a fragment of it.
	TextRef  url.URL              `json:"textref,omitempty"`  // References a textual resource or a fragment of it.
	VideoRef url.URL              `json:"videoref,omitempty"` // References a video resource or a fragment of it.
	Text     GuidedNavigationText `json:"text,omitempty"`     // Textual description.
}

// NewTextDescription creates a description carrying only plain text.
func NewTextDescription(text string) GuidedNavigationDescription {
	return GuidedNavigationDescription{
		Text: GuidedNavigationText{Plain: text},
	}
}

func (d GuidedNavigationDescription) Empty() bool {
	return d.AudioRef == nil && d.ImgRef == nil && d.TextRef == nil && d.VideoRef == nil && d.Text.Empty()
}

func (d *GuidedNavigationDescription) UnmarshalJSON(data []byte) error {
	// Just plain text
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		d.Text.Plain = plain
		return nil
	}

	var obj struct {
		AudioRef string               `json:"audioref"`
		ImgRef   string               `json:"imgref"`
		TextRef  string               `json:"textref"`
		VideoRef string               `json:"videoref"`
		Text     GuidedNavigationText `json:"text"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	var res GuidedNavigationDescription
	var err error
	if obj.AudioRef != "" {
		if res.AudioRef, err = url.URLFromString(obj.AudioRef); err != nil {
			return err
		}
	}
	if obj.ImgRef != "" {
		if res.ImgRef, err = url.URLFromString(obj.ImgRef); err != nil {
			return err
		}
	}
	if obj.TextRef != "" {
		if res.TextRef, err = url.URLFromString(obj.TextRef); err != nil {
			return err
		}
	}
	if obj.VideoRef != "" {
		if res.VideoRef, err = url.URLFromString(obj.VideoRef); err != nil {
			return err
		}
	}
	res.Text = obj.Text
	*d = res
	return nil
}

func (d GuidedNavigationDescription) MarshalJSON() ([]byte, error) {
	// Plain-text-only descriptions are serialized as a string, matching the
	// examples of the specification.
	if d.AudioRef == nil && d.ImgRef == nil && d.TextRef == nil && d.VideoRef == nil &&
		d.Text.SSML == "" && d.Text.Language == "" {
		return json.Marshal(d.Text.Plain)
	}

	res := make(map[string]interface{})
	if d.AudioRef != nil {
		if s := d.AudioRef.String(); s != "" {
			res["audioref"] = s
		}
	}
	if d.ImgRef != nil {
		if s := d.ImgRef.String(); s != "" {
			res["imgref"] = s
		}
	}
	if d.TextRef != nil {
		if s := d.TextRef.String(); s != "" {
			res["textref"] = s
		}
	}
	if d.VideoRef != nil {
		if s := d.VideoRef.String(); s != "" {
			res["videoref"] = s
		}
	}
	if !d.Text.Empty() {
		res["text"] = d.Text
	}
	return json.Marshal(res)
}
