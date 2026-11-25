package lcp

import (
	"context"
	"encoding/xml"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"sync"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	epubparser "github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// EncryptPublication returns a copy of the publication whose resources are
// encrypted with the content key, for writing with a packager: for example,
// packaging the result with pkg/packager/epub produces an LCP-protected EPUB.
// The resources are encrypted when read, with a copy of the key taken by
// EncryptPublication: the caller may clear its own.
//
// The links of the selected resources (see [Options]) declare their LCP
// encryption, with their original length, and reading them through the
// returned publication yields their ciphertext. A resource is selected and
// compressed as the first link to it in the reading order and resources
// says, and every link to it then declares the same encryption. The other resources the
// manifest links to are served as they are; other files are not served at
// all, so that no selected resource can be read in the clear under another
// name. A resource served in the clear which is the same file as a selected
// resource, e.g. through a symbolic link, is an error. The publication must not be
// protected already. The returned publication carries no services: links to
// the services of the source publication are not resources, and the SMIL
// alternates the media overlay service replaced with links to itself are
// restored, since packagers couldn't read the encrypted SMIL documents to
// pair them with their content documents.
//
// This rebuilds the publication from its manifest. To protect an existing EPUB
// or Readium package while leaving it otherwise untouched, use [EncryptEPUB]
// or [EncryptRWP] instead.
func EncryptPublication(ctx context.Context, p *pub.Publication, key ContentKey, opts Options) (*pub.Publication, error) {
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if err := checkUnprotected(p.Manifest); err != nil {
		return nil, err
	}
	fonts, err := obfuscatedFonts(p.Manifest)
	if err != nil {
		return nil, err
	}

	selection := opts.Encrypt
	if selection == nil {
		if p.Manifest.ConformsTo(manifest.ProfileEPUB) {
			selection = defaultEPUBSelection
		} else {
			selection = defaultRWPSelection(p.Manifest)
		}
	}

	m := p.Manifest
	m.ReadingOrder, err = restoreMediaOverlays(ctx, p.Fetcher, m)
	if err != nil {
		return nil, err
	}

	// Each resource is encrypted once, as the first link to it in the
	// reading order and resources says, as in the remuxes: other links to
	// it, answering differently, can't make it declared otherwise.
	planned := make(map[string]*manifest.Encryption) // by resource key
	decided := make(map[string]bool)
	plan := func(link manifest.Link) {
		if err != nil || outsidePackage(link) {
			return
		}
		key := resourceKey(link.Href)
		if decided[key] {
			return
		}
		decided[key] = true
		if fonts[key] || !selection(link) {
			return
		}
		if isLicense(key) {
			err = errLicensePathSelected(key)
			return
		}
		enc := &manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC}
		if opts.compress(link) {
			enc.Compression = CompressionDeflate
		}
		planned[key] = enc
	}
	walkLinks(m.ReadingOrder, plan)
	walkLinks(m.Resources, plan)
	if err != nil {
		return nil, err
	}
	// Every link to an encrypted resource declares its encryption, those of
	// the links too, which decryption reads.
	declared := func(link manifest.Link) manifest.Link {
		if !outsidePackage(link) {
			if enc := planned[resourceKey(link.Href)]; enc != nil {
				link.Properties = withEncryption(link.Properties, enc)
			}
		}
		return link
	}
	// A license, e.g. of a previous recipient, isn't part of the encrypted
	// publication, whichever list links to it: packagers would write it. The
	// fetcher only serves the resources of the manifest, so it isn't served
	// either.
	m.ReadingOrder = withoutLicenses(m.ReadingOrder)
	m.Resources = withoutLicenses(m.Resources)
	m.Links = withoutLicenses(m.Links)

	// Retain the exact source link. A package-relative key can have several
	// HTTP spellings that resolve to different URLs.
	entries := make(map[string]publicationEntry)
	var served []manifest.Link
	seenHref := make(map[string]bool)
	walkDeclarations(m, func(l manifest.Link) {
		if outsidePackage(l) {
			return
		}
		key := resourceKey(l.Href)
		if seenHref[l.Href.String()] {
			return
		}
		seenHref[l.Href.String()] = true
		if _, ok := entries[key]; !ok {
			entries[key] = publicationEntry{source: l, encryption: planned[key]}
		}
		served = append(served, l)
	})
	identified := append(slices.Clone(served), licenseLinks...)
	ids := make([]fs.FileInfo, len(identified))
	locations := make([]string, len(identified))
	for i, link := range identified {
		key := resourceKey(link.Href)
		entry := entries[key]
		first := i < len(served) && entry.source.Href.String() == link.Href.String()
		var enc *manifest.Encryption
		if first {
			enc = entry.encryption
		}
		id, location, err := identifyPublicationSource(ctx, p.Fetcher, link, enc)
		if err != nil {
			return nil, err
		}
		ids[i] = id
		locations[i] = locationKey(location)
		if i >= len(served) {
			continue
		}
		srcURL := ""
		if location != nil {
			srcURL = location.RemoveFragment().String()
		}
		if first {
			entry.file, entry.url = id, srcURL
			entries[key] = entry
		} else if entry.url != srcURL || ((entry.file != nil || id != nil) && !os.SameFile(entry.file, id)) {
			return nil, errors.Errorf("lcp: %s and %s resolve to different sources for the same package path", entry.source.Href, link.Href)
		}
	}
	groups, err := sameFiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	// Resources read from a URL, e.g. over HTTP, are the same when read from
	// the same URL, whatever the spelling of their links.
	groups = append(groups, sameLocations(locations)...)
	if err := checkLicenseAliases(identified, groups); err != nil {
		return nil, err
	}
	for _, group := range groups {
		var inClearHref, cipheredHref string
		for _, i := range group {
			if i >= len(served) {
				continue // a license
			}
			if planned[resourceKey(served[i].Href)] == nil {
				inClearHref = served[i].Href.String()
			} else {
				cipheredHref = served[i].Href.String()
			}
		}
		if inClearHref != "" && cipheredHref != "" {
			return nil, errors.Errorf("lcp: %s and %s are the same file, which would be both encrypted and in the clear", cipheredHref, inClearHref)
		}
	}

	m.ReadingOrder = mapLinks(m.ReadingOrder, declared)
	m.Resources = mapLinks(m.Resources, declared)
	m.Links = mapLinks(m.Links, declared)

	// The resources are encrypted when read, with a copy of the key: the
	// caller may clear its own meanwhile.
	f := &encryptingFetcher{
		Fetcher: p.Fetcher,
		key:     slices.Clone(key),
		entries: entries,
		random:  &serialRandom{r: opts.random()},
	}
	return pub.New(m, f, nil), nil
}

