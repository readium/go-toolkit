package epub

import (
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
)

type PublicationFactory struct {
	FallbackTitle   string
	PackageDocument PackageDocument
	NavigationData  map[string][]manifest.Link
	EncryptionData  map[string]manifest.Encryption
	DisplayOptions  map[string]string

	itemById       map[string]Item
	pubMetadata    PubMetadataAdapter
	itemrefByIdref map[string]ItemRef
	itemMetadata   map[string]LinkMetadataAdapter
}

func (f PublicationFactory) Create() manifest.Manifest {
	// Initialize
	epubVersion := f.PackageDocument.EPUBVersion
	links := f.PackageDocument.metadata.links
	spine := f.PackageDocument.Spine
	mani := f.PackageDocument.Manifest
	f.pubMetadata = PubMetadataAdapter{
		metadataAdapter: metadataAdapter{
			epubVersion: epubVersion,
			items:       f.PackageDocument.metadata.global,
		},
		fallbackTitle:      f.FallbackTitle,
		uniqueIdentifierID: f.PackageDocument.uniqueIdenfifierID,
		readingProgression: spine.direction,
		displayOptions:     f.DisplayOptions,
	}
	f.itemMetadata = make(map[string]LinkMetadataAdapter)
	for k, v := range f.PackageDocument.metadata.refine {
		f.itemMetadata[k] = LinkMetadataAdapter{
			epubVersion: epubVersion,
			items:       v,
		}
	}
	f.itemById = make(map[string]Item)
	for _, item := range mani {
		f.itemById[item.ID] = item
	}
	f.itemrefByIdref = make(map[string]ItemRef)
	for _, v := range spine.itemrefs {
		f.itemrefByIdref[v.idref] = v
	}

	// Compute Metadata
	metadata := f.pubMetadata.Metadata()
	metadataLinks := make([]manifest.Link, 0, len(links))
	for _, link := range links {
		metadataLinks = append(metadataLinks, mapEPUBLink(link))
	}

	// Compute Links
	var readingOrderIds []string
	for _, v := range spine.itemrefs {
		if v.linear {
			readingOrderIds = append(readingOrderIds, v.idref)
		}
	}
	readingOrder := make([]manifest.Link, 0, len(readingOrderIds))
	for _, id := range readingOrderIds {
		item, ok := f.itemById[id]
		if ok {
			readingOrder = append(readingOrder, f.computeLink(item, []string{}))
		}
	}
	readingOrderAllIds := f.computeIdsWithFallbacks(readingOrderIds)
	var resourceItems []Item
	for _, item := range mani {
		if !extensions.Contains(readingOrderAllIds, item.ID) {
			resourceItems = append(resourceItems, item)
		}
	}
	resources := make([]manifest.Link, 0, len(resourceItems))
	for _, item := range resourceItems {
		resources = append(resources, f.computeLink(item, []string{}))
	}

	ret := manifest.Manifest{
		Metadata:       metadata,
		Links:          metadataLinks,
		ReadingOrder:   readingOrder,
		Resources:      resources,
		Subcollections: make(map[string][]manifest.PublicationCollection),
	}

	// Compute TOC and OtherCollections
	if toc, ok := f.NavigationData["toc"]; ok {
		ret.TableOfContents = toc
	}
	for k, v := range f.NavigationData {
		if k == "toc" {
			continue
		}

		// RWPM uses camel case for the roles
		// https://github.com/readium/webpub-manifest/issues/53
		if k == "page-list" {
			k = "pageList"
		}

		ret.Subcollections[k] = []manifest.PublicationCollection{{
			Links: v,
		}}
	}

	return ret
}

// Compute a Publication [Link] from an EPUB metadata link
func mapEPUBLink(link EPUBLink) manifest.Link {
	var contains []string
	if extensions.Contains(link.rels, VOCABULARY_LINK+"record") {
		if extensions.Contains(link.properties, VOCABULARY_LINK+"onix") {
			contains = append(contains, "onix")
		}
		if extensions.Contains(link.properties, VOCABULARY_LINK+"xmp") {
			contains = append(contains, "xmp")
		}
	}
	return manifest.Link{
		Href: link.href,
		Type: link.mediaType,
		Rels: link.rels,
		Properties: manifest.Properties{
			"contains": contains,
		},
	}
}

// Recursively find the ids of the fallback items in [items]
func (f PublicationFactory) computeIdsWithFallbacks(ids []string) []string {
	var fallbackIds []string
	for _, id := range ids {
		for _, v := range f.computeFallbackChain(id) {
			if !extensions.Contains(fallbackIds, v) {
				fallbackIds = append(fallbackIds, v)
			}
		}
	}
	return fallbackIds
}

// Compute the ids contained in the fallback chain of [item]
func (f PublicationFactory) computeFallbackChain(id string) []string {
	// The termination has already been checked while computing links
	var ids []string
	item, ok := f.itemById[id]
	if !ok {
		return ids
	}
	if item.ID != "" {
		ids = append(ids, item.ID)
	}
	if item.fallback != "" {
		for _, v := range f.computeFallbackChain(item.fallback) {
			if !extensions.Contains(ids, v) {
				ids = append(ids, v)
			}
		}
	}
	return ids
}

