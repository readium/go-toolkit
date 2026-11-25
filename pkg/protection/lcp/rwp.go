package lcp

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"slices"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/internal/ziputil"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
)

const rwpManifestPath = "manifest.json"

// EncryptRWP encrypts the resources of a Readium Web Publication package
// (webpub, audiobook, divina...) or of an exploded one (a directory with a
// manifest.json), with the content key, and writes the protected package to w.
//
// The package is remuxed rather than rebuilt: entries which are not
// encrypted are copied byte for byte without being recompressed, and the
// manifest is patched in place, so that properties unknown to this toolkit
// are preserved. It comes first in the output, followed by the other entries
// in their original order. Encrypted resources are stored, after being
// deflated when [Options] says so.
//
// By default, every resource of the reading order and resources, with their
// alternates, is encrypted, except the cover: reading systems display it
// before the publication is unlocked, and the LCP profiles for audiobooks
// and PDF require it to stay in the clear. Files the manifest doesn't list
// are copied as they are, since encryption can only be declared on the
// manifest's links. For that reason, a resource selected for encryption
// which is not an entry of the package is an error: it may be an entry under
// another name, e.g. differing in case in an exploded package on a
// case-insensitive file system, which would be copied in the clear. So is a
// link of the reading order or resources which can't be classified, such as
// a URL template expanding to paths of the package.
//
// A package protected with a DRM, including LCP, is refused with
// [ErrAlreadyProtected]. The license is not part of the output: the License
// Server adds it when delivering the publication.
func EncryptRWP(ctx context.Context, a asset.PublicationAsset, key ContentKey, w io.Writer, opts Options) error {
	if err := key.Validate(); err != nil {
		return err
	}
	c, err := openRWP(ctx, a, opts)
	if err != nil {
		return err
	}
	defer c.f.Close()

	if c.licensed {
		return errors.Wrap(ErrAlreadyProtected, "package contains an LCP license")
	}
	if err := checkReservedEntries(c.entries); err != nil {
		return err
	}
	// A package also holding the file identifying another format, e.g. kept
	// from an EPUB it was converted from, would be taken for that format
	// when licensed or opened: see formatMarkers.
	for _, m := range formatMarkers {
		if m.format == rwpFormat {
			break
		}
		if slices.ContainsFunc(c.entries, func(e containerEntry) bool { return e.path == m.path }) {
			return errors.Errorf("lcp: the package holds %s, which identifies another format to its readers: remove it to protect the package as a Readium Web Publication", m.path)
		}
	}
	// The existing declarations are read from the raw manifest: a link the
	// parsed one drops, e.g. for lacking a media type, may declare its
	// resource encrypted, or obfuscated, and be patched otherwise.
	declared, outside, err := rwpDeclarations(c.rawManifest)
	if err != nil {
		return err
	}
	fonts := make(map[string]bool)
	for _, key := range slices.Sorted(maps.Keys(declared)) {
		if enc := declared[key]; !protection.IsFontObfuscation(enc.Algorithm) {
			return errors.Wrapf(ErrAlreadyProtected, "%s is encrypted with %s", key, enc.Algorithm)
		}
		fonts[key] = true
	}
	// So may links outside of the package, which are carried over: the
	// output would then hold resources it doesn't protect.
	for _, d := range outside {
		if !protection.IsFontObfuscation(d.enc.Algorithm) {
			return errors.Wrapf(ErrAlreadyProtected, "%s is encrypted with %s", d.href, d.enc.Algorithm)
		}
	}
	if err := checkFonts(c.manifest, fonts); err != nil {
		return err
	}

	selection := opts.Encrypt
	if selection == nil {
		selection = defaultRWPSelection(c.manifest)
	}
	// Encryption is declared on the links of the reading order and
	// resources, which are the only ones patched: they are the candidates.
	links := manifestLinks(manifest.Manifest{
		ReadingOrder: c.manifest.ReadingOrder,
		Resources:    c.manifest.Resources,
	})

	// Each resource is selected once, as the first link to it says: the
	// checks and the writing then rely on the same decisions.
	selected := make(map[string]bool, len(links)) // by resource key
	var keys []string                             // in manifest order
	decide := func(l manifest.Link) {
		if outsidePackage(l) {
			return
		}
		key := resourceKey(l.Href)
		if _, ok := selected[key]; !ok {
			selected[key] = key != rwpManifestPath && !fonts[key] && selection(links[key])
			keys = append(keys, key)
		}
	}
	walkLinks(c.manifest.ReadingOrder, decide)
	walkLinks(c.manifest.Resources, decide)
	for _, key := range keys {
		if selected[key] && isLicense(key) {
			return errLicensePathSelected(key)
		}
	}

	// The manifest comes first, so the encryption of each resource is planned
	// before any is written. The original length is all it needs.
	r := remuxer{ctx: ctx, f: c.f, key: key}
	planned := make(map[string]manifest.Encryption) // by resource key
	for _, e := range c.entries {
		if !selected[e.path] {
			continue
		}
		link := links[e.path]
		res := c.f.Get(ctx, e.link)
		length, rerr := res.Length(ctx)
		res.Close()
		if rerr != nil {
			return errors.Wrapf(rerr, "lcp: failed reading the length of %s", e.path)
		}
		enc := manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC, OriginalLength: length}
		if opts.compress(link) {
			enc.Compression = CompressionDeflate
		}
		planned[e.path] = enc
	}

	// Every selected resource must be an entry of the package. Otherwise, the
	// entry holding it under another name, e.g. differing in case on a
	// case-insensitive file system, would be copied in the clear, as files the
	// manifest doesn't list are.
	for _, key := range keys {
		if _, ok := planned[key]; selected[key] && !ok {
			return errors.Errorf("lcp: %s is listed in the manifest, but not found in the package", links[key].Href)
		}
	}
	doc, err := decodeDocument(c.rawManifest)
	if err != nil {
		return err
	}
	if err := checkRawLinks(doc, links); err != nil {
		return err
	}
	pins, err := checkSameFiles(ctx, c.f, c.entries, func(e containerEntry) (bool, bool) {
		_, ok := planned[e.path]
		return ok, true // the manifest is written in the clear, patched
	})
	if err != nil {
		return err
	}
	r.pins = pins

	patched, err := patchManifest(doc, func(key string, props map[string]any) map[string]any {
		if enc, ok := planned[key]; ok {
			if props == nil {
				props = make(map[string]any)
			}
			props["encrypted"] = enc.ToMap()
		}
		return props
	})
	if err != nil {
		return err
	}

	zw := ziputil.NewWriter(w)
	r.zw = zw
	if err := ziputil.WriteBytes(zw, rwpManifestPath, patched, zip.Deflate); err != nil {
		return err
	}
	for _, e := range c.entries {
		if e.path == rwpManifestPath || isLicense(e.path) {
			continue
		}
		link := c.link(e)
		plan, ok := planned[e.path]
		if !ok {
			if err := r.copy(e, link); err != nil {
				return err
			}
			continue
		}
		enc, err := r.encrypt(e, plan.Compression == CompressionDeflate, opts)
		if err != nil {
			return err
		}
		if enc.OriginalLength != plan.OriginalLength {
			return errors.Errorf("lcp: %s is %d bytes long, not %d as announced", e.path, enc.OriginalLength, plan.OriginalLength)
		}
	}
	return errors.Wrap(zw.Close(), "lcp: failed finalizing package")
}

