package manifest

import (
	"encoding/json"
	"testing"

	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
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
			{"href": "manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html"}
		],
		"resources": [
			{"href": "image.png", "type": "image/png"}
		],
		"toc": [
			{"href": "cover.html"},
			{"href": "chap1.html"}
		],
		"sub": {
			"links": [
				{"href": "sublink"}
			]
		}
	}`), &m))

	assert.Equal(t, Manifest{
		Context: []string{"https://readium.org/webpub-manifest/context.jsonld"},
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: MustNewHREFFromString("manifest.json", false), Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: MustNewHREFFromString("chap1.html", false), MediaType: &mediatype.HTML},
		},
		Resources: LinkList{
			Link{Href: MustNewHREFFromString("image.png", false), MediaType: &mediatype.PNG},
		},
		TableOfContents: LinkList{
			Link{Href: MustNewHREFFromString("cover.html", false)},
			Link{Href: MustNewHREFFromString("chap1.html", false)},
		},
		Subcollections: PublicationCollectionMap{
			"sub": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: MustNewHREFFromString("sublink", false)}},
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
			{"href": "manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Context: []string{"context1", "context2"},
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: MustNewHREFFromString("manifest.json", false), Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: MustNewHREFFromString("chap1.html", false), MediaType: &mediatype.HTML},
		},
	}, m, "unmarshalled JSON object should be equal to Manifest object with @context array")
}

func TestManifestUnmarshalJSONRequiresMetadata(t *testing.T) {
	var m Manifest
	assert.Error(t, json.Unmarshal([]byte(`{
		"links": [
			{"href": "manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html"}
		]
	}`), &m))
}

// {readingOrder} used to be {spine}, so we parse {spine} as a fallback.
func TestManifestUnmarshalJSONSpinFallback(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "manifest.json", "rel": "self"}
		],
		"spine": [
			{"href": "chap1.html", "type": "text/html"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: MustNewHREFFromString("manifest.json", false), Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: MustNewHREFFromString("chap1.html", false), MediaType: &mediatype.HTML},
		},
	}, m)
}

/*func TestManifestUnmarshalJSONIgnoresMissingReadingOrderType(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html"},
			{"href": "chap2.html"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: MustNewHREFFromString( "manifest.json", false), Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: MustNewHREFFromString( "chap1.html", false), MediaType: &mediatype.HTML},
		},
	}, m)
}

func TestManifestUnmarshalJSONIgnoresResourceWithoutType(t *testing.T) {
	var m Manifest
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html"}
		],
		"resources": [
			{"href": "withtype", "type": "text/html"},
			{"href": "withouttype"}
		]
	}`), &m))

	assert.Equal(t, Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString("Title"),
		},
		Links: LinkList{
			Link{Href: MustNewHREFFromString( "manifest.json", false), Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: MustNewHREFFromString( "chap1.html", false), MediaType: &mediatype.HTML},
		},
		Resources: LinkList{
			Link{Href: MustNewHREFFromString( "withtype", false), MediaType: &mediatype.HTML},
		},
	}, m)
}*/

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
			Link{Href: MustNewHREFFromString("manifest.json", false), Rels: Strings{"self"}},
		},
		ReadingOrder: LinkList{
			Link{Href: MustNewHREFFromString("chap1.html", false), MediaType: &mediatype.HTML},
		},
		Resources: LinkList{
			Link{Href: MustNewHREFFromString("image.png", false), MediaType: &mediatype.PNG},
		},
		TableOfContents: LinkList{
			Link{Href: MustNewHREFFromString("cover.html", false)}, Link{Href: MustNewHREFFromString("chap1.html", false)},
		},
		Subcollections: PublicationCollectionMap{
			"sub": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: MustNewHREFFromString("sublink", false)}},
			}},
		},
	})
	assert.NoError(t, err)

	assert.JSONEq(t, `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {"title": "Title"},
		"links": [
			{"href": "manifest.json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "chap1.html", "type": "text/html"}
		],
		"resources": [
			{"href": "image.png", "type": "image/png"}
		],
		"toc": [
			{"href": "cover.html"},
			{"href": "chap1.html"}
		],
		"sub": {
			"metadata": {},
			"links": [
				{"href": "sublink"}
			]
		}
	}`, string(bin))
}