// identifyPublicationSource records an encrypted resource's length from the
// same pinned source used to authorize later reads.
func identifyPublicationSource(ctx context.Context, f fetcher.Fetcher, link manifest.Link, enc *manifest.Encryption) (fs.FileInfo, url.URL, error) {
	res := f.Get(ctx, link)
	defer res.Close()
	id, err := fileIdentity(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "lcp: failed identifying the file of %s", link.Href)
	}
	location, err := fetcher.PinLocation(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "lcp: failed pinning %s", link.Href)
	}
	if enc != nil {
		if err := pinTo(res, id); err != nil {
			return nil, nil, errors.Wrapf(err, "lcp: failed identifying %s", link.Href)
		}
		n, rerr := res.Length(ctx)
		if rerr != nil {
			return nil, nil, errors.Wrapf(rerr, "lcp: failed reading the length of %s", link.Href)
		}
		if n < 0 || (enc.Compression != CompressionDeflate && EncryptedLength(n) < 0) {
			return nil, nil, errors.Errorf("lcp: invalid resource length for %s", link.Href)
		}
		enc.OriginalLength = n
	}
	return id, location, nil
}

// restoreMediaOverlays returns a copy of the reading order of the manifest
// where each SMIL document of the resources is an alternate of the item it
// synchronizes, as the EPUB parser presents them.
//
// The media overlay service of the streamer replaces these alternates with
// links to itself, and packagers recover the pairings by reading the SMIL
// documents, as pkg/packager/epub does. Once encrypted, the documents can't
// be read, and the encrypted publication carries no services: the pairings
// are recovered from the plaintext beforehand, replacing the links to the
// service.
func restoreMediaOverlays(ctx context.Context, f fetcher.Fetcher, m manifest.Manifest) (manifest.LinkList, error) {
	items := make(map[string]int) // resource key → index in the reading order
	paired := make(map[int]bool)  // items with a SMIL alternate
	for i, l := range m.ReadingOrder {
		if _, ok := items[resourceKey(l.Href)]; !ok {
			items[resourceKey(l.Href)] = i
		}
		if slices.ContainsFunc(l.Alternates, isSMIL) {
			paired[i] = true
		}
	}

	readingOrder := slices.Clone(m.ReadingOrder)
	for _, smil := range m.Resources {
		if !isSMIL(smil) || outsidePackage(smil) {
			continue
		}
		target, err := smilTarget(ctx, f, smil)
		if err != nil {
			return nil, err
		}
		i, ok := items[target]
		if !ok || paired[i] {
			continue
		}
		paired[i] = true
		item := readingOrder[i]
		alternates := slices.Clone(item.Alternates)
		if j := slices.IndexFunc(alternates, isMediaOverlayServiceLink); j >= 0 {
			alternates[j] = smil
		} else {
			alternates = append(alternates, smil)
		}
		item.Alternates = alternates
		readingOrder[i] = item
	}
	return readingOrder, nil
}

