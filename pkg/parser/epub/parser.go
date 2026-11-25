package epub

import (
	"context"
	"slices"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// MaxDocumentSize is the size limit of the XML documents the parser reads
// into memory: the container and package documents, navigation documents,
// display options and media overlays. They are parsed into trees, from
// packages which may inflate them a thousandfold, so the limit is far above
// the size of legitimate ones.
const MaxDocumentSize = extensions.MaxDocumentSize

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

	opfRes := f.Get(ctx, manifest.Link{Href: manifest.NewHREF(opfPath)})
	opfData, errx := fetcher.ReadResourceUpTo(ctx, opfRes, MaxDocumentSize)
	opfRes.Close()
	if errx != nil {
		return nil, errx
	}

	packageDocument, err := parsePackageDocumentData(opfData, opfPath)
	if err != nil {
		return nil, err
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

	navigationData, err := parseNavigationData(ctx, *packageDocument, f)
	if err != nil {
		return nil, errors.Wrap(err, "failed reading the navigation document")
	}
	displayOptions, err := parseDisplayOptions(ctx, f)
	if err != nil {
		return nil, errors.Wrap(err, "failed reading the display options")
	}

	manifest := PublicationFactory{
		FallbackTitle:   fallbackTitle,
		PackageDocument: *packageDocument,
		NavigationData:  navigationData,
		EncryptionData:  encryptionData,
		DisplayOptions:  displayOptions,
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
		pub.GuidedNavigationService_Name: pub.HTMLGuidedNavigationServiceFactory(),
		pub.MediaOverlayService_Name:     MediaOverlayFactory(),
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
		// The detection of the protection doesn't read encryption.xml when
		// there is an LCP license.
		var err error
		doc, err = readOptionalXML(ctx, f, manifest.MustNewHREFFromString(protection.PathEPUBEncryption, false))
		if err != nil || doc == nil {
			return nil, err
		}
	}
	// A resource declared several times takes all its declarations, as the
	// LCP remux does, rather than its last one: an earlier one may bound its
	// length. Its declarations are found by its ResourceID, whatever the
	// spelling of their hrefs.
	var data map[string]manifest.Encryption
	for _, entry := range ParseEncryptionEntries(doc, scheme) {
		if data == nil {
			data = make(map[string]manifest.Encryption)
		}
		key := entry.Href
		if u, err := url.URLFromString(entry.Href); err == nil {
			key, _ = ResourceID(u)
		}
		prev, ok := data[key]
		if !ok {
			data[key] = entry.Encryption
			continue
		}
		merged, err := protection.MergeEncryption(prev, entry.Encryption)
		if err != nil {
			return nil, errors.Errorf("%s is %v", entry.Href, err)
		}
		data[key] = merged
	}
	return data, nil
}

// readXML reads an XML document of the container, up to MaxDocumentSize.
func readXML(ctx context.Context, f fetcher.Fetcher, href manifest.HREF) (*xmlquery.Node, *fetcher.ResourceError) {
	res := f.Get(ctx, manifest.Link{Href: href})
	defer res.Close()
	return fetcher.ReadResourceAsXMLUpTo(ctx, res, MaxDocumentSize)
}

// readOptionalXML reads an optional XML document of the container: it
// returns nil, without error, when the document is missing or isn't valid
// XML, but a document larger than MaxDocumentSize is an error rather than
// silently ignored.
func readOptionalXML(ctx context.Context, f fetcher.Fetcher, href manifest.HREF) (*xmlquery.Node, error) {
	n, rerr := readXML(ctx, f, href)
	if rerr != nil {
		if errors.Is(rerr, fetcher.ErrResourceTooLarge) {
			return nil, rerr
		}
		return nil, nil
	}
	return n, nil
}

func parseNavigationData(ctx context.Context, packageDocument PackageDocument, f fetcher.Fetcher) (ret map[string]manifest.LinkList, err error) {
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
		n, nerr := readOptionalXML(ctx, f, manifest.NewHREF(ncxItem.Href))
		if nerr != nil || n == nil {
			return ret, nerr
		}
		ret = ParseNCX(n, ncxItem.Href)
	} else {
		var navItem *Item
		for _, v := range packageDocument.Manifest {
			if slices.Contains(v.Properties, VocabularyItem+"nav") {
				navItem = &v
			}
			if navItem != nil {
				break
			}
		}
		if navItem == nil {
			return
		}
		n, errx := readOptionalXML(ctx, f, manifest.NewHREF(navItem.Href))
		if errx != nil || n == nil {
			return ret, errx
		}
		ret = ParseNavDoc(n, navItem.Href)
	}
	return
}

func parseDisplayOptions(ctx context.Context, f fetcher.Fetcher) (ret map[string]string, err error) {
	ret = make(map[string]string)
	displayOptionsXml, err := readOptionalXML(ctx, f, manifest.MustNewHREFFromString("META-INF/com.apple.ibooks.display-options.xml", false))
	if err == nil && displayOptionsXml == nil {
		displayOptionsXml, err = readOptionalXML(ctx, f, manifest.MustNewHREFFromString("META-INF/com.kobobooks.display-options.xml", false))
	}
	if err != nil || displayOptionsXml == nil {
		return ret, err
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