func TestManifestSelfLinkReplacedWhenPackaged(t *testing.T) {
	var rm map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "manifest.json", "rel": ["self"], "templated": false}
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
			Link{Href: MustNewHREFFromString("manifest.json", false), Rels: Strings{"alternate"}},
		},
		ReadingOrder: LinkList{},
	}, *m)
}

func TestManifestSelfLinkKeptWhenRemote(t *testing.T) {
	var rm map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {"title": "Title"},
		"links": [
			{"href": "manifest.json", "rel": ["self"], "templated": false}
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
			Link{Href: MustNewHREFFromString("manifest.json", false), Rels: Strings{"self"}},
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
	m2 := m.NormalizeHREFsToSelf()

	assert.Equal(t, "chap1.html", m2.ReadingOrder[0].Href.String())
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
	m2 := m.NormalizeHREFsToSelf()

	assert.Equal(t, "http://example.com/directory/chap1.html", m2.ReadingOrder[0].Href.String())
}

func TestManifestLocatorFromMinimalLink(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:      MustNewHREFFromString("href", false),
			MediaType: &mediatype.HTML,
			Title:     "Resource",
		}},
	}

	var z float64
	assert.Equal(t, &Locator{
		Href:      url.MustURLFromString("href"),
		MediaType: mediatype.HTML,
		Title:     "Resource",
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: MustNewHREFFromString("href", false),
	}))
}

func TestManifestLocatorFromInside(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:      MustNewHREFFromString("href1", false),
			MediaType: &mediatype.HTML,
		}},
		Resources: LinkList{{
			Href:      MustNewHREFFromString("href2", false),
			MediaType: &mediatype.HTML,
		}},
		Links: LinkList{{
			Href:      MustNewHREFFromString("href3", false),
			MediaType: &mediatype.HTML,
		}},
	}

	var z float64
	assert.Equal(t, &Locator{
		Href:      url.MustURLFromString("href1"),
		MediaType: mediatype.HTML,
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: MustNewHREFFromString("href1", false),
	}))
	assert.Equal(t, &Locator{
		Href:      url.MustURLFromString("href2"),
		MediaType: mediatype.HTML,
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: MustNewHREFFromString("href2", false),
	}))
	assert.Equal(t, &Locator{
		Href:      url.MustURLFromString("href3"),
		MediaType: mediatype.HTML,
		Locations: Locations{
			Progression: &z,
		},
	}, manifest.LocatorFromLink(Link{
		Href: MustNewHREFFromString("href3", false),
	}))
}

func TestManifestLocatorFromFullLinkWithFragment(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:      MustNewHREFFromString("href", false),
			MediaType: &mediatype.HTML,
			Title:     "Resource",
		}},
	}

	assert.Equal(t, &Locator{
		Href:      url.MustURLFromString("href"),
		MediaType: mediatype.HTML,
		Title:     "Resource",
		Locations: Locations{
			Fragments: []string{"page=42"},
		},
	}, manifest.LocatorFromLink(Link{
		Href:      MustNewHREFFromString("href#page=42", false),
		MediaType: &mediatype.XML,
		Title:     "My link",
	}))
}

func TestManifestLocatorFallbackTitle(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:      MustNewHREFFromString("href", false),
			MediaType: &mediatype.HTML,
		}},
	}
	assert.Equal(t, &Locator{
		Href:      url.MustURLFromString("href"),
		MediaType: mediatype.HTML,
		Title:     "My link",
		Locations: Locations{
			Fragments: []string{"page=42"},
		},
	}, manifest.LocatorFromLink(Link{
		Href:      MustNewHREFFromString("href#page=42", false),
		MediaType: &mediatype.HTML,
		Title:     "My link",
	}))
}

func TestManifestLocatorLinkNotFound(t *testing.T) {
	manifest := Manifest{
		Metadata: Metadata{
			LocalizedTitle: NewLocalizedStringFromString(""),
		},
		ReadingOrder: LinkList{{
			Href:      MustNewHREFFromString("href", false),
			MediaType: &mediatype.HTML,
		}},
	}
	assert.Nil(t, manifest.LocatorFromLink(Link{
		Href: MustNewHREFFromString("notfound", false),
	}))
}
