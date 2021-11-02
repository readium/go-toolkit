package epub

import (
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
)

type PackageDocument struct {
	Path               string
	EPUBVersion        float64
	uniqueIdenfifierID string
	metadata           EPUBMetadata
	Manifest           []Item
	Spine              Spine
}

func ParsePackageDocument(document *xmlquery.Node, filePath string) (*PackageDocument, error) {
	packagePrefixes := parsePrefixes(document.SelectAttr("prefix"))
	prefixMap := make(map[string]string)
	for k, v := range PACKAGE_RESERVED_PREFIXES {
		prefixMap[k] = v
	}
	for k, v := range packagePrefixes {
		prefixMap[k] = v
	}

	// Version
	epubVersion := 1.2
	rv := document.SelectAttr("version")
	if rv != "" {
		ev, err := strconv.ParseFloat(rv, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing package version")
		}
		epubVersion = ev
	}

	metadata := NewMetadataParser(epubVersion, prefixMap).Parse(document, filePath)
	if metadata == nil {
		return nil, errors.New("failed parsing package metadata")
	}
	manifestElement := document.SelectElement("/manifest[namespace-uri()='" + NAMESPACE_OPF + "']")
	if manifestElement == nil {
		return nil, errors.New("package manifest not found")
	}
	spineElement := document.SelectElement("/spine[namespace-uri()='" + NAMESPACE_OPF + "']")
	if spineElement == nil {
		return nil, errors.New("package spine not found")
	}

	mels := manifestElement.SelectElements("/item[namespace-uri()='" + NAMESPACE_OPF + "']")
	manifest := make([]Item, 0, len(mels))
	for i, mel := range mels {
		item := ParseItem(mel, filePath, prefixMap)
		if item == nil {
			return nil, errors.New("failed parsing package manifest item " + strconv.Itoa(i))
		}
		manifest = append(manifest, *item)
	}

	return &PackageDocument{
		Path:               filePath,
		EPUBVersion:        epubVersion,
		uniqueIdenfifierID: document.SelectAttr("unique-identifier"),
		metadata:           *metadata,
		Manifest:           manifest,
		Spine:              ParseSpine(spineElement, prefixMap, epubVersion),
	}, nil

}

type Item struct {
	Href         string
	ID           string
	fallback     string
	mediaOverlay string
	MediaType    string
	Properties   []string
}

func ParseItem(element *xmlquery.Node, filePath string, prefixMap map[string]string) *Item {
	rawHref := element.SelectAttr("href")
	if rawHref == "" {
		return nil
	}
	href, err := util.NewHREF(rawHref, filePath).String()
	if err != nil {
		return nil
	}
	pp := parseProperties(element.SelectAttr("properties"))
	properties := make([]string, 0, len(pp))
	for _, prop := range parseProperties(element.SelectAttr("properties")) {
		if prop == "" {
			continue
		}
		properties = append(properties, resolveProperty(prop, prefixMap, ITEM))
	}
	return &Item{
		Href:         href,
		ID:           element.SelectAttr("id"),
		fallback:     element.SelectAttr("fallback"),
		mediaOverlay: element.SelectAttr("media-overlay"),
		MediaType:    element.SelectAttr("media-type"),
		Properties:   properties,
	}
}

type Spine struct {
	itemrefs  []ItemRef
	direction manifest.ReadingProgression
	TOC       string
}

func ParseSpine(element *xmlquery.Node, prefixMap map[string]string, epubVersion float64) Spine {
	itemrefs := make([]ItemRef, 0)
	for _, itemref := range element.SelectElements("itemref") {
		itemref := ParseItemRef(itemref, prefixMap)
		if itemref == nil {
			continue
		}
		itemrefs = append(itemrefs, *itemref)
	}

	pageProgressionDiretion := manifest.AUTO
	switch element.SelectAttr("page-progression-direction") {
	case "ltr":
		pageProgressionDiretion = manifest.LTR
	case "rtl":
		pageProgressionDiretion = manifest.RTL
	}

	ncx := ""
	if epubVersion > 3.0 {
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
		properties = append(properties, resolveProperty(prop, prefixMap, ITEMREF))
	}

	return &ItemRef{
		idref:      idref,
		linear:     element.SelectAttr("linear") != "no",
		properties: properties,
	}
}
