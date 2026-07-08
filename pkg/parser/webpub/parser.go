package webpub

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/parser/pdf"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Parses any Readium Web Publication package or manifest, e.g. WebPub, Audiobook, DiViNa, LCPDF...
type WebPubParser struct {
	client *http.Client
}

func NewParser(client *http.Client) WebPubParser {
	return WebPubParser{
		client: client,
	}
}

// Parse implements PublicationParser
func (p WebPubParser) Parse(ctx context.Context, a asset.PublicationAsset, f fetcher.Fetcher) (*pub.Builder, error) {
	mediaType := a.MediaType(ctx)
	if !isMediatypeReadiumWebPubProfile(mediaType) {
		return nil, nil
	}

	isPackage := !mediaType.IsRwpm()

	lFetcher := f
	var manifestJSON map[string]interface{}
	if isPackage {
		res := lFetcher.Get(ctx, manifest.Link{Href: manifest.MustNewHREFFromString("manifest.json", false)})
		mjr, rerr := fetcher.ReadResourceAsJSON(ctx, res)
		res.Close()
		if rerr != nil {
			return nil, errors.Wrap(rerr, "failed reading manifest.json from package")
		}
		manifestJSON = mjr
	} else {
		// For a single manifest file, reads the first (and only) file in the fetcher.
		links, err := lFetcher.Links(ctx)
		if err != nil {
			return nil, err
		}
		if len(links) == 0 {
			return nil, errors.New("links is empty")
		}
		res := lFetcher.Get(ctx, links[0])
		mj, rerr := fetcher.ReadResourceAsJSON(ctx, res)
		res.Close()
		if rerr != nil {
			return nil, errors.Wrap(rerr, "failed reading manifest")
		}
		manifestJSON = mj
	}

	m, err := manifest.ManifestFromJSON(manifestJSON, isPackage)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing RWPM Manifest")
	}

	if err := checkProfileRequirements(mediaType, m); err != nil {
		return nil, err
	}

	if !isPackage {
		// For a bare manifest, we discard the [fetcher] provided by the Streamer, because it was
		// only used to read the manifest file. Resources are fetched relative to the manifest's
		// own location instead, whether it's on a local file system or on a remote server.
		relativeAsset, ok := a.(asset.RelativePublicationAsset)
		if !ok {
			return nil, errors.New("asset does not provide access to resources relative to the manifest")
		}

		// The publication is rooted at the topmost directory reached by the manifest's
		// HREFs, e.g. one level above the manifest for `../audio/track.mp3`.
		maxUp := 0
		m.TransformHREFs(func(h manifest.HREF) manifest.HREF {
			if up := parentEscapeDepth(h); up > maxUp {
				maxUp = up
			}
			return h
		})

		location := relativeAsset.Location()
		rootRef := "./"
		if maxUp > 0 {
			rootRef = strings.Repeat("../", maxUp)
		}
		root := location.Resolve(url.MustURLFromString(rootRef))

		// HREFs reaching outside the manifest's directory are normalized to the publication
		// root, like the resources of a package: `../audio/track.mp3` referenced from
		// `manifest/manifest.json` becomes `audio/track.mp3`, and a `cover.jpg` next to the
		// manifest becomes `manifest/cover.jpg`. This keeps the manifest and the fetcher
		// consistent when the publication is served under a single base URL.
		if maxUp > 0 {
			dir := location.Resolve(url.MustURLFromString("./"))
			m.TransformHREFs(func(h manifest.HREF) manifest.HREF {
				if h.IsTemplated() {
					return h
				}
				u := h.Resolve(nil, nil)
				if u == nil {
					return h
				}
				// Fragment-only and query-only HREFs (e.g. a `#part1` TOC entry) point
				// within the manifest itself and must not become path links.
				if u.Path() == "" {
					return h
				}
				rel := root.Relativize(dir.Resolve(u))
				if _, ok := rel.(url.AbsoluteURL); ok {
					// The HREF points outside the publication root: an absolute URL
					// (e.g. a resource hosted elsewhere) or a root-absolute path.
					// Leave it untouched rather than storing a mangled absolute form;
					// absolute HTTP(S) HREFs are fetched as-is and other HREFs resolve
					// against the publication root by the relative fetcher.
					return h
				}
				return manifest.NewHREF(rel)
			})
		}

		relativeFetcher, err := relativeAsset.CreateRelativeFetcher(ctx, root)
		if err != nil {
			return nil, errors.Wrap(err, "failed creating a fetcher for the manifest's resources")
		}
		lFetcher = &manifestResourceFetcher{
			relative: relativeFetcher,
			client:   p.client,
		}
	}

	serviceFactories := make(map[pub.ServiceName]pub.ServiceFactory)

	switch {
	case m.ConformsTo(manifest.ProfilePDF):
		// The positions service only supports a single PDF in the reading order.
		if len(m.ReadingOrder) == 1 {
			serviceFactories[pub.PositionsService_Name] = pdf.PositionsServiceFactory()
		}
	case m.ConformsTo(manifest.ProfileDivina):
		serviceFactories[pub.PositionsService_Name] = pub.PerResourcePositionsServiceFactory(mediatype.MustNewOfString("image/*"))
	case m.ConformsTo(manifest.ProfileEPUB):
		serviceFactories[pub.PositionsService_Name] = epub.PositionsServiceFactory(nil)
	}

	// Add guided navigation service for WebPubs with HTML contents. It serves SMIL
	// media overlays when reading order items have them, and converts the HTML
	// resources themselves otherwise. It replaces the content service:
	// serviceFactories[pub.ContentService_Name] = pub.DefaultContentServiceFactory([]iterator.ResourceContentIteratorFactory{
	// 	iterator.HTMLFactory(),
	// })
	for _, link := range m.ReadingOrder {
		if link.MediaType != nil && link.MediaType.IsHTML() {
			serviceFactories[pub.GuidedNavigationService_Name] = epub.MediaOverlayFactory()
			break
		}
	}

	var servicesBuilder *pub.ServicesBuilder
	if len(serviceFactories) > 0 {
		servicesBuilder = pub.NewServicesBuilder(serviceFactories)
	}
	return pub.NewBuilder(*m, lFetcher, servicesBuilder), nil
}

