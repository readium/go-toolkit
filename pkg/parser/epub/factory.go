package epub

import (
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

type PublicationFactory struct {
	FallbackTitle   string
	PackageDocument PackageDocument
	NavigationData  map[string]manifest.LinkList
	EncryptionData  map[url.URL]manifest.Encryption
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
			links:       f.PackageDocument.metadata.links,
		},
		fallbackTitle:      f.FallbackTitle,
		uniqueIdentifierID: f.PackageDocument.uniqueIdentifierID,
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
	metadata.OtherMetadata[NamespaceOPF+"#version"] = f.PackageDocument.EPUBVersionString
	metadataLinks := make(manifest.LinkList, 0, len(links))
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
	readingOrder := make(manifest.LinkList, 0, len(readingOrderIds))
	for _, id := range readingOrderIds {
		item, ok := f.itemById[id]
		if ok {
			readingOrder = append(readingOrder, f.computeLink(item, []string{}))
		}
	}
	var resourceItems []Item
	for _, item := range mani {
		if !extensions.Contains(readingOrderIds, item.ID) {
			resourceItems = append(resourceItems, item)
		}
	}
	resources := make(manifest.LinkList, 0, len(resourceItems))
	for _, item := range resourceItems {
		resources = append(resources, f.computeLink(item, []string{}))
	}

	ret := manifest.Manifest{
		Context:        manifest.Strings{manifest.WebpubManifestContext},
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
	l := manifest.Link{
		Href:      manifest.NewHREF(link.href),
		MediaType: link.mediaType,
		Rels:      link.rels,
	}

	var contains []string
	if extensions.Contains(link.rels, VocabularyLink+"record") {
		if extensions.Contains(link.properties, VocabularyLink+"onix") {
			contains = append(contains, "onix")
		}
		if extensions.Contains(link.properties, VocabularyLink+"xmp") {
			contains = append(contains, "xmp")
		}
	}

	if len(contains) > 0 {
		l.Properties = manifest.Properties{
			"contains": contains,
		}
	}

	return l
}

// Compute a Publication [Link] for an epub [Item] and its fallbacks
func (f PublicationFactory) computeLink(item Item, fallbackChain []string) manifest.Link {
	itemref, _ := f.itemrefByIdref[item.ID]
	rels, properties := f.computePropertiesAndRels(item, &itemref)

	ret := manifest.Link{
		Href:       manifest.NewHREF(item.Href),
		MediaType:  item.MediaType,
		Rels:       rels,
		Alternates: f.computeAlternates(item, fallbackChain),
	}

	if len(properties) > 0 {
		ret.Properties = properties
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
		rels = extensions.AddToSet(rels, v)
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
		rels = extensions.AddToSet(rels, "cover")
	}

	if edat, ok := f.EncryptionData[item.Href]; ok {
		properties["encrypted"] = edat.ToMap() // ToMap makes it JSON-like
	}

	return rels, manifest.Properties(properties)
}

// Compute alternate links for [item], checking for an infinite recursion
func (f PublicationFactory) computeAlternates(item Item, fallbackChain []string) (ret manifest.LinkList) {
	if item.fallback != "" && !extensions.Contains(fallbackChain, item.fallback) {
		if item, ok := f.itemById[item.fallback]; ok {
			if item.ID != "" {
				updatedChain := make([]string, len(fallbackChain)+1)
				copy(updatedChain, fallbackChain)
				updatedChain[len(updatedChain)-1] = item.ID
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
		case VocabularyItem + "scripted":
			contains = append(contains, "js")
		case VocabularyItem + "mathml":
			contains = append(contains, "mathml")
		case VocabularyItem + "svg":
			contains = append(contains, "svg")
		case VocabularyItem + "xmp-record":
			contains = append(contains, "xmp")
		case VocabularyItem + "remote-resources":
			contains = append(contains, "remote-resources")
		case VocabularyItem + "nav":
			rels = append(rels, "contents")
		case VocabularyItem + "cover-image":
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
		case VocabularyRendition + "page-spread-center":
			linkProperties["page"] = "center"
		case VocabularyRendition + "page-spread-left":
			fallthrough
		case VocabularyItemref + "page-spread-left":
			linkProperties["page"] = "left"
		case VocabularyRendition + "page-spread-right":
			fallthrough
		case VocabularyItemref + "page-spread-right":
			linkProperties["page"] = "right"
		// Spread
		case VocabularyRendition + "spread-node":
			linkProperties["spread"] = "none"
		case VocabularyRendition + "spread-auto":
			linkProperties["spread"] = "auto"
		case VocabularyRendition + "spread-landscape":
			linkProperties["spread"] = "landscape"
		case VocabularyRendition + "spread-portrait":
			fallthrough
		case VocabularyRendition + "spread-both":
			linkProperties["spread"] = "both"
		// Layout
		case VocabularyRendition + "layout-reflowable":
			linkProperties["layout"] = "reflowable"
		case VocabularyRendition + "layout-pre-paginated":
			linkProperties["layout"] = "fixed"
		// Orientation
		case VocabularyRendition + "orientation-auto":
			linkProperties["orientation"] = "auto"
		case VocabularyRendition + "orientation-landscape":
			linkProperties["orientation"] = "landscape"
		case VocabularyRendition + "orientation-portrait":
			linkProperties["orientation"] = "portrait"
		// Overflow
		case VocabularyRendition + "flow-auto":
			linkProperties["overflow"] = "auto"
		case VocabularyRendition + "flow-paginated":
			linkProperties["overflow"] = "paginated"
		case VocabularyRendition + "flow-scrolled-continuous":
			fallthrough
		case VocabularyRendition + "flow-scrolled-doc":
			linkProperties["overflow"] = "scrolled"
		}
	}
	return linkProperties
}
