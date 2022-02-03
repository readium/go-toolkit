package manifest

import (
	"encoding/json"
	"sort"

	"github.com/pkg/errors"
)

type Collection = Contributor
type Collections = Contributors

type PublicationCollectionMap map[string][]PublicationCollection

// PublicationCollection can be used as extension point in the Readium Web Publication Manifest.
// https://readium.org/webpub-manifest/schema/subcollection.schema.json
type PublicationCollection struct {
	Metadata       map[string]interface{}   `json:"metadata,omitempty"`
	Links          []Link                   `json:"links,omitempty"`
	Subcollections PublicationCollectionMap `json:"-"`
}

func appendPublicationCollectionToJSON(pc PublicationCollectionMap, obj map[string]interface{}) {
	for role, collections := range pc {
		if len(collections) == 0 {
			continue
		}
		if len(collections) == 1 {
			obj[role] = collections[0]
		} else {
			obj[role] = collections
		}
	}
}

// Parses a [PublicationCollection] from its RWPM JSON representation.
//
// TODO log [warnings] ?
//  The [links]' href and their children's will be normalized recursively using the provided [normalizeHref] closure.
func PublicationCollectionFromJSON(rawJson interface{}, normalizeHref LinkHrefNormalizer) (*PublicationCollection, error) {
	if rawJson == nil {
		return nil, nil
	}

	var err error
	var links []Link
	// var metadata map[string]interface{}
	metadata := make(map[string]interface{})
	var subcollections PublicationCollectionMap
	//subcollections := make(PublicationCollectionMap)

	switch dd := rawJson.(type) {
	case map[string]interface{}:
		lkz, ok := dd["links"].([]interface{})
		if ok {
			links, err = LinksFromJSONArray(lkz, normalizeHref)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshalling 'links'")
			}
		}

		mtd, ok := dd["metadata"].(map[string]interface{})
		if ok {
			metadata = mtd
		}

		delete(dd, "links")
		delete(dd, "metadata")

		subcollections, err = PublicationCollectionsFromJSON(dd, normalizeHref)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling subcollections")
		}
	case []interface{}:
		links, err = LinksFromJSONArray(dd, normalizeHref)
		if err != nil {
			return nil, errors.Wrap(err, "failed unmarshalling as Link array")
		}
	default:
		return nil, errors.Errorf("core collection of invalid type %T", rawJson)
	}

	if len(links) == 0 {
		return nil, errors.New("core collection must have at least one link")
	}

	return &PublicationCollection{
		Metadata:       metadata,
		Links:          links,
		Subcollections: subcollections,
	}, nil
}

func (pc *PublicationCollection) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fpc, err := PublicationCollectionFromJSON(object, LinkHrefNormalizerIdentity)
	if err != nil {
		return err
	}
	*pc = *fpc
	return nil
}

func (pc PublicationCollection) MarshalJSON() ([]byte, error) {
	if pc.Metadata == nil && pc.Subcollections == nil {
		return json.Marshal(pc.Links)
	}
	res := make(map[string]interface{})
	res["metadata"] = pc.Metadata
	res["links"] = pc.Links
	appendPublicationCollectionToJSON(pc.Subcollections, res)

	return json.Marshal(res)
}

// Parses a map of [PublicationCollection] indexed by their roles from its RWPM JSON representation.
//
// The [Links]' href and their children's will be normalized recursively using the provided [normalizeHref] closure.
func PublicationCollectionsFromJSON(rawJson map[string]interface{}, normalizeHref LinkHrefNormalizer) (PublicationCollectionMap, error) {
	if rawJson == nil {
		return nil, nil
	}
	collections := PublicationCollectionMap{}
	roles := make([]string, len(rawJson))
	i := 0
	for role := range rawJson {
		roles[i] = role
		i++
	}
	sort.Strings(roles)
	for _, role := range roles {
		sub := rawJson[role]

		// Parses a list of links or a single collection object.
		collection, err := PublicationCollectionFromJSON(sub, normalizeHref)
		if collection != nil {
			if _, ok := collections[role]; ok {
				collections[role] = append(collections[role], *collection)
			} else {
				collections[role] = []PublicationCollection{*collection}
			}
		} else if subArr, ok := sub.([]interface{}); ok {
			// Parses a list of collection objects.
			var newCollections []PublicationCollection
			for j, v := range subArr {
				c, err := PublicationCollectionFromJSON(v, normalizeHref)
				if err != nil {
					return nil, errors.Wrapf(err, "failed unmarshalling PublicationCollection for role %s at position %d", role, j)
				}
				if c != nil {
					newCollections = append(newCollections, *c)
				}
			}
			if len(newCollections) > 0 {
				if _, ok := collections[role]; ok {
					collections[role] = append(collections[role], newCollections...)
				} else {
					collections[role] = newCollections
				}
			}
		} else if err != nil {
			return nil, errors.Wrapf(err, "failed unmarshalling PublicationCollection object for role %s", role)
		}
	}
	if len(collections) == 0 {
		return nil, nil
	}
	return collections, nil
}

func (pcm *PublicationCollectionMap) UnmarshalJSON(b []byte) error {
	var object map[string]interface{}
	err := json.Unmarshal(b, &object)
	if err != nil {
		return err
	}
	fpc, err := PublicationCollectionsFromJSON(object, LinkHrefNormalizerIdentity)
	if err != nil {
		return err
	}
	*pcm = fpc
	return nil
}

func (pcm PublicationCollectionMap) MarshalJSON() ([]byte, error) {
	if len(pcm) == 0 {
		return []byte("null"), nil
	}

	res := make(map[string]interface{})

	for k, v := range pcm {
		if len(v) == 0 {
			continue
		}
		if len(v) == 1 {
			res[k] = v[0]
		} else {
			res[k] = v
		}
	}

	return json.Marshal(res)
}