// Checks that the manifest conforms to the profiles implied by the asset's media type.
// The requirements are based on the media type rather than `metadata.conformsTo`, which a bare
// manifest is not required to declare: [manifest.Manifest.ConformsTo] also infers conformance
// from the reading order.
func checkProfileRequirements(mt mediatype.MediaType, m *manifest.Manifest) error {
	// Checks the requirements from the LCPDF specification.
	// https://readium.org/lcp-specs/notes/lcp-for-pdf.html
	if mt.Equal(&mediatype.LCPProtectedPDF) && (len(m.ReadingOrder) == 0 || !m.ReadingOrder.AllMatchMediaType(&mediatype.PDF)) {
		return errors.New("publication does not conform to the LCPDF specification")
	}

	if mt.Matches(&mediatype.ReadiumAudiobook, &mediatype.ReadiumAudiobookManifest, &mediatype.LCPProtectedAudiobook) &&
		!m.ConformsTo(manifest.ProfileAudiobook) {
		return errors.New("publication does not conform to the Audiobook profile")
	}

	if mt.Matches(&mediatype.ReadiumDivina, &mediatype.ReadiumDivinaManifest) &&
		!m.ConformsTo(manifest.ProfileDivina) {
		return errors.New("publication does not conform to the Divina profile")
	}

	return nil
}

// Returns how many levels above the manifest's directory the HREF reaches,
// e.g. 1 for `../audio/track.mp3` and 0 for `audio/track.mp3`.
func parentEscapeDepth(h manifest.HREF) int {
	if h.IsTemplated() {
		return 0
	}
	u := h.Resolve(nil, nil)
	if u == nil {
		return 0
	}
	if _, ok := u.(url.AbsoluteURL); ok {
		return 0
	}

	p := path.Clean(u.Path())
	depth := 0
	for p == ".." || strings.HasPrefix(p, "../") {
		depth++
		p = strings.TrimPrefix(strings.TrimPrefix(p, ".."), "/")
	}
	return depth
}

func isMediatypeReadiumWebPubProfile(mt mediatype.MediaType) bool {
	return mt.Matches(
		&mediatype.ReadiumWebpub, &mediatype.ReadiumWebpubManifest,
		&mediatype.ReadiumAudiobook, &mediatype.ReadiumAudiobookManifest, &mediatype.LCPProtectedAudiobook,
		&mediatype.ReadiumDivina, &mediatype.ReadiumDivinaManifest, &mediatype.LCPProtectedPDF,
	)
}