func isSMIL(link manifest.Link) bool {
	return link.MediaType != nil && link.MediaType.Matches(&mediatype.SMIL)
}

// mediaOverlayServicePath is the path of the links to the media overlay
// service, whatever their parameters.
var mediaOverlayServicePath = pub.MediaOverlayLink.URL(nil, nil).Path()

// isMediaOverlayServiceLink reports whether a link points to the media
// overlay service, which serves SMIL documents as guided navigation.
func isMediaOverlayServiceLink(link manifest.Link) bool {
	u := link.URL(nil, nil)
	return u != nil && u.Path() == mediaOverlayServicePath
}

// smilTarget returns the resource key of the content document a SMIL
// document synchronizes: the one its first text element refers to. It
// returns an empty key when there is none, e.g. for a malformed document.
//
// The document is only read up to its first text element, and at most up to
// the size limit of the EPUB parser's documents: the XML decoder holds whole
// text nodes and attributes in memory, which a small package may inflate a
// thousandfold. Exceeding it is an error.
func smilTarget(ctx context.Context, f fetcher.Fetcher, smil manifest.Link) (string, error) {
	res := f.Get(ctx, smil)
	defer res.Close()
	var src string
	_, rerr := streamThrough(ctx, res, func(r io.Reader) (int64, error) {
		limited := &io.LimitedReader{R: r, N: epubparser.MaxDocumentSize + 1}
		decoder := xml.NewDecoder(limited)
		decoder.Strict = false
		for src == "" {
			token, err := decoder.Token()
			if err != nil {
				if limited.N <= 0 {
					return 0, errors.Wrapf(fetcher.ErrResourceTooLarge, "%s is larger than %s", smil.Href, extensions.FormatSize(epubparser.MaxDocumentSize))
				}
				return 0, nil
			}
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "text" {
				for _, attr := range start.Attr {
					if attr.Name.Local == "src" {
						src = attr.Value
					}
				}
			}
		}
		return 0, nil
	})
	if rerr != nil {
		return "", errors.Wrapf(rerr, "lcp: failed reading %s", smil.Href)
	}

	base := smil.URL(nil, nil)
	ref, err := url.FromEPUBHref(src)
	if src == "" || base == nil || err != nil {
		return "", nil
	}
	resolved := base.Resolve(ref)
	if resolved == nil {
		return "", nil
	}
	return containerPath(resolved.Path()), nil
}

// DecryptPublication returns a copy of the publication whose LCP-encrypted
// resources are decrypted with the content key.
//
// The links of the decrypted resources no longer declare their encryption, so
// that writing the result with a packager produces an unprotected
// publication. Other encryption metadata, such as font obfuscation, is
// preserved. The license isn't part of the result, and a remote resource
// declared LCP-encrypted is an error, as it would stay encrypted without it.
// The returned publication carries no services: to keep them, call
// [DecryptBuilder] while the publication is being built instead.
func DecryptPublication(p *pub.Publication, key ContentKey) (*pub.Publication, error) {
	// The publication is decrypted to be written: remote resources, which
	// packagers link to rather than copy, would stay encrypted, without the
	// license. The streamer, using DecryptBuilder, decrypts them as it reads
	// them.
	var err error
	walkDeclarations(p.Manifest, func(l manifest.Link) {
		if enc := l.Properties.Encryption(); err == nil && requiresLCPLicense(enc) && outsidePackage(l) {
			err = errOutsideLCP(l.Href.String(), l)
		}
	})
	if err != nil {
		return nil, err
	}
	m, f, err := decryptManifestAndFetcher(p.Manifest, p.Fetcher, key, 0, declarations{})
	if err != nil {
		return nil, err
	}
	return pub.New(m, f, nil), nil
}

