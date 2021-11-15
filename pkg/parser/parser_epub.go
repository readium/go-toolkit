package parser

import (
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util"
)

type EPUBParser struct {
}

func (p EPUBParser) Parse(asset asset.PublicationAsset, f fetcher.Fetcher) (*pub.Builder, error) {
	fallbackTitle := asset.Name()

	if !asset.MediaType().Equal(&mediatype.EPUB) {
		return nil, nil
	}

	opfPath, err := epub.GetRootFilePath(f)
	if err != nil {
		return nil, err
	}
	if opfPath[0] != '/' {
		opfPath = "/" + opfPath
	}

	opfXmlDocument, errx := f.Get(manifest.Link{Href: opfPath}).ReadAsXML()
	if errx != nil {
		return nil, err
	}

	packageDocument, err := epub.ParsePackageDocument(opfXmlDocument, opfPath)
	if err != nil {
		return nil, errors.Wrap(err, "invalid OPF file")
	}

	manifest := epub.PublicationFactory{
		FallbackTitle:   fallbackTitle,
		PackageDocument: *packageDocument,
		NavigationData:  parseNavigationData(*packageDocument, f),
		EncryptionData:  parseEncryptionData(f),
		DisplayOptions:  parseDisplayOptions(f),
	}.Create()

	ffetcher := f
	if manifest.Metadata.Identifier != "" {
		ffetcher = f // TODO TransformingFetcher(fetcher, EpubDeobfuscator(it)::transform)
	}

	return pub.NewBuilder(manifest, ffetcher), nil // TODO services!
}

func parseEncryptionData(fetcher fetcher.Fetcher) (ret map[string]manifest.Encryption) {
	n, err := fetcher.Get(manifest.Link{Href: "/META-INF/encryption.xml"}).ReadAsXML()
	if err != nil {
		return
	}
	return epub.ParseEncryption(n)
}

func parseNavigationData(packageDocument epub.PackageDocument, fetcher fetcher.Fetcher) (ret map[string][]manifest.Link) {
	ret = make(map[string][]manifest.Link)
	if packageDocument.EPUBVersion < 3.0 {
		var ncxItem *epub.Item
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
		n, err := fetcher.Get(manifest.Link{Href: ncxPath}).ReadAsXML()
		if err != nil {
			return
		}
		ret = epub.ParseNCX(n, ncxPath)
	} else {
		var navItem *epub.Item
		for _, v := range packageDocument.Manifest {
			for _, st := range v.Properties {
				if st == epub.VOCABULARY_ITEM+"nav" {
					navItem = &v
					break
				}
			}
		}
		if navItem == nil {
			return
		}
		navPath, err := util.NewHREF(navItem.Href, packageDocument.Path).String()
		if err != nil {
			return
		}
		n, err := fetcher.Get(manifest.Link{Href: navPath}).ReadAsXML()
		if err != nil {
			return
		}
		ret = epub.ParseNavDoc(n, navPath)
	}
	return
}

func parseDisplayOptions(fetcher fetcher.Fetcher) (ret map[string]string) {
	ret = make(map[string]string)
	displayOptionsXml, err := fetcher.Get(manifest.Link{Href: "/META-INF/com.apple.ibooks.display-options.xml"}).ReadAsXML()
	if err != nil {
		displayOptionsXml, err = fetcher.Get(manifest.Link{Href: "/META-INF/com.kobobooks.display-options.xml"}).ReadAsXML()
		if err != nil {
			return
		}
	}

	if platform := displayOptionsXml.SelectElement("platform"); platform != nil {
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
