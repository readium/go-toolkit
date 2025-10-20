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
	AudioRef    url.URL                  `json:"audioref,omitempty"`    // References an audio resource or a fragment of it.
	ImgRef      url.URL                  `json:"imgref,omitempty"`      // References an image or a fragment of it.
	TextRef     url.URL                  `json:"textref,omitempty"`     // References a textual resource or a fragment of it.
	Text        GuidedNavigationText     `json:"text,omitempty"`        // Textual equivalent of the resources or fragment of the resources referenced by the current Guided Navigation Object.
	Level       uint8                    `json:"level,omitempty"`       // Level 1-6, for e.g. headings
	Role        []GuidedNavigationRole   `json:"role,omitempty"`        // Convey the structural semantics of a publication
	Children    []GuidedNavigationObject `json:"children,omitempty"`    // Items that are children of the containing Guided Navigation Object.
	Description string                   `json:"description,omitempty"` // Text, audio or image description for the current Guided Navigation Object.
}

func (o GuidedNavigationObject) Empty() bool {
	return o.TextRef == nil && o.ImgRef == nil && o.AudioRef == nil && o.Text.Empty() && len(o.Children) == 0 && o.Description == ""
}

func (o GuidedNavigationObject) ChildrenOnly() bool {
	return o.TextRef == nil && o.ImgRef == nil && o.AudioRef == nil && o.Text.Empty() && len(o.Children) > 0 && o.Description == "" && len(o.Role) == 0 && o.Level == 0
}

func (o GuidedNavigationObject) TextOnly() bool {
	return o.TextRef == nil && o.ImgRef == nil && o.AudioRef == nil && !o.Text.Empty() && len(o.Children) == 0 && o.Description == "" && len(o.Role) == 0 && o.Level == 0
}

func (o GuidedNavigationObject) MarshalJSON() ([]byte, error) {
	res := make(map[string]interface{})
	if o.Empty() {
		return json.Marshal(res)
	}

	if o.TextRef != nil {
		if s := o.TextRef.String(); s != "" {
			res["textref"] = o.TextRef.String()
		}
	}
	if o.ImgRef != nil {
		if s := o.ImgRef.String(); s != "" {
			res["imgref"] = o.ImgRef.String()
		}
	}
	if o.AudioRef != nil {
		if s := o.AudioRef.String(); s != "" {
			res["audioref"] = o.AudioRef.String()
		}
	}

	if (o.Text != GuidedNavigationText{}) {
		res["text"] = o.Text
	}
	if o.Level != 0 {
		res["level"] = o.Level
	}
	if len(o.Role) > 0 {
		res["role"] = o.Role
	}
	if len(o.Children) > 0 {
		res["children"] = o.Children
	}
	if o.Description != "" {
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

// Same as GuidedNavigationObject but without Children
/*type GuidedNavigationDescriptionObject struct {
	AudioRef url.URL  `json:"audioref,omitempty"` // References an audio resource or a fragment of it.
	ImgRef   url.URL  `json:"imgref,omitempty"`   // References an image or a fragment of it.
	TextRef  url.URL  `json:"textref,omitempty"`  // References a textual resource or a fragment of it.
	Text     string   `json:"text,omitempty"`     // Textual equivalent of the resources or fragment of the resources referenced by the current Guided Navigation Object.
	Level    uint8    `json:"level,omitempty"`    // TODO
	Role     []string `json:"role,omitempty"`     // Convey the structural semantics of a publication
}*/

// TODO: functions for objects to get e.g. audio time, audio file, text file, fragment id, audio "clip", image xywh, etc.
// This will come after the URL utility revamp to avoid implementation twice