// DecryptBuilder makes the publication under construction decrypt its
// LCP-encrypted resources with the content key, as [DecryptPublication] does,
// while keeping its services. It is meant to be called from
// streamer.Config.OnCreatePublication.
//
// Only the resources the manifest declares are decrypted. [ContentProtection]
// also decrypts those that only the package declares, in an encryption.xml
// entry for a resource missing from the package document, or a manifest link
// the parser drops for lacking a media type.
func DecryptBuilder(b *pub.Builder, key ContentKey) error {
	return decryptBuilder(b, key, 0, declarations{})
}

// decryptBuilder is [DecryptBuilder] with the given maximum inflated length
// (see [inflationLimit]), and the declarations of the package, by resource
// key, which the manifest may lack: see [packageDeclarations].
func decryptBuilder(b *pub.Builder, key ContentKey, maxInflated int64, declared declarations) error {
	m, f, err := decryptManifestAndFetcher(b.Manifest, b.Fetcher, key, maxInflated, declared)
	if err != nil {
		return err
	}
	b.Manifest = m
	b.Fetcher = f
	return nil
}

func decryptManifestAndFetcher(m manifest.Manifest, f fetcher.Fetcher, key ContentKey, maxInflated int64, declared declarations) (manifest.Manifest, fetcher.Fetcher, error) {
	d, err := NewDecryptor(key)
	if err != nil {
		return m, nil, err
	}
	d.maxInflated = maxInflated

	// Every declaration of a resource is merged before the LCP-encrypted ones
	// are selected, as in the remuxes: see [lcpEncrypted].
	merged := maps.Clone(declared.local)
	if merged == nil {
		merged = make(map[string]manifest.Encryption)
	}
	// Remote resources declared LCP-encrypted are decrypted when the fetcher
	// can read them, whether they are requested with their link or only
	// their href: their declarations are kept by URL (see remoteKey), merged
	// before the LCP-encrypted ones are selected, as for the resources of
	// the package.
	remoteDeclared := maps.Clone(declared.remote)
	if remoteDeclared == nil {
		remoteDeclared = make(map[string]manifest.Encryption)
	}
	walkDeclarations(m, func(link manifest.Link) {
		enc := link.Properties.Encryption()
		switch {
		case err != nil || enc == nil:
		case !outsidePackage(link):
			err = declare(merged, resourceKey(link.Href), *enc)
		case isRemote(link):
			err = declare(remoteDeclared, remoteKey(link), *enc)
		}
	})
	if err != nil {
		return m, nil, err
	}
	encrypted, err := lcpEncrypted(merged)
	if err != nil {
		return m, nil, err
	}
	remote, err := lcpEncrypted(remoteDeclared)
	if err != nil {
		return m, nil, err
	}

	decrypt := func(link manifest.Link) manifest.Link {
		if IsLCPEncrypted(link.Properties.Encryption()) && !outsidePackage(link) {
			link.Properties = withEncryption(link.Properties, nil)
		}
		return link
	}
	// The license isn't part of the decrypted publication: see
	// [decryptingFetcher.Get].
	m.ReadingOrder = withoutLicenses(mapLinks(m.ReadingOrder, decrypt))
	m.Resources = withoutLicenses(mapLinks(m.Resources, decrypt))
	m.Links = withoutLicenses(mapLinks(m.Links, decrypt))
	return m, &decryptingFetcher{Fetcher: f, decryptor: d, encrypted: encrypted, remote: remote}, nil
}

// remoteKey returns the key of a remote link's resource: its URL, as the
// EPUB parser keys remote resources (see [epubparser.ResourceID]).
func remoteKey(link manifest.Link) string {
	u := link.URL(nil, nil)
	if u == nil {
		return link.Href.String()
	}
	key, _ := epubparser.ResourceID(u)
	return key
}

// checkUnprotected returns [ErrAlreadyProtected] when a resource of the
// manifest is encrypted with anything but font obfuscation, which is not
// protection.
func checkUnprotected(m manifest.Manifest) error {
	var err error
	check := func(link manifest.Link) {
		if enc := link.Properties.Encryption(); err == nil && enc != nil && !protection.IsFontObfuscation(enc.Algorithm) {
			err = errors.Wrapf(ErrAlreadyProtected, "%s is encrypted with %s", link.Href, enc.Algorithm)
		}
	}
	walkDeclarations(m, check)
	return err
}