// Compute a Publication [Link] for an epub [Item] and its fallbacks
func (f PublicationFactory) computeLink(item Item, fallbackChain []string) manifest.Link {
	itemref, _ := f.itemrefByIdref[item.ID]
	rels, properties := f.computePropertiesAndRels(item, &itemref)

	ret := manifest.Link{
		Href:       item.Href,
		Type:       item.MediaType,
		Rels:       rels,
		Properties: properties,
		Alternates: f.computeAlternates(item, fallbackChain),
	}

	if itm, ok := f.itemMetadata[item.ID]; ok {
		if duration := itm.Duration(); duration != nil {
			ret.Duration = *duration
		}
	}

	return ret
}

func (f PublicationFactory) computePropertiesAndRels(item Item, itemref *ItemRef) ([]string, manifest.Properties) {
	properties := make(map[string]interface{})
	var rels []string
	manifestRels, contains, others := parseItemProperties(item.Properties)
	for _, v := range manifestRels {
		if !extensions.Contains(rels, v) {
			rels = append(rels, v)
		}
	}
	if len(contains) > 0 {
		properties["contains"] = contains
	}
	for _, v := range others {
		properties[v] = true
	}
	if itemref != nil {
		for k, v := range parseItemrefProperties(itemref.properties) {
			properties[k] = v
		}
	}

	coverId := f.pubMetadata.Cover()
	if coverId == item.ID {
		rels = append(rels, "cover")
	}

	if edat, ok := f.EncryptionData[item.ID]; ok {
		properties["encryption"] = edat // TODO: determine if .toJSON().toMap() necessary
	}

	return rels, manifest.Properties(properties)
}

// Compute alternate links for [item], checking for an infinite recursion
func (f PublicationFactory) computeAlternates(item Item, fallbackChain []string) (ret []manifest.Link) {
	if item.fallback != "" && !extensions.Contains(fallbackChain, item.fallback) {
		if item, ok := f.itemById[item.fallback]; ok {
			if item.ID != "" {
				updatedChain := make([]string, len(fallbackChain)+1)
				copy(updatedChain, fallbackChain)
				updatedChain[len(fallbackChain)-1] = item.ID
				ret = append(ret, f.computeLink(item, updatedChain))
			} else {
				cloneChain := make([]string, len(fallbackChain))
				copy(cloneChain, fallbackChain)
				ret = append(ret, f.computeLink(item, cloneChain))
			}
		}
	}
	if item.mediaOverlay != "" {
		if item, ok := f.itemById[item.mediaOverlay]; ok {
			ret = append(ret, f.computeLink(item, []string{}))
		}
	}
	return
}

func parseItemProperties(properties []string) (rels []string, contains []string, others []string) {
	for _, property := range properties {
		switch property {
		case VOCABULARY_ITEM + "scripted":
			contains = append(contains, "js")
		case VOCABULARY_ITEM + "mathml":
			contains = append(contains, "mathml")
		case VOCABULARY_ITEM + "svg":
			contains = append(contains, "svg")
		case VOCABULARY_ITEM + "xmp-record":
			contains = append(contains, "xmp")
		case VOCABULARY_ITEM + "remote-resources":
			contains = append(contains, "remote-resources")
		case VOCABULARY_ITEM + "nav":
			rels = append(rels, "contents")
		case VOCABULARY_ITEM + "cover-image":
			rels = append(rels, "cover")
		default:
			others = append(others, property)
		}
	}
	return
}

func parseItemrefProperties(properties []string) map[string]string {
	linkProperties := make(map[string]string)
	for _, property := range properties {
		switch property {
		// Page
		case VOCABULARY_RENDITION + "page-spread-center":
			linkProperties["page"] = "center"
		case VOCABULARY_RENDITION + "page-spread-left":
			fallthrough
		case VOCABULARY_ITEMREF + "page-spread-left":
			linkProperties["page"] = "left"
		case VOCABULARY_RENDITION + "page-spread-right":
			fallthrough
		case VOCABULARY_ITEMREF + "page-spread-right":
			linkProperties["page"] = "right"
		// Spread
		case VOCABULARY_RENDITION + "spread-node":
			linkProperties["spread"] = "none"
		case VOCABULARY_RENDITION + "spread-auto":
			linkProperties["spread"] = "auto"
		case VOCABULARY_RENDITION + "spread-landscape":
			linkProperties["spread"] = "landscape"
		case VOCABULARY_RENDITION + "spread-portrait":
			fallthrough
		case VOCABULARY_RENDITION + "spread-both":
			linkProperties["spread"] = "both"
		// Layout
		case VOCABULARY_RENDITION + "layout-reflowable":
			linkProperties["layout"] = "reflowable"
		case VOCABULARY_RENDITION + "layout-pre-paginated":
			linkProperties["layout"] = "fixed"
		// Orientation
		case VOCABULARY_RENDITION + "orientation-auto":
			linkProperties["orientation"] = "auto"
		case VOCABULARY_RENDITION + "orientation-landscape":
			linkProperties["orientation"] = "landscape"
		case VOCABULARY_RENDITION + "orientation-portrait":
			linkProperties["orientation"] = "portrait"
		// Overflow
		case VOCABULARY_RENDITION + "flow-auto":
			linkProperties["overflow"] = "auto"
		case VOCABULARY_RENDITION + "flow-paginated":
			linkProperties["overflow"] = "paginated"
		case VOCABULARY_RENDITION + "flow-scrolled-continuous":
			fallthrough
		case VOCABULARY_RENDITION + "flow-scrolled-doc":
			linkProperties["overflow"] = "scrolled"
		}
	}
	return linkProperties
}
