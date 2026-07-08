package epub

import (
	"context"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
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
func (p Parser) Parse(ctx context.Context, asset asset.PublicationAsset, f fetcher.Fetcher) (*pub.Builder, error) {
	fallbackTitle := asset.Name()

	if !asset.MediaType(ctx).Equal(&mediatype.EPUB) {
		return nil, nil
	}

	opfPath, err := GetRootFilePath(ctx, f)
	if err != nil {
		return nil, err
	}

	// Detect DRM

	opfXmlDocument, errx := fetcher.ReadResourceAsXML(ctx, f.Get(ctx, manifest.Link{Href: manifest.NewHREF(opfPath)}))
	if errx != nil {
		return nil, errx
	}

	packageDocument, err := ParsePackageDocument(opfXmlDocument, opfPath)
	if err != nil {
		return nil, errors.Wrap(err, "invalid OPF file")
	}

	// Detect the container-level DRM scheme. This is done unconditionally,
	// not gated on the presence of META-INF/encryption.xml, because schemes
	// like Adobe ADEPT, Barnes & Noble, Apple FairPlay and Kobo announce
	// themselves through other well-known files (rights.xml / sinf.xml).
	// TODO: surface the publication-level scheme on the manifest itself so
	// consumers can detect protection even when encryption.xml is absent.
	scheme, encryptionDoc, err := protection.IdentifyEPUBProtection(ctx, f)
	if err != nil {
		return nil, errors.Wrap(err, "failed identifying EPUB protection scheme")
	}

	encryptionData, err := parseEncryptionData(ctx, f, scheme.URI(), encryptionDoc)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing encryption data")
	}

	manifest := PublicationFactory{
		FallbackTitle:   fallbackTitle,
		PackageDocument: *packageDocument,
		NavigationData:  parseNavigationData(ctx, *packageDocument, f),
		EncryptionData:  encryptionData,
		DisplayOptions:  parseDisplayOptions(ctx, f),
	}.Create()

	ffetcher := f
	if manifest.Metadata.Identifier != "" {
		ffetcher = fetcher.NewTransformingFetcher(f, NewDeobfuscator(manifest.Metadata.Identifier).Transform)
	}

	builder := pub.NewServicesBuilder(map[pub.ServiceName]pub.ServiceFactory{
		pub.PositionsService_Name: PositionsServiceFactory(p.reflowablePositionsStrategy),
		// The guided navigation service replaces the content service
		// pub.ContentService_Name: pub.DefaultContentServiceFactory([]iterator.ResourceContentIteratorFactory{
		// 	iterator.HTMLFactory(),
		// }),
		pub.GuidedNavigationService_Name: MediaOverlayFactory(),
	})
	return pub.NewBuilder(manifest, ffetcher, builder), nil
}

// parseEncryptionData parses META-INF/encryption.xml when present and stamps
// each entry with the supplied DRM scheme URI (typically obtained by calling
// [protection.IdentifyEPUBProtection] at a higher level). A missing
// encryption.xml is normal and returns (nil, nil).
//
// When doc is non-nil it is used directly, avoiding a redundant read+parse of
// encryption.xml — [protection.IdentifyEPUBProtection] returns the document it
// already parsed for exactly this purpose.
func parseEncryptionData(ctx context.Context, f fetcher.Fetcher, scheme string, doc *xmlquery.Node) (map[string]manifest.Encryption, error) {
	if doc == nil {
		var rerr *fetcher.ResourceError
		doc, rerr = fetcher.ReadResourceAsXML(ctx, f.Get(ctx, manifest.Link{Href: manifest.MustNewHREFFromString("META-INF/encryption.xml", false)}))
		if rerr != nil {
			return nil, nil
		}
	}
	return ParseEncryption(doc, scheme), nil
}

func parseNavigationData(ctx context.Context, packageDocument PackageDocument, f fetcher.Fetcher) (ret map[string]manifest.LinkList) {
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
				if mediatype.NCX.Contains(v.MediaType) {
					ncxItem = &v
					break
				}
			}
		}
		if ncxItem == nil {
			return
		}
		n, nerr := fetcher.ReadResourceAsXML(ctx, f.Get(ctx, manifest.Link{Href: manifest.NewHREF(ncxItem.Href)}))
		if nerr != nil {
			return
		}
		ret = ParseNCX(n, ncxItem.Href)
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
		n, errx := fetcher.ReadResourceAsXML(ctx, f.Get(ctx, manifest.Link{Href: manifest.NewHREF(navItem.Href)}))
		if errx != nil {
			return
		}
		ret = ParseNavDoc(n, navItem.Href)
	}
	return
}

func parseDisplayOptions(ctx context.Context, f fetcher.Fetcher) (ret map[string]string) {
	ret = make(map[string]string)
	displayOptionsXml, err := fetcher.ReadResourceAsXML(ctx, f.Get(ctx, manifest.Link{Href: manifest.MustNewHREFFromString("META-INF/com.apple.ibooks.display-options.xml", false)}))
	if err != nil {
		displayOptionsXml, err = fetcher.ReadResourceAsXML(ctx, f.Get(ctx, manifest.Link{Href: manifest.MustNewHREFFromString("META-INF/com.kobobooks.display-options.xml", false)}))
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