// encryptingFetcher serves the ciphertext of the resources selected by
// [EncryptPublication], and the other resources of the manifest as they are.
//
// It fails closed: resources the manifest doesn't link to are not served.
// Otherwise, a selected resource could be read in the clear under another
// name the source fetcher resolves to it, e.g. differing in case on a
// case-insensitive file system, or a symbolic link.
type encryptingFetcher struct {
	fetcher.Fetcher
	key     ContentKey
	entries map[string]publicationEntry
	random  *serialRandom
}

type publicationEntry struct {
	source     manifest.Link
	encryption *manifest.Encryption
	file       fs.FileInfo
	url        string
}

// serialRandom draws the random bytes of each resource at once, one resource
// at a time: the resources of a publication may be fetched concurrently,
// while [Options.Random] is read from one goroutine at a time.
type serialRandom struct {
	mu sync.Mutex
	r  io.Reader
}

func (s *serialRandom) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return io.ReadFull(s.r, p)
}

// Links implements Fetcher
func (f *encryptingFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	links, err := f.Fetcher.Links(ctx)
	if err != nil {
		return nil, err
	}
	served := make(manifest.LinkList, 0, len(links))
	for _, l := range links {
		key := resourceKey(l.Href)
		if entry, ok := f.entries[key]; ok && !outsidePackage(l) {
			listed := entry.source
			if entry.encryption != nil {
				listed.Properties = withEncryption(listed.Properties, entry.encryption)
			}
			served = append(served, listed)
		}
	}
	return served, nil
}

// Get implements Fetcher
func (f *encryptingFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	notFound := fetcher.NewFailureResource(link, fetcher.NotFound(errors.Errorf("lcp: %s is not a resource of the publication", link.Href)))
	// A link outside of the package isn't a resource of the publication,
	// whatever the resource of the package its path matches.
	if outsidePackage(link) {
		return notFound
	}
	key := resourceKey(link.Href)
	entry, ok := f.entries[key]
	if !ok {
		return notFound
	}
	res := f.Fetcher.Get(ctx, entry.source)
	var location string
	u, err := fetcher.PinLocation(res)
	if err != nil {
		res.Close()
		return fetcher.NewFailureResource(link, fetcher.Forbidden(err))
	}
	if u != nil {
		location = u.RemoveFragment().String()
	}
	if location != entry.url {
		res.Close()
		return fetcher.NewFailureResource(link, fetcher.Forbidden(errors.New("lcp: source URL changed since it was checked")))
	}
	if err := pinTo(res, entry.file); err != nil {
		res.Close()
		return fetcher.NewFailureResource(link, fetcher.Forbidden(errors.Wrapf(err, "lcp: %s can't be served", link.Href)))
	}
	if entry.encryption == nil {
		return res
	}
	compress := entry.encryption.Compression == CompressionDeflate
	er, err := newEncryptingResource(res, f.key, compress, f.random)
	if err != nil {
		res.Close()
		return fetcher.NewFailureResource(link, fetcher.Other(err))
	}
	er.declaration = entry.encryption
	return er
}

// isLicenseLink reports whether a link points to the license of the package,
// at the path of either format. A template is expanded without parameters,
// as fetchers do, rather than taken for a link outside of the package: it may
// resolve to the license.
func isLicenseLink(link manifest.Link) bool {
	u := link.URL(nil, nil)
	return u != nil && !url.IsRemoteReference(u) && isLicense(containerPath(u.Path()))
}

// withoutLicenses returns the links of ll which don't point to the license,
// with their alternates and children.
func withoutLicenses(ll manifest.LinkList) manifest.LinkList {
	if ll == nil {
		return nil
	}
	out := make(manifest.LinkList, 0, len(ll))
	for _, link := range ll {
		if isLicenseLink(link) {
			continue
		}
		link.Alternates = withoutLicenses(link.Alternates)
		link.Children = withoutLicenses(link.Children)
		out = append(out, link)
	}
	return out
}

// decryptingFetcher decrypts the resources it knows are LCP-encrypted,
// whatever the link they are requested with.
type decryptingFetcher struct {
	fetcher.Fetcher
	decryptor Decryptor
	encrypted map[string]manifest.Encryption // by resource key
	remote    map[string]manifest.Encryption // by remoteKey
}

