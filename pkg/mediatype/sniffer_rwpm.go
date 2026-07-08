package mediatype

import "strings"

// Duck-typed inspection of a Readium Web Publication Manifest parsed as generic JSON.
// The manifest package depends on mediatype, so sniffers can't use [manifest.Manifest];
// instead these helpers mirror the conformance heuristics of Manifest.ConformsTo on the
// raw JSON.

// Readium Web Publication Profiles, from the registry:
// https://readium.org/webpub-manifest/profiles/
const (
	rwpmProfileAudiobook = "https://readium.org/webpub-manifest/profiles/audiobook"
	rwpmProfileDivina    = "https://readium.org/webpub-manifest/profiles/divina"
	rwpmProfilePDF       = "https://readium.org/webpub-manifest/profiles/pdf"
)

const lcpSchemeURI = "http://readium.org/2014/01/lcp"

// Returns whether the JSON has a `metadata` object bearing a valid title: a plain
// string or a localized-string object whose translations are all strings, as
// [manifest.LocalizedStringFromJSON] requires. This is the minimal RWPM signature,
// shared by Readium manifests and OPDS 2 documents.
func rwpmHasMetadataTitle(js map[string]interface{}) bool {
	metadata, ok := js["metadata"].(map[string]interface{})
	if !ok {
		return false
	}
	switch title := metadata["title"].(type) {
	case string: // Plain title
		return true
	case map[string]interface{}: // Localized title: every translation must be a string
		for _, v := range title {
			if _, ok := v.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Returns whether the JSON has the shape of a RWPM that [manifest.ManifestFromJSON]
// would accept: a `metadata` object bearing a title, along with a `readingOrder`
// (or legacy `spine`) array of link objects which have an `href` and a valid `type`.
func isRWPMJSON(js map[string]interface{}) bool {
	if js == nil {
		return false
	}

	if !rwpmHasMetadataTitle(js) {
		return false
	}

	rawLinks, ok := js["readingOrder"].([]interface{})
	if !ok {
		if rawLinks, ok = js["spine"].([]interface{}); !ok {
			return false
		}
	}
	for _, raw := range rawLinks {
		link, ok := raw.(map[string]interface{})
		if !ok {
			return false
		}
		if href, ok := link["href"].(string); !ok || href == "" {
			return false
		}
		// A missing `type` only drops the link from the reading order,
		// but an unparsable one fails the whole manifest.
		if typ, ok := link["type"].(string); ok && typ != "" {
			if _, err := NewOfString(typ); err != nil {
				return false
			}
		}
	}
	return true
}

// Returns the link objects of the given collection ("readingOrder", "resources", "links"...).
func rwpmLinks(js map[string]interface{}, key string) []map[string]interface{} {
	raw, ok := js[key].([]interface{})
	if !ok {
		return nil
	}
	links := make([]map[string]interface{}, 0, len(raw))
	for _, v := range raw {
		if link, ok := v.(map[string]interface{}); ok {
			links = append(links, link)
		}
	}
	return links
}

// Returns the media types of the reading order (or legacy spine) links.
// Links without a valid media type are skipped, like [manifest.ManifestFromJSON] drops them.
func rwpmReadingOrderTypes(js map[string]interface{}) []MediaType {
	links := rwpmLinks(js, "readingOrder")
	if links == nil {
		links = rwpmLinks(js, "spine")
	}
	types := make([]MediaType, 0, len(links))
	for _, link := range links {
		typ, ok := link["type"].(string)
		if !ok {
			continue
		}
		mt, err := NewOfString(typ)
		if err != nil {
			continue
		}
		types = append(types, mt)
	}
	return types
}

// Returns whether the RWPM conforms to the given profile, mirroring
// [manifest.Manifest.ConformsTo]: either the profile is declared in
// `metadata.conformsTo`, or every reading order resource satisfies [matches].
func rwpmConformsTo(js map[string]interface{}, profile string, matches func(mt MediaType) bool) bool {
	types := rwpmReadingOrderTypes(js)
	if len(types) == 0 {
		return false
	}

	metadata, _ := js["metadata"].(map[string]interface{})
	switch conformsTo := metadata["conformsTo"].(type) {
	case string:
		if conformsTo == profile {
			return true
		}
	case []interface{}:
		for _, v := range conformsTo {
			if v == profile {
				return true
			}
		}
	}

	for _, mt := range types {
		if !matches(mt) {
			return false
		}
	}
	return true
}

// Returns whether the `self` link of the RWPM matches one of the given media types.
func rwpmSelfLinkMatches(js map[string]interface{}, mediaTypes ...*MediaType) bool {
	for _, link := range rwpmLinks(js, "links") {
		rels, ok := link["rel"].([]interface{})
		if !ok {
			rels = []interface{}{link["rel"]}
		}
		isSelf := false
		for _, rel := range rels {
			if rel == "self" {
				isSelf = true
				break
			}
		}
		if !isSelf {
			continue
		}
		typ, ok := link["type"].(string)
		if !ok {
			continue
		}
		mt, err := NewOfString(typ)
		if err != nil {
			continue
		}
		if mt.Matches(mediaTypes...) {
			return true
		}
	}
	return false
}

// Returns whether a link of the RWPM's `links` collection has a relation starting with
// the given prefix.
func rwpmHasLinkWithRelPrefix(js map[string]interface{}, prefix string) bool {
	for _, link := range rwpmLinks(js, "links") {
		rels, ok := link["rel"].([]interface{})
		if !ok {
			rels = []interface{}{link["rel"]}
		}
		for _, rel := range rels {
			if s, ok := rel.(string); ok && strings.HasPrefix(s, prefix) {
				return true
			}
		}
	}
	return false
}

// Returns whether a reading order resource of the RWPM is encrypted with the LCP scheme.
func rwpmHasLCPScheme(js map[string]interface{}) bool {
	links := rwpmLinks(js, "readingOrder")
	if links == nil {
		links = rwpmLinks(js, "spine")
	}
	for _, link := range links {
		properties, ok := link["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		encrypted, ok := properties["encrypted"].(map[string]interface{})
		if !ok {
			continue
		}
		if encrypted["scheme"] == lcpSchemeURI {
			return true
		}
	}
	return false
}
