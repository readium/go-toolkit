package epub

import (
	"maps"
	"slices"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
)

type PublicationFactory struct {
	FallbackTitle   string
	PackageDocument PackageDocument
	NavigationData  map[string]manifest.LinkList
	// EncryptionData is the encryption of the resources, by their
	// [ResourceID].
	EncryptionData map[string]manifest.Encryption
	DisplayOptions map[string]string

	itemById       map[string]Item
	pubMetadata    PubMetadataAdapter
	itemrefByIdref map[string]ItemRef
	itemMetadata   map[string]LinkMetadataAdapter
	coverID        string
	// alternatesLeft is what remains of the budget of alternates of the
	// publication: see maxAlternates.
	alternatesLeft *int
}

func (f PublicationFactory) Create() manifest.Manifest {
	alternatesLeft := maxAlternates(len(f.PackageDocument.Manifest))
	f.alternatesLeft = &alternatesLeft
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
	f.coverID = f.pubMetadata.Cover()
	f.itemMetadata = make(map[string]LinkMetadataAdapter, len(f.PackageDocument.metadata.refine))
	for k, v := range f.PackageDocument.metadata.refine {
		f.itemMetadata[k] = LinkMetadataAdapter{
			epubVersion: epubVersion,
			items:       v,
		}
	}
	f.itemById = make(map[string]Item, len(mani))
	for _, item := range mani {
		f.itemById[item.ID] = item
	}
	f.itemrefByIdref = make(map[string]ItemRef, len(spine.itemrefs))
	linearCount := 0
	for _, v := range spine.itemrefs {
		f.itemrefByIdref[v.idref] = v
		if v.linear {
			linearCount++
		}
	}

	// Compute Metadata
	metadata := f.pubMetadata.Metadata()
	metadata.OtherMetadata[NamespaceOPF+"#version"] = f.PackageDocument.EPUBVersionString
	metadataLinks := make(manifest.LinkList, 0, len(links))
	for _, link := range links {
		metadataLinks = append(metadataLinks, mapEPUBLink(link))
	}

	// Compute Links
	readingOrderIDs := make(map[string]struct{}, linearCount)
	readingOrder := make(manifest.LinkList, 0, linearCount)
	for _, v := range spine.itemrefs {
		if !v.linear {
			continue
		}
		readingOrderIDs[v.idref] = struct{}{}
		item, ok := f.itemById[v.idref]
		if ok {
			readingOrder = append(readingOrder, f.computeLink(item, nil))
		}
	}
	// Count first to size the link slice without retaining a second copy of the
	// manifest items. Membership checks must not scan the whole spine per item.
	resourceCount := 0
	for _, item := range mani {
		if _, linear := readingOrderIDs[item.ID]; !linear {
			resourceCount++
		}
	}
	resources := make(manifest.LinkList, 0, resourceCount)
	for _, item := range mani {
		if _, linear := readingOrderIDs[item.ID]; !linear {
			resources = append(resources, f.computeLink(item, nil))
		}
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
	var properties map[string]any
	var rels []string
	manifestRels, contains, others := parseItemProperties(item.Properties)
	for _, v := range manifestRels {
		rels = extensions.AddToSet(rels, v)
	}
	if len(contains) > 0 || len(others) > 0 {
		properties = make(map[string]any, len(others)+1)
	}
	if len(contains) > 0 {
		properties["contains"] = contains
	}
	for _, v := range others {
		properties[v] = true
	}
	if itemref != nil {
		itemrefProperties := parseItemrefProperties(itemref.properties)
		if properties == nil {
			properties = itemrefProperties
		} else {
			maps.Copy(properties, itemrefProperties)
		}
	}

	if f.coverID == item.ID {
		rels = extensions.AddToSet(rels, "cover")
	}

	if len(f.EncryptionData) > 0 {
		key, _ := ResourceID(item.Href)
		if edat, ok := f.EncryptionData[key]; ok {
			if properties == nil {
				properties = make(map[string]any, 1)
			}
			properties["encrypted"] = edat.ToMap() // ToMap makes it JSON-like
		}
	}

	return rels, manifest.Properties(properties)
}

// maxAlternateDepth bounds the nesting of the alternates of a link: each
// item may have both a fallback and a media overlay, which a crafted package
// could chain into exponentially many links.
const maxAlternateDepth = 8

// maxAlternates returns the budget of alternates of a publication with the
// given number of items: each link expands its own alternates, which many
// links may share, so the depth limit alone doesn't bound their total. Items
// rarely have more than a fallback and a media overlay.
func maxAlternates(items int) int {
	return max(1000, 4*items)
}

// computeAlternates returns the alternates of an item: its fallback and its
// media overlay, with their own alternates. chain is the active path of
// items whose alternates are being computed, which both kinds of reference
// follow: an item already on it would make a cycle.
func (f PublicationFactory) computeAlternates(item Item, chain []string) (ret manifest.LinkList) {
	if len(chain) >= maxAlternateDepth {
		return nil
	}
	follow := func(id string) {
		if id == "" || extensions.Contains(chain, id) {
			return
		}
		alternate, ok := f.itemById[id]
		if !ok {
			return
		}
		if f.alternatesLeft != nil {
			if *f.alternatesLeft <= 0 {
				return
			}
			*f.alternatesLeft--
		}
		ret = append(ret, f.computeLink(alternate, append(slices.Clone(chain), id)))
	}
	follow(item.fallback)
	follow(item.mediaOverlay)
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

func parseItemrefProperties(properties []string) map[string]any {
	var page string
	for _, property := range properties {
		switch property {
		// Page
		case VocabularyRendition + "page-spread-center":
			page = "center"
		case VocabularyRendition + "page-spread-left":
			fallthrough
		case VocabularyItemref + "page-spread-left":
			page = "left"
		case VocabularyRendition + "page-spread-right":
			fallthrough
		case VocabularyItemref + "page-spread-right":
			page = "right"
		}
	}
	if page == "" {
		return nil
	}
	return map[string]any{"page": page}
}