// licenseIdentities returns the identities of the files of the licenses of
// the package, and the URLs they are read from (see [locationKey]), as of
// now: a license replaced since an earlier check, e.g. by renaming a new file
// over it, is identified anew.
func (f *decryptingFetcher) licenseIdentities(ctx context.Context) (files []fs.FileInfo, locations []string, err error) {
	for _, l := range licenseLinks {
		res := f.Fetcher.Get(ctx, l)
		id, err := fileIdentity(res)
		if err != nil {
			res.Close()
			return nil, nil, errors.Wrapf(err, "lcp: failed identifying the license at %s", l.Href)
		}
		if id != nil {
			files = append(files, id)
		}
		u, err := fetcher.PinLocation(res)
		if err != nil {
			res.Close()
			return nil, nil, err
		}
		if loc := locationKey(u); loc != "" {
			locations = append(locations, loc)
		}
		res.Close()
	}
	return files, locations, nil
}

// check returns the identity of the file a resource is read from, if any,
// and whether it is the license, under another name: the same file, or the
// same URL. A file which can't be identified is an error, rather than taken
// for a resource not read from a file, which would be read unchecked.
func (f *decryptingFetcher) check(res fetcher.Resource, files []fs.FileInfo, locations []string) (fs.FileInfo, bool, error) {
	var info fs.FileInfo
	if fb, ok := res.(fetcher.FileBacked); ok {
		i, err := fb.FileInfo()
		switch {
		case err == nil && i == nil:
			return nil, false, errors.New("file identity is missing")
		case err == nil:
			info = i
			for _, license := range files {
				if os.SameFile(info, license) {
					return info, true, nil
				}
			}
		case !errors.Is(err, fetcher.ErrNotFileBacked):
			return nil, false, err
		}
	}
	u, err := fetcher.PinLocation(res)
	if err != nil {
		return nil, false, err
	}
	if loc := locationKey(u); loc != "" && slices.Contains(locations, loc) {
		return info, true, nil
	}
	return info, false, nil
}

// Get implements Fetcher
//
// The license is not served: a decrypted publication doesn't need it, and
// written by a packager, it would disclose the identifier and passphrase hint
// of its recipient, and make the publication look protected. Neither are
// other names of its file, e.g. hard links, or other spellings on a
// case-insensitive file system: the files of the other resources are pinned
// to the ones checked not to be the license.
func (f *decryptingFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	notServed := func() fetcher.Resource {
		return fetcher.NewFailureResource(link, fetcher.NotFound(errors.Errorf("lcp: the license of a decrypted publication isn't served")))
	}
	if isLicenseLink(link) {
		return notServed()
	}
	res := f.Fetcher.Get(ctx, link)
	files, locations, err := f.licenseIdentities(ctx)
	if err != nil {
		res.Close()
		return fetcher.NewFailureResource(link, fetcher.Forbidden(err))
	}
	info, license, err := f.check(res, files, locations)
	switch {
	case err != nil:
		res.Close()
		return fetcher.NewFailureResource(link, fetcher.NotFound(errors.Wrapf(err, "lcp: failed identifying the file of %s", link.Href)))
	case license:
		res.Close()
		return notServed()
	case info != nil:
		if err := pinTo(res, info); err != nil {
			res.Close()
			return fetcher.NewFailureResource(link, fetcher.Forbidden(errors.Wrapf(err, "lcp: %s can't be served", link.Href)))
		}
	}
	switch {
	case !outsidePackage(link):
		if enc, ok := f.encrypted[resourceKey(link.Href)]; ok {
			return f.decryptor.decrypt(res, enc)
		}
	case isRemote(link):
		// Decrypted from the declaration of its URL, if any, rather than
		// from the resource of the package its path matches.
		if enc, ok := f.remote[remoteKey(link)]; ok {
			return f.decryptor.decrypt(res, enc)
		}
	}
	return f.decryptor.Transform(res)
}

// Links implements Fetcher, without the license: see [decryptingFetcher.Get].
func (f *decryptingFetcher) Links(ctx context.Context) (manifest.LinkList, error) {
	links, err := f.Fetcher.Links(ctx)
	if err != nil {
		return nil, err
	}
	// Filtered into a slice of its own: fetchers may return their cached one.
	files, locations, err := f.licenseIdentities(ctx)
	if err != nil {
		return nil, err
	}
	listed := make(manifest.LinkList, 0, len(links))
	for _, l := range links {
		if isLicenseLink(l) {
			continue
		}
		res := f.Fetcher.Get(ctx, l)
		_, license, err := f.check(res, files, locations)
		res.Close()
		if !license && err == nil {
			listed = append(listed, l)
		}
	}
	return listed, nil
}