// DecryptRWP decrypts the LCP-encrypted resources of a Readium Web
// Publication package with the content key, and writes the unprotected
// package to w.
//
// The package is remuxed rather than rebuilt: the other entries are copied
// byte for byte, and the manifest is patched in place to remove the LCP
// encryption properties. The LCP license is removed. Only
// [Options.ArchiveFactory] is used.
//
// A package which is not protected with LCP is refused with
// [ErrNotProtected].
func DecryptRWP(ctx context.Context, a asset.PublicationAsset, key ContentKey, w io.Writer, opts Options) error {
	if err := key.Validate(); err != nil {
		return err
	}
	c, err := openRWP(ctx, a, opts)
	if err != nil {
		return err
	}
	defer c.f.Close()

	// The declarations are read from the raw manifest, as InjectLicense does:
	// the parsed one drops e.g. the links without a media type, whose
	// resources would be copied encrypted, without their license.
	declared, outside, err := rwpDeclarations(c.rawManifest)
	if err != nil {
		return err
	}
	encrypted, err := lcpEncrypted(declared)
	if err != nil {
		return err
	}
	if !c.licensed && len(encrypted) == 0 {
		return ErrNotProtected
	}
	// Resources outside of the package can't be decrypted: keeping their
	// declarations without the license would deliver their ciphertext as if
	// it were in the clear.
	for _, d := range outside {
		if requiresLCPLicense(&d.enc) {
			return errOutsideLCP(d.href, d.link)
		}
	}
	if err := checkDeclaredEntries(encrypted, c.entries, func(path string) bool {
		return path == rwpManifestPath || isLicense(path)
	}); err != nil {
		return err
	}
	pins, err := checkEntryLicenseAliases(ctx, c.f, c.entries)
	if err != nil {
		return err
	}

	doc, err := decodeDocument(c.rawManifest)
	if err != nil {
		return err
	}
	patched, err := patchManifest(doc, func(key string, props map[string]any) map[string]any {
		if _, ok := encrypted[key]; ok && props != nil {
			delete(props, "encrypted")
			if len(props) == 0 {
				return nil
			}
		}
		return props
	})
	if err != nil {
		return err
	}

	zw := ziputil.NewWriter(w)
	r := remuxer{ctx: ctx, f: c.f, zw: zw, key: key, pins: pins}
	if err := ziputil.WriteBytes(zw, rwpManifestPath, patched, zip.Deflate); err != nil {
		return err
	}
	for _, e := range c.entries {
		if e.path == rwpManifestPath || isLicense(e.path) {
			continue
		}
		link := c.link(e)
		if enc, ok := encrypted[e.path]; ok {
			if err := r.decrypt(e, link, enc); err != nil {
				return err
			}
		} else if err := r.copy(e, link); err != nil {
			return err
		}
	}
	return errors.Wrap(zw.Close(), "lcp: failed finalizing package")
}

