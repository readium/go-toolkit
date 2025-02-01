package manifest

import (
	"encoding/json"
	"testing"

	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

func TestPubCollectionUnmarshalMinimalJSON(t *testing.T) {
	var pc PublicationCollection
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": null,
		"links": [{"href": "/link"}]
	}`), &pc))

	assert.Equal(t, PublicationCollection{
		Links:    []Link{{Href: NewHREF(url.MustURLFromString("/link"))}},
		Metadata: map[string]interface{}{},
	}, pc, "unmarshalled JSON object should be equal to PublicationCollection object")
}

func TestPubCollectionUnmarshalFullJSON(t *testing.T) {
	var pc PublicationCollection
	assert.NoError(t, json.Unmarshal([]byte(`{
		"metadata": {
			"metadata1": "value"
		},
		"links": [
			{"href": "/link"}
		],
		"sub1": {
			"links": [
				{"href": "/sublink"}
			]
		},
		"sub2": [
			{"href": "/sublink1"},
			{"href": "/sublink2"}
		],
		"sub3": [
			{
				"links": [
					{"href": "/sublink3"}
				]
			},
			{
				"links": [
					{"href": "/sublink4"}
				]
			}
		]
	}`), &pc))

	assert.Equal(t, PublicationCollection{
		Links: []Link{{Href: NewHREF(url.MustURLFromString("/link"))}},
		Metadata: map[string]interface{}{
			"metadata1": "value",
		},
		Subcollections: PublicationCollectionMap{
			"sub1": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink"))}},
			}},
			"sub2": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink1"))}, {Href: NewHREF(url.MustURLFromString("/sublink2"))}},
			}},
			"sub3": {
				{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink3"))}}},
				{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink4"))}}},
			},
		},
	}, pc, "unmarshalled JSON object should be equal to PublicationCollection object")
}

func TestPubCollectionUnmarshalNilJSON(t *testing.T) {
	pc, err := PublicationCollectionFromJSON(nil)
	assert.NoError(t, err)
	assert.Nil(t, pc)
}

func TestPubCollectionUnmarshalJSONMultipleCollections(t *testing.T) {
	var pcsr map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(`{
		"sub1": {
			"links": [
				{"href": "/sublink"}
			]
		},
		"sub2": [
			{"href": "/sublink1"},
			{"href": "/sublink2"}
		],
		"sub3": [
			{
				"links": [
					{"href": "/sublink3"}
				]
			},
			{
				"links": [
					{"href": "/sublink4"}
				]
			}
		]
	}`), &pcsr))
	pcs, err := PublicationCollectionsFromJSON(pcsr)
	assert.NoError(t, err)

	assert.Equal(t, PublicationCollectionMap{
		"sub1": {{
			Metadata: map[string]interface{}{},
			Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink"))}},
		}},
		"sub2": {{
			Metadata: map[string]interface{}{},
			Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink1"))}, {Href: NewHREF(url.MustURLFromString("/sublink2"))}},
		}},
		"sub3": {
			{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink3"))}}},
			{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink4"))}}},
		},
	}, pcs, "unmarshalled JSON object should be equal to map of PublicationCollection to role")
}

func TestPubCollectionMinimalJSON(t *testing.T) {
	bin, err := json.Marshal(&PublicationCollection{
		Metadata: map[string]interface{}{},
		Links:    []Link{{Href: NewHREF(url.MustURLFromString("/link"))}},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"metadata": {},
		"links": [{"href": "/link"}]
	}`, string(bin))
}

func TestPubCollectionFullJSON(t *testing.T) {
	bin, err := json.Marshal(&PublicationCollection{
		Metadata: map[string]interface{}{
			"metadata1": "value",
		},
		Links: []Link{{Href: NewHREF(url.MustURLFromString("/link"))}},
		Subcollections: PublicationCollectionMap{
			"sub1": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink"))}},
			}},
			"sub2": {{
				Metadata: map[string]interface{}{},
				Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink1"))}, {Href: NewHREF(url.MustURLFromString("/sublink2"))}},
			}},
			"sub3": {
				{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink3"))}}},
				{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink4"))}}},
			},
		},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"metadata": {
			"metadata1": "value"
		},
		"links": [
			{"href": "/link"}
		],
		"sub1": {
			"metadata": {},
			"links": [
				{"href": "/sublink"}
			]
		},
		"sub2": {
			"metadata": {},
			"links": [
				{"href": "/sublink1"},
				{"href": "/sublink2"}
			]
		},
		"sub3": [
			{
				"metadata": {},
				"links": [
					{"href": "/sublink3"}
				]
			},
			{
				"metadata": {},
				"links": [
					{"href": "/sublink4"}
				]
			}
		]
	}`, string(bin))
}

func TestPubCollectionMultipleCollectionsJSON(t *testing.T) {
	bin, err := json.Marshal(PublicationCollectionMap{
		"sub1": {{
			Metadata: map[string]interface{}{},
			Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink"))}},
		}},
		"sub2": {{
			Metadata: map[string]interface{}{},
			Links:    []Link{{Href: NewHREF(url.MustURLFromString("/sublink1"))}, {Href: NewHREF(url.MustURLFromString("/sublink2"))}},
		}},
		"sub3": {
			{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink3"))}}},
			{Metadata: map[string]interface{}{}, Links: []Link{{Href: NewHREF(url.MustURLFromString("/sublink4"))}}},
		},
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"sub1": {
			"metadata": {},
			"links": [
				{"href": "/sublink"}
			]
		},
		"sub2": {
			"metadata": {},
			"links": [
				{"href": "/sublink1"},
				{"href": "/sublink2"}
			]
		},
		"sub3": [
			{
				"metadata": {},
				"links": [
					{"href": "/sublink3"}
				]
			},
			{
				"metadata": {},
				"links": [
					{"href": "/sublink4"}
				]
			}
		]
	}`, string(bin))
}
