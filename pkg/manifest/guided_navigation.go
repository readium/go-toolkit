package manifest

// Readium Guided Navigation Document
// https://readium.org/guided-navigation/schema/document.schema.json
type GuidedNavigationDocument struct {
	Links  LinkList                 `json:"links,omitempty"` // References to other resources that are related to the current Guided Navigation Document.
	Guided []GuidedNavigationObject `json:"guided"`          // A sequence of resources and/or media fragments into these resources, meant to be presented sequentially to the user.
}

// Readium Guided Navigation Object
// https://readium.org/guided-navigation/schema/object.schema.json
// TODO: Role should be typed
// TODO: all refs should be url.URL
type GuidedNavigationObject struct {
	AudioRef string                   `json:"audioref,omitempty"` // References an audio resource or a fragment of it.
	ImgRef   string                   `json:"imgref,omitempty"`   // References an image or a fragment of it.
	TextRef  string                   `json:"textref,omitempty"`  // References a textual resource or a fragment of it.
	Text     string                   `json:"text,omitempty"`     // Textual equivalent of the resources or fragment of the resources referenced by the current Guided Navigation Object.
	Role     []string                 `json:"role,omitempty"`     // Convey the structural semantics of a publication
	Children []GuidedNavigationObject `json:"children,omitempty"` // Items that are children of the containing Guided Navigation Object.
}

// TODO: functions for objects to get e.g. audio time, audio file, text file, fragment id, audio "clip", image xywh, etc.
// This will come after the URL utility revamp to avoid implementation twice