// rwpContainer gathers what the remuxing functions need to know about a
// Readium Web Publication package.
type rwpContainer struct {
	f           fetcher.Fetcher
	entries     []containerEntry
	rawManifest []byte
	manifest    manifest.Manifest
	links       map[string]manifest.Link
	licensed    bool
}

func openRWP(ctx context.Context, a asset.PublicationAsset, opts Options) (*rwpContainer, error) {
	f, err := opts.createFetcher(ctx, a)
	if err != nil {
		return nil, err
	}
	c, err := readRWP(ctx, f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return c, nil
}

// parseRWPManifest parses the manifest.json of a Readium Web Publication
// package.
func parseRWPManifest(raw []byte) (*manifest.Manifest, error) {
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil, errors.Wrap(err, "lcp: invalid manifest.json")
	}
	m, err := manifest.ManifestFromJSON(rawMap, true)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: invalid manifest.json")
	}
	return m, nil
}

func readRWP(ctx context.Context, f fetcher.Fetcher) (*rwpContainer, error) {
	raw, err := readEntry(ctx, f, rwpManifestPath, maxManifestSize)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("lcp: asset is not a Readium Web Publication package, manifest.json is missing")
	}
	m, err := parseRWPManifest(raw)
	if err != nil {
		return nil, err
	}

	license, err := readEntry(ctx, f, protection.PathRWPLCPLicense, maxLicenseSize)
	if err != nil {
		return nil, err
	}
	entries, err := containerEntries(ctx, f)
	if err != nil {
		return nil, err
	}
	return &rwpContainer{
		f:           f,
		entries:     entries,
		rawManifest: raw,
		manifest:    *m,
		links:       manifestLinks(*m),
		licensed:    license != nil,
	}, nil
}

// link returns the manifest link of an entry, or the fetcher's link for
// files missing from the manifest.
func (c *rwpContainer) link(e containerEntry) manifest.Link {
	if l, ok := c.links[e.path]; ok {
		return l
	}
	return e.link
}

