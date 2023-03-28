package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManifestUnmarshalMinimalJSON(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [],
		"readingOrder": []
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links:        LinkList{},
		ReadingOrder: LinkList{},
	}, m, "unmarshalled JSON object should be equal to Manifest object")
}

func TestManifestUnmarshalFullJSON(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "/chap1.html", "type": "text/html"}
		],
		"resources": [
			{"href": "/image.png", "type": "image/png"}
		],
		"toc": [
			{"href": "/cover.html"},
			{"href": "/chap1.html"}
		],
		"sub": {
			"links": [
				{"href": "/sublink"}
			]
		}
	}`), &m))

	assert.Equal(t, Manifest{
		Context: []string{"https://readium.org/webpub-manifest/context.jsonld"},
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: "/chap1.html", Type: "text/html"},
		},
		Resources: LinkList{
			Link{Href: "/image.png", Type: "image/png"},
		},
		TableOfContents: LinkList{
			Link{Href: "/cover.html"},
			Link{Href: "/chap1.html"},
		},
		Subcollections: PublicationCollectionMap{
			"sub": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: "/sublink"}},
			}},
		},
	}, m, "unmarshalled JSON object should be equal to Manifest object")
}

func TestManifestUnmarshalJSONContextAsArray(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"@context": ["context1", "context2"],
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "/chap1.html", "type": "text/html"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Context: []string{"context1", "context2"},
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: "/chap1.html", Type: "text/html"},
		},
	}, m, "unmarshalled JSON object should be equal to Manifest object with @context array")
}

func TestManifestUnmarshalJSONRequiresMetadata(t *testing.T) {
	var m Manifest
	assert.Error(t, json.Unmarshal([]byte(`{
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "/chap1.html", "type": "text/html"}
		]
	}`), &m))
}

// {readingOrder} used to be {spine}, so we parse {spine} as a fallback.
func TestManifestUnmarshalJSONSpinFallback(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"spine": [
			{"href": "/chap1.html", "type": "text/html"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: "/chap1.html", Type: "text/html"},
		},
	}, m)
}

func TestManifestUnmarshalJSONIgnoresMissingReadingOrderType(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "/chap1.html", "type": "text/html"},
			{"href": "/chap2.html"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: "/chap1.html", Type: "text/html"},
		},
	}, m)
}

func TestManifestUnmarshalJSONIgnoresResourceWithoutType(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "/chap1.html", "type": "text/html"}
		],
		"resources": [
			{"href": "/withtype", "type": "text/html"},
			{"href": "/withouttype"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: "/chap1.html", Type: "text/html"},
		},
		Resources: LinkList{
			Link{Href: "/withtype", Type: "text/html"},
		},
	}, m)
}

func TestManifestMinimalJSON(t *testing.T) {
	bin, err := json.Marshal(Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links:        LinkList{},
		ReadingOrder: LinkList{},
	})
	assert.NoError(t, err)

	assert.JSONEq(t, `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {"title": "Title"},
		"links": [],
		"readingOrder": []
	}`, string(bin))
}

func TestManifestFullJSON(t *testing.T) {
	bin, err := json.Marshal(Manifest{
		Context: []string{"https://readium.org/webpub-manifest/context.jsonld"},
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: "/chap1.html", Type: "text/html"},
		},
		Resources: LinkList{
			Link{Href: "/image.png", Type: "image/png"},
		},
		TableOfContents: LinkList{
			Link{Href: "/cover.html"}, Link{Href: "/chap1.html"},
		},
		Subcollections: PublicationCollectionMap{
			"sub": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: "/sublink"}},
			}},
		},
	})
	assert.NoError(t, err)

	assert.JSONEq(t, `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "/chap1.html", "type": "text/html"}
		],
		"resources": [
			{"href": "/image.png", "type": "image/png"}
		],
		"toc": [
			{"href": "/cover.html"},
			{"href": "/chap1.html"}
		],
		"sub": {
			"metadata": {},
			"links": [
				{"href": "/sublink"}
			]
		}
	}`, string(bin))
}

func TestManifestSelfLinkReplacedWhenPackaged(t *testing.T) {
	var rm map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": ["self"], "templated": false}
		],
		"readingOrder": []
	}`), &rm))
	m, err := ManifestFromJSON(rm, true)
	assert.NoError(t, err)

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"alternate"}},
		},
		ReadingOrder: LinkList{},
	}, *m)
}

func TestManifestSelfLinkKeptWhenRemote(t *testing.T) {
	var rm map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "/manifest.json", "rel": ["self"], "templated": false}
		],
		"readingOrder": []
	}`), &rm))
	m, err := ManifestFromJSON(rm, false)
	assert.NoError(t, err)

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: "/manifest.json", Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{},
	}, *m)
}

func TestManifestHrefResolvedToRoot(t *testing.T) {
	var rm map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "http://example.com/manifest.json", "rel": ["self"], "templated": false}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html", "templated": false}
		]
	}`), &rm))

	m, err := ManifestFromJSON(rm, true)
	assert.NoError(t, err)

	assert.Equal(t, "/chap1.html", m.ReadingOrder[0].Href)
}

func TestManifestHrefResolvedToRootRemotePackage(t *testing.T) {
	var rm map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "http://example.com/directory/manifest.json", "rel": ["self"], "templated": false}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html", "templated": false}
		]
	}`), &rm))

	m, err := ManifestFromJSON(rm, false)
	assert.NoError(t, err)

	assert.Equal(t, "http://example.com/directory/chap1.html", m.ReadingOrder[0].Href)
}

func TestManifestLocatorFromMinimalLink(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:  "/href",
			Type:  "text/html",
			Title: "Resource",
		}},
	}

	var z float64
	assert.Equal(t, &Locator{
		Href:  "/href",
		Type:  "text/html",
		Title: "Resource",
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: "/href",
	}))
}

func TestManifestLocatorFromInside(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href: "/href1",
			Type: "text/html",
		}},
		Resources: LinkList{{
			Href: "/href2",
			Type: "text/html",
		}},
		Links: LinkList{{
			Href: "/href3",
			Type: "text/html",
		}},
	}

	var z float64
	assert.Equal(t, &Locator{
		Href: "/href1",
		Type: "text/html",
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: "/href1",
	}))
	assert.Equal(t, &Locator{
		Href: "/href2",
		Type: "text/html",
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: "/href2",
	}))
	assert.Equal(t, &Locator{
		Href: "/href3",
		Type: "text/html",
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: "/href3",
	}))
}

func TestManifestLocatorFromFullLinkWithFragment(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:  "/href",
			Type:  "text/html",
			Title: "Resource",
		}},
	}

	assert.Equal(t, &Locator{
		Href:  "/href",
		Type:  "text/html",
		Title: "Resource",
		Locations: Locations{
			Fragments: []string{"page=42"},
		},
	}, manifest.LocatorFromLink(Link{
		Href:  "/href#page=42",
		Type:  "text/xml",
		Title: "My link",
	}))
}

func TestManifestLocatorFallbackTitle(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href: "/href",
			Type: "text/html",
		}},
	}
	assert.Equal(t, &Locator{
		Href:  "/href",
		Type:  "text/html",
		Title: "My link",
		Locations: Locations{
			Fragments: []string{"page=42"},
		},
	}, manifest.LocatorFromLink(Link{
		Href:  "/href#page=42",
		Type:  "text/xml",
		Title: "My link",
	}))
}

func TestManifestLocatorLinkNotFound(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href: "/href",
			Type: "text/html",
		}},
	}
	assert.Nil(t, manifest.LocatorFromLink(Link{
		Href: "/notfound",
	}))
}
