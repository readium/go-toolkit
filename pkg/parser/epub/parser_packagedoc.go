package epub

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/readium/xmlquery"
)

type PackageDocument struct {
	Path               url.URL
	EPUBVersion        float64
	EPUBVersionString  string
	uniqueIdentifierID string
	metadata           EPUBMetadata
	Manifest           []Item
	Spine              Spine
}

func ParsePackageDocument(document *xmlquery.Node, filePath url.URL) (*PackageDocument, error) {
	pkg := document.SelectElement("/" + NSSelect(NamespaceOPF, "package"))
	if pkg == nil {
		return nil, errors.New("package root element not found")
	}
	packagePrefixes := parsePrefixes(pkg.SelectAttr("prefix"))
	prefixMap := make(map[string]string)
	for k, v := range PackageReservedPrefixes {
		prefixMap[k] = v
	}
	for k, v := range packagePrefixes {
		prefixMap[k] = v
	}

	// Version
	epubVersionString := pkg.SelectAttr("version")
	if epubVersionString == "" {
		epubVersionString = "1.2"
	}
	epubVersion, err := strconv.ParseFloat(epubVersionString, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing package version")
	}

	metadata := NewMetadataParser(epubVersion, prefixMap).Parse(document, filePath)
	if metadata == nil {
		return nil, errors.New("failed parsing package metadata")
	}
	manifestElement := pkg.SelectElement("/" + NSSelect(NamespaceOPF, "manifest"))
	if manifestElement == nil {
		return nil, errors.New("package manifest not found")
	}
	spineElement := pkg.SelectElement("/" + NSSelect(NamespaceOPF, "spine"))
	if spineElement == nil {
		return nil, errors.New("package spine not found")
	}

	mels := manifestElement.SelectElements("/" + NSSelect(NamespaceOPF, "item"))
	manifest := make([]Item, 0, len(mels))
	for _, mel := range mels {
		item := ParseItem(mel, filePath, prefixMap)
		if item == nil {
			// return nil, errors.New("failed parsing package manifest item at index " + strconv.Itoa(i))
			continue
		}
		manifest = append(manifest, *item)
	}

	return &PackageDocument{
		Path:               filePath,
		EPUBVersion:        epubVersion,
		EPUBVersionString:  epubVersionString,
		uniqueIdentifierID: pkg.SelectAttr("unique-identifier"),
		metadata:           *metadata,
		Manifest:           manifest,
		Spine:              ParseSpine(spineElement, prefixMap, epubVersion),
	}, nil

}

type Item struct {
	Href         url.URL
	ID           string
	fallback     string
	mediaOverlay string
	MediaType    *mediatype.MediaType
	Properties   []string
}

func ParseItem(element *xmlquery.Node, filePath url.URL, prefixMap map[string]string) *Item {
	rawHref := element.SelectAttr("href")
	if rawHref == "" {
		return nil
	}
	u, err := url.FromEPUBHref(rawHref)
	if err != nil {
		return nil
	}
	u = filePath.Resolve(u)
	item := &Item{
		Href:         u,
		ID:           element.SelectAttr("id"),
		fallback:     element.SelectAttr("fallback"),
		mediaOverlay: element.SelectAttr("media-overlay"),
		MediaType:    mediatype.MaybeNewOfString(element.SelectAttr("media-type")),
	}
	pp := parseProperties(element.SelectAttr("properties"))
	if len(pp) > 0 {
		item.Properties = make([]string, 0, len(pp))
		for _, prop := range pp {
			if prop == "" {
				continue
			}
			item.Properties = append(item.Properties, resolveProperty(prop, prefixMap, DefaultVocabItem))
		}
	}
	return item
}

type Spine struct {
	itemrefs  []ItemRef
	direction manifest.ReadingProgression
	TOC       string
}

func ParseSpine(element *xmlquery.Node, prefixMap map[string]string, epubVersion float64) Spine {
	selectedElements := element.SelectElements(
		"/" + NSSelect(NamespaceOPF, "itemref"),
	)
	itemrefs := make([]ItemRef, 0, len(selectedElements))
	for _, itemref := range selectedElements {
		itemref := ParseItemRef(itemref, prefixMap)
		if itemref == nil {
			continue
		}
		itemrefs = append(itemrefs, *itemref)
	}

	pageProgressionDiretion := manifest.Auto
	switch element.SelectAttr("page-progression-direction") {
	case "ltr":
		pageProgressionDiretion = manifest.LTR
	case "rtl":
		pageProgressionDiretion = manifest.RTL
	}

	ncx := ""
	if epubVersion < 3.0 {
		ncx = element.SelectAttr("toc")
	}

	return Spine{
		itemrefs:  itemrefs,
		direction: pageProgressionDiretion,
		TOC:       ncx,
	}
}

type ItemRef struct {
	idref      string
	linear     bool
	properties []string
}

func ParseItemRef(element *xmlquery.Node, prefixMap map[string]string) *ItemRef {
	idref := element.SelectAttr("idref")
	if idref == "" {
		return nil
	}

	pp := parseProperties(element.SelectAttr("properties"))
	properties := make([]string, 0, len(pp))
	for _, prop := range parseProperties(element.SelectAttr("properties")) {
		if prop == "" {
			continue
		}
		properties = append(properties, resolveProperty(prop, prefixMap, DefaultVocabItemref))
	}

	return &ItemRef{
		idref:      idref,
		linear:     element.SelectAttr("linear") != "no",
		properties: properties,
	}
}