// patchManifest rewrites the properties of the link objects of the reading
// order, resources and links (with their alternates and children) of a raw
// RWPM: every link which may declare the encryption of a resource.
// update receives the resource key of each link and its properties (nil when
// absent), and returns the new properties (nil to remove them).
//
// The manifest is processed as generic JSON, so that everything this
// toolkit's model doesn't know about is preserved. Numbers are kept as they
// were written, and insignificant whitespace is left out. A manifest
// exceeding the size limit of manifests, e.g. once encryption properties are
// added, is an error: readers, including this package's, would refuse it.
func patchManifest(doc map[string]any, update func(key string, props map[string]any) map[string]any) ([]byte, error) {
	walkRawDeclarations(doc, func(link map[string]any) {
		href, _ := link["href"].(string)
		templated, _ := link["templated"].(bool)
		h, err := manifest.ParseLinkHREF(href, templated)
		// Links outside of the package are left as they are, whatever the
		// resource of the package their path matches.
		if href == "" || err != nil || outsidePackage(manifest.Link{Href: h}) {
			return
		}
		props, _ := link["properties"].(map[string]any)
		if props = update(resourceKey(h), props); props == nil {
			delete(link, "properties")
		} else {
			link["properties"] = props
		}
	})

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, errors.Wrap(err, "lcp: failed serializing manifest.json")
	}
	patched := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if len(patched) > maxManifestSize {
		return nil, errors.Errorf("lcp: manifest.json would be larger than %s, the size limit of readers", extensions.FormatSize(maxManifestSize))
	}
	return patched, nil
}

// walkRawLinks calls fn on the link objects of the reading order and
// resources of a raw RWPM, with their alternates and children. Like the
// manifest parser, it takes the legacy "spine" for the reading order when
// there is no "readingOrder".
func walkRawLinks(doc map[string]any, fn func(link map[string]any)) {
	readingOrder, ok := doc["readingOrder"].([]any)
	if !ok {
		readingOrder, _ = doc["spine"].([]any)
	}
	walkRawList(readingOrder, fn)
	walkRawList(doc["resources"], fn)
}

// walkRawDeclarations calls fn on the link objects of a raw RWPM which may
// declare the encryption of a resource, as [walkDeclarations] does for a
// parsed one: those of [walkRawLinks], and of links.
func walkRawDeclarations(doc map[string]any, fn func(link map[string]any)) {
	walkRawLinks(doc, fn)
	walkRawList(doc["links"], fn)
}

// walkRawList calls fn on the link objects of a raw list of links, and
// recursively on their alternates and children.
func walkRawList(v any, fn func(link map[string]any)) {
	list, _ := v.([]any)
	for _, item := range list {
		if link, ok := item.(map[string]any); ok {
			fn(link)
			walkRawList(link["alternate"], fn)
			walkRawList(link["children"], fn)
		}
	}
}

// outsideDeclaration is the encryption a link of a raw manifest outside of
// the package declares.
type outsideDeclaration struct {
	href string
	link manifest.Link
	enc  manifest.Encryption
}

// checkRawLinks returns an error when a link of the reading order or
// resources of a raw RWPM points to a resource of the package which none of
// the classified links points to, or is a URL template expanding to paths
// of the package.
//
// The manifest parser drops the links it can't use, e.g. those without a
// media type, so that encryption is neither planned nor declared for their
// resources. Their entries would otherwise be copied in the clear as files
// the manifest doesn't list, while the manifest still lists them. The same
// goes for templates, which aren't classified: the resources they expand to
// depend on their parameters.
func checkRawLinks(doc map[string]any, classified map[string]manifest.Link) error {
	var err error
	walkRawLinks(doc, func(link map[string]any) {
		if err != nil {
			return
		}
		href, _ := link["href"].(string)
		templated, _ := link["templated"].(bool)
		h, herr := manifest.ParseLinkHREF(href, templated)
		if href == "" || herr != nil {
			err = errors.Errorf("lcp: invalid link in manifest.json: %q", href)
			return
		}
		l := manifest.Link{Href: h}
		if isLocalTemplate(l) {
			err = errors.Errorf("lcp: %s in manifest.json is a URL template expanding to paths of the package, whose resources can't be classified for encryption", href)
			return
		}
		if outsidePackage(l) {
			return
		}
		if _, ok := classified[resourceKey(h)]; !ok {
			err = errors.Errorf("lcp: %s in manifest.json can't be classified for encryption, e.g. because its link has no media type", href)
		}
	})
	return err
}
