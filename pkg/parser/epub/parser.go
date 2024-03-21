package epub

import (
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/content/iterator"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util"
)

type Parser struct {
	reflowablePositionsStrategy ReflowableStrategy
}

func NewParser(strategy ReflowableStrategy) Parser {
	if strategy == nil {
		strategy = RecommendedReflowableStrategy
	}
	return Parser{
		reflowablePositionsStrategy: strategy,
	}
}

// Parse implements PublicationParser
func (p Parser) Parse(asset asset.PublicationAsset, f fetcher.Fetcher) (*pub.Builder, error) {
	fallbackTitle := asset.Name()

	if !asset.MediaType().Equal(&mediatype.EPUB) {
		return nil, nil
	}

	opfPath, err := GetRootFilePath(f)
	if err != nil {
		return nil, err
	}
	if opfPath[0] != '/' {
		opfPath = "/" + opfPath
	}

	opfXmlDocument, errx := f.Get(manifest.Link{Href: opfPath}).ReadAsXML(map[string]string{
		NamespaceOPF:                         "opf",
		NamespaceDC:                          "dc",
		VocabularyDCTerms:                    "dcterms",
		"http://www.idpf.org/2013/rendition": "rendition",
	})
	if errx != nil {
		return nil, errx
	}

	packageDocument, err := ParsePackageDocument(opfXmlDocument, opfPath)
	if err != nil {
		return nil, errors.Wrap(err, "invalid OPF file")
	}

	manifest := PublicationFactory{
		FallbackTitle:   fallbackTitle,
		PackageDocument: *packageDocument,
		NavigationData:  parseNavigationData(*packageDocument, f),
		EncryptionData:  parseEncryptionData(f),
		DisplayOptions:  parseDisplayOptions(f),
	}.Create()

	ffetcher := f
	if manifest.Metadata.Identifier != "" {
		ffetcher = fetcher.NewTransformingFetcher(f, NewDeobfuscator(manifest.Metadata.Identifier).Transform)
	}

	builder := pub.NewServicesBuilder(map[string]pub.ServiceFactory{
		pub.PositionsService_Name: PositionsServiceFactory(p.reflowablePositionsStrategy),
		pub.ContentService_Name: pub.DefaultContentServiceFactory([]iterator.ResourceContentIteratorFactory{
			iterator.HTMLFactory(),
		}),
	})
	return pub.NewBuilder(manifest, ffetcher, builder), nil
}

func parseEncryptionData(fetcher fetcher.Fetcher) (ret map[string]manifest.Encryption) {
	n, err := fetcher.Get(manifest.Link{Href: "/META-INF/encryption.xml"}).ReadAsXML(map[string]string{
		NamespaceENC:  "enc",
		NamespaceSIG:  "ds",
		NamespaceCOMP: "comp",
	})
	if err != nil {
		return
	}
	return ParseEncryption(n)
}

func parseNavigationData(packageDocument PackageDocument, fetcher fetcher.Fetcher) (ret map[string]manifest.LinkList) {
	ret = make(map[string]manifest.LinkList)
	if packageDocument.EPUBVersion < 3.0 {
		var ncxItem *Item
		if packageDocument.Spine.TOC != "" {
			for _, v := range packageDocument.Manifest {
				if v.ID == packageDocument.Spine.TOC {
					ncxItem = &v
					break
				}
			}
		} else {
			for _, v := range packageDocument.Manifest {
				if mediatype.NCX.ContainsFromString(v.MediaType) {
					ncxItem = &v
					break
				}
			}
		}
		if ncxItem == nil {
			return
		}
		ncxPath, err := util.NewHREF(ncxItem.Href, packageDocument.Path).String()
		if err != nil {
			return
		}
		n, nerr := fetcher.Get(manifest.Link{Href: ncxPath}).ReadAsXML(map[string]string{
			NamespaceNCX: "ncx",
		})
		if nerr != nil {
			return
		}
		ret = ParseNCX(n, ncxPath)
	} else {
		var navItem *Item
		for _, v := range packageDocument.Manifest {
			for _, st := range v.Properties {
				if st == VocabularyItem+"nav" {
					navItem = &v
					break
				}
			}
			if navItem != nil {
				break
			}
		}
		if navItem == nil {
			return
		}
		navPath, err := util.NewHREF(navItem.Href, packageDocument.Path).String()
		if err != nil {
			return
		}
		n, errx := fetcher.Get(manifest.Link{Href: navPath}).ReadAsXML(map[string]string{
			NamespaceXHTML: "html",
			NamespaceOPS:   "epub",
		})
		if errx != nil {
			return
		}
		ret = ParseNavDoc(n, navPath)
	}
	return
}

func parseDisplayOptions(fetcher fetcher.Fetcher) (ret map[string]string) {
	ret = make(map[string]string)
	displayOptionsXml, err := fetcher.Get(manifest.Link{Href: "/META-INF/com.apple.ibooks.display-options.xml"}).ReadAsXML(nil)
	if err != nil {
		displayOptionsXml, err = fetcher.Get(manifest.Link{Href: "/META-INF/com.kobobooks.display-options.xml"}).ReadAsXML(nil)
		if err != nil {
			return
		}
	}

	if platform := displayOptionsXml.SelectElement("//platform"); platform != nil {
		for _, option := range platform.SelectElements("option") {
			optName := option.SelectAttr("name")
			optValue := option.InnerText()
			if optName != "" && optValue != "" {
				ret[optName] = optValue
			}
		}
	}
	return
}
