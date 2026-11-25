package lcp

import (
	"archive/zip"
	"context"
	"io"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/internal/ziputil"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	epubparser "github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// EncryptEPUB encrypts the resources of an EPUB with the content key, and
// writes the protected EPUB to w. This is what the lcpencrypt tool does
// before the License Server distributes the publication.
//
// The EPUB is remuxed rather than rebuilt: the entries which are not
// encrypted, including the package document, are copied byte for byte
// without being recompressed, and META-INF/encryption.xml gains an entry
// for each encrypted resource. Encrypted resources are stored, after being
// deflated when [Options] says so.
//
// By default, every publication resource is encrypted, except those reading
// systems need before the publication is unlocked: the cover, the navigation
// document, the NCX and the page map. Obfuscated fonts declared in
// encryption.xml are left as they are. The mimetype file, the package
// document and the META-INF directory are never encrypted, whatever
// [Options.Encrypt] says.
//
// An EPUB protected with a DRM, including LCP, or whose encryption.xml
// declares anything but font obfuscation, is refused with
// [ErrAlreadyProtected]. Font obfuscation declared for a resource which is
// not a font, according to its media type, is an error. The license is not
// part of the output: the License Server adds it when delivering the
// publication.
func EncryptEPUB(ctx context.Context, a asset.PublicationAsset, key ContentKey, w io.Writer, opts Options) error {
	if err := key.Validate(); err != nil {
		return err
	}
	c, err := openEPUB(ctx, a, opts, func(scheme protection.Scheme, encryption map[string]manifest.Encryption) error {
		switch scheme {
		case protection.NoDRM, protection.Generic:
		default:
			return errors.Wrapf(ErrAlreadyProtected, "EPUB is protected with %s", scheme)
		}
		// A "generic" encryption.xml may declare font obfuscation, which is
		// carried over, or an unknown encryption, which is not: its entries
		// could only be rewritten partially.
		for href, enc := range encryption {
			if !protection.IsFontObfuscation(enc.Algorithm) {
				return errors.Wrapf(ErrAlreadyProtected, "%s is encrypted with %s", href, enc.Algorithm)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	defer c.f.Close()

	// Resources declared in encryption.xml keep their obfuscation, which
	// only fonts may: see checkFonts. Those the package document doesn't
	// list are known by the media type of their extension.
	fonts := make(map[string]bool)
	for _, e := range c.entries {
		if _, declared := c.encryption[e.path]; !declared || c.isContainerFile(e) {
			continue
		}
		fonts[e.path] = true
		// Judged by the link of any rendition, or by the extension of
		// those no package document lists.
		if l := c.link(e); l.MediaType == nil || !l.MediaType.IsFont() {
			return notFontError(l)
		}
	}
	// Every link to an obfuscated resource, in any rendition, must present
	// it as a font: links reduced to one per resource would only tell what
	// one rendition says.
	for _, m := range append([]manifest.Manifest{c.manifest}, c.renditions...) {
		if err := checkFonts(m, fonts); err != nil {
			return err
		}
	}

	selection := opts.Encrypt
	if selection == nil {
		selection = defaultEPUBSelection
	}
	// plan returns whether an entry is encrypted, and whether it is written
	// at all: some container files are regenerated instead. Each entry is
	// planned once, so that the check of the names of a file and the writing
	// rely on the same decisions.
	type decision struct{ encrypt, written bool }
	decisions := make(map[string]decision, len(c.entries))
	for _, e := range c.entries {
		switch {
		case isLicense(e.path):
		case c.isContainerFile(e):
			decisions[e.path] = decision{written: e.path != protection.PathEPUBEncryption && e.path != "mimetype"}
		default:
			if _, ok := decisions[e.path]; !ok {
				decisions[e.path] = decision{encrypt: !fonts[e.path] && selection(c.link(e)), written: true}
			}
		}
	}
	plan := func(e containerEntry) (encrypt bool, written bool) {
		d := decisions[e.path]
		return d.encrypt, d.written
	}
	pins, err := checkSameFiles(ctx, c.f, c.entries, plan)
	if err != nil {
		return err
	}

	zw := ziputil.NewWriter(w)
	if err := ziputil.WriteMimetype(zw, mediatype.EPUB.String()); err != nil {
		return err
	}
	r := remuxer{ctx: ctx, f: c.f, zw: zw, key: key, pins: pins}

	var added []protection.EncryptionEntry
	for _, e := range c.entries {
		encrypt, written := plan(e)
		if !written {
			continue
		}
		link := c.link(e)
		if c.isContainerFile(e) {
			link = e.link
		}
		if !encrypt {
			if err := r.copy(e, link); err != nil {
				return err
			}
			continue
		}

		enc, err := r.encrypt(e, opts.compress(link), opts)
		if err != nil {
			return err
		}
		href, err := encodedPath(e.path)
		if err != nil {
			return err
		}
		added = append(added, protection.EncryptionEntry{Href: href, Encryption: enc})
	}

	if err := writeEncryptionXML(zw, append(c.existingEntries(), added...)); err != nil {
		return err
	}
	return errors.Wrap(zw.Close(), "lcp: failed finalizing EPUB")
}

// DecryptEPUB decrypts the LCP-encrypted resources of an EPUB with the
// content key, and writes the unprotected EPUB to w.
//
// The EPUB is remuxed rather than rebuilt: the other entries are copied byte
// for byte. The LCP license is removed, as well as the LCP entries of
// META-INF/encryption.xml; other entries, such as obfuscated fonts, are
// kept. Only [Options.ArchiveFactory] is used.
//
// An EPUB which is not protected with LCP is refused with [ErrNotProtected].
func DecryptEPUB(ctx context.Context, a asset.PublicationAsset, key ContentKey, w io.Writer, opts Options) error {
	if err := key.Validate(); err != nil {
		return err
	}
	c, err := openEPUB(ctx, a, opts, func(scheme protection.Scheme, _ map[string]manifest.Encryption) error {
		if scheme != protection.LCP {
			return ErrNotProtected
		}
		return nil
	})
	if err != nil {
		return err
	}
	defer c.f.Close()

	// Remote resources can't be decrypted: dropping their declarations,
	// with the license, would deliver their ciphertext as if it were in the clear.
	declared := make(map[string]manifest.Encryption)
	for _, entry := range c.existingEntries() {
		if entry.Encryption.Scheme != Scheme {
			continue
		}
		key, local := uriKey(entry.Href)
		if !local {
			return errors.Errorf("lcp: %s is declared LCP-encrypted, but is a remote resource, which can't be decrypted", entry.Href)
		}
		if err := checkLCPAlgorithm(entry.Href, entry.Encryption); err != nil {
			return err
		}
		declared[key] = entry.Encryption
	}
	if err := checkDeclaredEntries(declared, c.entries, func(path string) bool {
		return path == "mimetype" || path == protection.PathEPUBEncryption || isLicense(path)
	}); err != nil {
		return err
	}
	pins, err := checkEntryLicenseAliases(ctx, c.f, c.entries)
	if err != nil {
		return err
	}

	zw := ziputil.NewWriter(w)
	if err := ziputil.WriteMimetype(zw, mediatype.EPUB.String()); err != nil {
		return err
	}
	r := remuxer{ctx: ctx, f: c.f, zw: zw, key: key, pins: pins}

	for _, e := range c.entries {
		switch e.path {
		case "mimetype", protection.PathEPUBEncryption:
			continue // regenerated
		}
		if isLicense(e.path) {
			continue
		}
		link := c.link(e)
		if entry, ok := c.encryption[e.path]; ok && IsLCPEncrypted(&entry.Encryption) {
			if err := r.decrypt(e, link, entry.Encryption); err != nil {
				return err
			}
		} else if err := r.copy(e, link); err != nil {
			return err
		}
	}

	var remaining []protection.EncryptionEntry
	for _, entry := range c.existingEntries() {
		if !IsLCPEncrypted(&entry.Encryption) {
			remaining = append(remaining, entry)
		}
	}
	if err := writeEncryptionXML(zw, remaining); err != nil {
		return err
	}
	return errors.Wrap(zw.Close(), "lcp: failed finalizing EPUB")
}

// epubContainer gathers what the remuxing functions need to know about an
// EPUB.
type epubContainer struct {
	f          fetcher.Fetcher // raw access to the container, without deobfuscation
	entries    []containerEntry
	manifest   manifest.Manifest
	links      map[string]manifest.Link              // manifest links by resource key
	encryption map[string]protection.EncryptionEntry // encryption.xml entries by resource key
	// renditions are the manifests of the renditions other than the first,
	// whose links may describe resources otherwise: see
	// [epubContainer.checkFonts].
	renditions []manifest.Manifest
	// packageDocuments are the paths of the package documents, by
	// resource key.
	packageDocuments map[string]bool
}

// epubCheck validates the protection of an EPUB, given its scheme and its
// encryption.xml entries, before it is parsed.
type epubCheck func(scheme protection.Scheme, encryption map[string]manifest.Encryption) error

// openEPUB opens and inspects an EPUB, aborting with the error of check.
func openEPUB(ctx context.Context, a asset.PublicationAsset, opts Options, check epubCheck) (*epubContainer, error) {
	f, err := opts.createFetcher(ctx, a)
	if err != nil {
		return nil, err
	}
	c, err := readEPUB(ctx, a, f, check)
	if err != nil {
		f.Close()
		return nil, err
	}
	return c, nil
}

// renditionManifest returns the manifest of the rendition of an EPUB whose
// package document is at u, as the parser makes it for the first rendition.
func renditionManifest(ctx context.Context, f fetcher.Fetcher, u url.URL) (manifest.Manifest, error) {
	res := f.Get(ctx, manifest.Link{Href: manifest.NewHREF(u)})
	defer res.Close()
	doc, rerr := fetcher.ReadResourceAsXMLUpTo(ctx, res, epubparser.MaxDocumentSize)
	if rerr != nil {
		return manifest.Manifest{}, rerr
	}
	pd, err := epubparser.ParsePackageDocument(doc, u)
	if err != nil {
		return manifest.Manifest{}, err
	}
	return epubparser.PublicationFactory{PackageDocument: *pd}.Create(), nil
}

func readEPUB(ctx context.Context, a asset.PublicationAsset, f fetcher.Fetcher, check epubCheck) (*epubContainer, error) {
	scheme, doc, err := protection.IdentifyEPUBProtection(ctx, f)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: failed identifying EPUB protection")
	}
	if doc == nil {
		// Detection stops before reading encryption.xml when a license is present.
		res := f.Get(ctx, manifest.Link{Href: manifest.MustNewHREFFromString(protection.PathEPUBEncryption, false)})
		d, rerr := fetcher.ReadResourceAsXMLUpTo(ctx, res, protection.MaxMetadataSize)
		res.Close()
		if rerr == nil {
			doc = d
		} else if rerr.Code != fetcher.CodeNotFound {
			return nil, errors.Wrap(rerr, "lcp: failed reading encryption.xml")
		}
	}
	parsed := make(map[string]manifest.Encryption) // by URI
	merged := make(map[string]manifest.Encryption) // by resource key
	uris := make(map[string]string)                // by resource key
	if doc != nil {
		for _, entry := range epubparser.ParseEncryptionEntries(doc, scheme.URI()) {
			key, _ := uriKey(entry.Href)
			if err := declare(merged, key, entry.Encryption); err != nil {
				return nil, err
			}
			parsed[entry.Href], uris[key] = merged[key], entry.Href
		}
	}
	if err := check(scheme, parsed); err != nil {
		return nil, err
	}
	encryption := make(map[string]protection.EncryptionEntry, len(merged))
	for key, enc := range merged {
		encryption[key] = protection.EncryptionEntry{Href: uris[key], Encryption: enc}
	}

	// The parser is only used to learn about the resources: media types and
	// roles. Bytes are always read from the raw fetcher.
	builder, err := epubparser.NewParser(nil).Parse(ctx, a, f)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: failed parsing EPUB")
	}
	if builder == nil {
		return nil, errors.New("lcp: asset is not an EPUB")
	}
	// Every package document is exempt from encryption: the first rootfile,
	// whatever its media type, and those of the other renditions. Their
	// paths are cleaned as entry names are, so that e.g.
	// "./OEBPS/content.opf" is exempt as the entry it names.
	opfPath, err := epubparser.GetRootFilePath(ctx, f)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: failed locating the package document")
	}
	renditions, err := epubparser.GetPackageDocumentPaths(ctx, f)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: failed locating the package documents")
	}
	packageDocuments := map[string]bool{containerPath(opfPath.Path()): true}
	links := manifestLinks(builder.Manifest)
	var others []manifest.Manifest
	for _, u := range renditions {
		key := containerPath(u.Path())
		if packageDocuments[key] {
			continue
		}
		packageDocuments[key] = true
		// The other renditions tell the roles of their own resources, such
		// as their cover and navigation document, which the default
		// selection leaves in the clear: the relations of their links are
		// added to those of the first rendition.
		rendition, err := renditionManifest(ctx, f, u)
		if err != nil {
			return nil, errors.Wrapf(err, "lcp: failed parsing the package document %s", key)
		}
		others = append(others, rendition)
		for k, l := range manifestLinks(rendition) {
			if prev, ok := links[k]; ok {
				for _, rel := range l.Rels {
					prev.Rels = extensions.AddToSet(prev.Rels, rel)
				}
				l = prev
			}
			links[k] = l
		}
	}

	entries, err := containerEntries(ctx, f)
	if err != nil {
		return nil, err
	}

	return &epubContainer{
		f:          f,
		entries:    entries,
		manifest:   builder.Manifest,
		links:      links,
		renditions: others,
		encryption: encryption,

		packageDocuments: packageDocuments,
	}, nil
}

// isContainerFile reports whether the entry is part of the OCF container
// itself, rather than a publication resource: these are never encrypted.
func (c *epubContainer) isContainerFile(e containerEntry) bool {
	return e.path == "mimetype" || c.packageDocuments[e.path] || strings.HasPrefix(e.path, "META-INF/")
}

// link returns the manifest link of an entry, or the fetcher's link for
// files missing from the manifest.
func (c *epubContainer) link(e containerEntry) manifest.Link {
	if l, ok := c.links[e.path]; ok {
		return l
	}
	return e.link
}

// existingEntries returns the entries of the source encryption.xml, sorted
// by href for a deterministic output.
func (c *epubContainer) existingEntries() []protection.EncryptionEntry {
	entries := make([]protection.EncryptionEntry, 0, len(c.encryption))
	for _, entry := range c.encryption {
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b protection.EncryptionEntry) int {
		return strings.Compare(a.Href, b.Href)
	})
	return entries
}

func writeEncryptionXML(zw *zip.Writer, entries []protection.EncryptionEntry) error {
	data, err := protection.MarshalEPUBEncryption(entries)
	if err != nil {
		return errors.Wrap(err, "lcp: failed serializing encryption.xml")
	}
	if data == nil {
		return nil
	}
	return ziputil.WriteBytes(zw, protection.PathEPUBEncryption, data, zip.Deflate)
}

// encodedPath percent-encodes a container path for use in a URI.
func encodedPath(p string) (string, error) {
	u, err := url.URLFromPath(p)
	if err != nil {
		return "", errors.Wrapf(err, "lcp: invalid path %s", p)
	}
	return u.String(), nil
}
