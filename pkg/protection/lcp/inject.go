package lcp

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"encoding/json"
	"encoding/xml"
	stderrors "errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/ziputil"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	epubparser "github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/protection"
)

// InjectLicense embeds a license into an LCP-protected EPUB or Readium Web
// Publication package, as a License Server does when delivering a
// publication to a user, and writes the result to w. An existing license is
// replaced, as is a license at the path of the other format, which would be
// ambiguous.
//
// The package is remuxed: its entries are copied byte for byte. A parsed
// license is written as it was parsed, keeping its signature valid.
//
// A publication which is not protected with LCP, e.g. the original
// publication instead of its encrypted version, is refused with
// [ErrNotProtected]. A license it already holds doesn't make it protected:
// resources of the package must be declared LCP-encrypted, each of them must
// be in the package, and have the size of the ciphertext of its content. This
// is exact for uncompressed resources declaring their original length, and a
// structural check otherwise (the size of AES-CBC ciphertexts), so that
// neither a leftover license nor leftover declarations deliver a publication
// in the clear as a protected one.
//
// What is checked is what is written, even if the package changes meanwhile:
// the document declaring the encrypted resources (encryption.xml or
// manifest.json) is written as it was read, it must be valid, and a resource
// declared several times must be declared consistently. The declared
// resources are copied from the files checked, whole: those which may change
// while they are copied, the files of exploded packages and remote
// resources, are first copied to temporary files, so that w never receives
// content which doesn't pass the checks.
//
// These checks catch mistakes, such as injecting into a publication in the
// clear, or into a package changing meanwhile. They can't tell a ciphertext
// from other content of its size, so they don't make a package from an
// untrusted source safe to deliver: inject licenses into the packages the
// License Server's encryption produced. Only [Options.ArchiveFactory] is
// used.
func InjectLicense(ctx context.Context, a asset.PublicationAsset, license *License, w io.Writer, opts Options) error {
	if license == nil {
		return errNilLicense
	}
	data := license.Raw()
	if data == nil {
		var err error
		if data, err = json.Marshal(license); err != nil {
			return errors.Wrap(err, "lcp: failed serializing license")
		}
	}

	f, err := opts.createFetcher(ctx, a)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := containerEntries(ctx, f)
	if err != nil {
		return err
	}
	// The licenses the package holds are replaced, and other names of
	// their files mustn't deliver them instead.
	pins, err := checkEntryLicenseAliases(ctx, f, entries)
	if err != nil {
		return err
	}

	// The document declaring the encrypted resources, by path.
	var licensePath, declarationsPath string
	var declarations []byte
	var declared map[string]manifest.Encryption
	// The format is told as the streamer tells it.
	format, err := assetFormat(ctx, a, f)
	if err != nil {
		return err
	}
	switch format {
	case epubFormat:
		scheme, _, err := protection.IdentifyEPUBProtection(ctx, f)
		if err != nil {
			return errors.Wrap(err, "lcp: failed identifying EPUB protection")
		}
		if scheme != protection.LCP {
			return ErrNotProtected
		}
		licensePath, declarationsPath = protection.PathEPUBLCPLicense, protection.PathEPUBEncryption
		if declarations, err = readEntry(ctx, f, declarationsPath, protection.MaxMetadataSize); err != nil {
			return err
		}
		if declared, err = epubEncryptedResources(declarations); err != nil {
			return err
		}

	case rwpFormat:
		licensePath, declarationsPath = protection.PathRWPLCPLicense, rwpManifestPath
		if declarations, err = readEntry(ctx, f, declarationsPath, maxManifestSize); err != nil {
			return err
		}
		// The manifest written must be one readers open, besides
		// declaring encrypted resources.
		if declarations != nil {
			if _, err := parseRWPManifest(declarations); err != nil {
				return err
			}
		}
		if declared, err = rwpEncryptedResources(declarations); err != nil {
			return err
		}

	default:
		return errors.New("lcp: asset is neither an EPUB nor a Readium Web Publication package")
	}
	// The entries the injection replaces or drops aren't delivered: they
	// can't be the protected resources.
	epub := licensePath == protection.PathEPUBLCPLicense
	for key := range declared {
		if isLicense(key) || key == declarationsPath || (epub && key == "mimetype") {
			return errors.Wrapf(ErrNotProtected, "%s is declared LCP-encrypted, but is replaced by the injection", key)
		}
	}
	checked, err := checkEncryptedResources(ctx, f, entries, declared, pins)
	if err != nil {
		return err
	}

	zw := ziputil.NewWriter(w)
	r := remuxer{ctx: ctx, f: f, zw: zw, pins: pins}
	if epub {
		if err := ziputil.WriteMimetype(zw, mediatype.EPUB.String()); err != nil {
			return err
		}
	} else if err := r.writeAs(declarationsPath, declarations); err != nil {
		return err // the manifest comes first
	}
	if err := ziputil.WriteBytes(zw, licensePath, data, zip.Deflate); err != nil {
		return err
	}
	for _, e := range entries {
		switch {
		// The mimetype file of an EPUB is written first: in a Readium Web
		// Publication package, an entry of that name is a resource.
		case isLicense(e.path), epub && e.path == "mimetype", !epub && e.path == declarationsPath:
			continue
		case e.path == declarationsPath:
			if err := r.writeAs(e.path, declarations); err != nil {
				return err
			}
		case checked[e.path] != nil:
			if err := r.copyChecked(e, *checked[e.path]); err != nil {
				return err
			}
		default:
			if err := r.copy(e, e.link); err != nil {
				return err
			}
		}
	}
	return errors.Wrap(zw.Close(), "lcp: failed finalizing package")
}

// epubEncryptedResources returns the encryption of the resources of the
// package an encryption.xml document declares LCP-encrypted, by resource key.
func epubEncryptedResources(document []byte) (map[string]manifest.Encryption, error) {
	local, _, err := epubDeclarations(document)
	if err != nil {
		return nil, err
	}
	return lcpEncrypted(local)
}

// epubDeclarations returns the encryption an encryption.xml document declares
// for the resources of the package, by resource key, and for remote ones, by
// their key (see [uriKey]), whatever the algorithm: the declarations of a
// resource are merged, but not selected (see [lcpEncrypted]).
func epubDeclarations(document []byte) (local, remote map[string]manifest.Encryption, err error) {
	if document == nil {
		return nil, nil, nil
	}
	doc, err := xmlquery.ParseWithOptions(bytes.NewReader(document), xmlquery.ParserOptions{
		Decoder: &xmlquery.DecoderOptions{Strict: true, Entity: xml.HTMLEntity},
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "lcp: invalid encryption.xml")
	}
	local = make(map[string]manifest.Encryption)
	remote = make(map[string]manifest.Encryption)
	for _, entry := range epubparser.ParseEncryptionEntries(doc, protection.LCP.URI()) {
		key, isLocal := uriKey(entry.Href)
		declared := local
		if !isLocal {
			declared = remote
		}
		if err := declare(declared, key, entry.Encryption); err != nil {
			return nil, nil, err
		}
	}
	return local, remote, nil
}

// rwpEncryptedResources returns the encryption of the resources of the
// package a raw Readium Web Publication manifest declares LCP-encrypted, by
// resource key: see [rwpDeclarations].
func rwpEncryptedResources(raw []byte) (map[string]manifest.Encryption, error) {
	declared, _, err := rwpDeclarations(raw)
	if err != nil {
		return nil, err
	}
	return lcpEncrypted(declared)
}

// rwpDeclarations returns the encryption a raw Readium Web Publication
// manifest declares for the resources of the package, by resource key,
// whatever the algorithm. The raw manifest is read rather than the parsed
// one, which drops e.g. the links without a media type, whatever they
// declare. Outside declarations retain their manifest order.
func rwpDeclarations(raw []byte) (map[string]manifest.Encryption, []outsideDeclaration, error) {
	if raw == nil {
		return nil, nil, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, errors.Wrap(err, "lcp: invalid manifest.json")
	}
	declared := make(map[string]manifest.Encryption)
	var outside []outsideDeclaration
	var err error
	walkRawDeclarations(doc, func(link map[string]any) {
		if err != nil {
			return
		}
		props, _ := link["properties"].(map[string]any)
		raw, _ := props["encrypted"].(map[string]any)
		enc, jerr := manifest.EncryptionFromJSON(raw)
		if jerr != nil || enc == nil {
			// As the manifest parser, which ignores invalid declarations.
			return
		}
		// Hrefs are parsed as the manifest parser does: a declaration whose
		// resource can't be found is an error, rather than ignored.
		href, _ := link["href"].(string)
		templated, _ := link["templated"].(bool)
		h, herr := manifest.ParseLinkHREF(href, templated)
		if href == "" || herr != nil {
			err = errors.Errorf("lcp: %q in manifest.json is declared encrypted, but isn't a valid link", href)
			return
		}
		l := manifest.Link{Href: h}
		if outsidePackage(l) {
			outside = append(outside, outsideDeclaration{href: href, link: l, enc: *enc})
		} else {
			err = declare(declared, resourceKey(h), *enc)
		}
	})
	return declared, outside, err
}

// lcpEncrypted returns the LCP-encrypted resources among the merged
// declarations of resources. Declarations are merged first, then selected,
// so that a resource also declared otherwise, e.g. obfuscated, is an error
// rather than taken for LCP-encrypted only. So is a resource declared with the
// LCP scheme but an algorithm this package doesn't support (see
// [checkLCPAlgorithm]).
func lcpEncrypted(declared map[string]manifest.Encryption) (map[string]manifest.Encryption, error) {
	for _, key := range slices.Sorted(maps.Keys(declared)) {
		if enc := declared[key]; enc.Scheme == Scheme {
			if err := checkLCPAlgorithm(displayKey(key), enc); err != nil {
				return nil, err
			}
		}
	}
	maps.DeleteFunc(declared, func(_ string, enc manifest.Encryption) bool {
		return !IsLCPEncrypted(&enc)
	})
	return declared, nil
}

// requiresLCPLicense includes unknown LCP algorithms, which must fail closed.
func requiresLCPLicense(enc *manifest.Encryption) bool {
	return enc != nil && enc.Scheme == Scheme && !protection.IsFontObfuscation(enc.Algorithm)
}

// checkLCPAlgorithm returns an error when a resource declared with the LCP
// scheme isn't encrypted with an algorithm this package supports. Taken for
// a resource of another scheme instead, it would be left encrypted, e.g. by
// a decryption dropping the license it needs. Font obfuscation isn't
// encryption with the content key: older producers declared obfuscated fonts
// of LCP publications with the LCP scheme.
func checkLCPAlgorithm(href string, enc manifest.Encryption) error {
	if !IsLCPEncrypted(&enc) && !protection.IsFontObfuscation(enc.Algorithm) {
		return errors.Errorf("lcp: %s is declared LCP-encrypted with %s, which isn't supported", href, enc.Algorithm)
	}
	return nil
}

// declare adds the declaration of the encryption of a resource to declared,
// by key, merged with the previous ones (see [protection.MergeEncryption]).
// Checking and decrypting a resource, and the EPUB parser, apply the same
// rule.
func declare(declared map[string]manifest.Encryption, key string, enc manifest.Encryption) error {
	prev, ok := declared[key]
	if !ok {
		declared[key] = enc
		return nil
	}
	merged, err := protection.MergeEncryption(prev, enc)
	if err != nil {
		return errors.Errorf("lcp: %s is %v", displayKey(key), err)
	}
	declared[key] = merged
	return nil
}

// displayKey returns a key of declarations as it is shown in errors: the
// path of a resource of the package, or the URL of a remote one, without the
// slash keeping it apart from paths (see [epubparser.ResourceID]).
func displayKey(key string) string {
	return strings.TrimPrefix(key, "/")
}

// checkedResource is a resource declared LCP-encrypted, as it was checked.
type checkedResource struct {
	// file identifies the file it is read from, nil when it isn't read from a
	// file, e.g. for an entry of an archive opened once.
	file fs.FileInfo
	// length is the length it was checked to have.
	length int64
}

// checkEncryptedResources returns [ErrNotProtected] unless resources are
// declared LCP-encrypted, and each of them is in the package, with the size
// of the ciphertext of its content. It returns them as they were checked, by
// path, to copy them as such.
//
// The size is exact for uncompressed resources declaring their original
// length, as readers rely on it to locate their content: a resource
// decrypted in place is shorter than its ciphertext. For the others, the
// check is structural: an AES-CBC ciphertext is an initialization vector
// followed by padded blocks, so a multiple of the block size and at least two
// blocks, which content in the clear may happen to be.
func checkEncryptedResources(ctx context.Context, f fetcher.Fetcher, entries []containerEntry, declared map[string]manifest.Encryption, pins map[string]fs.FileInfo) (map[string]*checkedResource, error) {
	if len(declared) == 0 {
		return nil, errors.Wrap(ErrNotProtected, "no resource of the package is declared LCP-encrypted")
	}
	byPath := make(map[string]containerEntry, len(entries))
	for _, e := range entries {
		byPath[e.path] = e
	}
	checked := make(map[string]*checkedResource, len(declared))
	for _, key := range slices.Sorted(maps.Keys(declared)) {
		e, ok := byPath[key]
		if !ok {
			return nil, errors.Wrapf(ErrNotProtected, "%s is declared LCP-encrypted, but is not in the package", key)
		}
		c, err := checkResource(ctx, f, e, pins[e.path])
		if err != nil {
			return nil, err
		}
		n := c.length
		enc := declared[key]
		if uncompressed := enc.Compression == "" || enc.Compression == protection.CompressionNone; uncompressed && enc.OriginalLength > 0 {
			if want := EncryptedLength(enc.OriginalLength); want < 0 || n != want {
				return nil, errors.Wrapf(ErrNotProtected, "%s is declared LCP-encrypted, but its %d bytes aren't the %d of the ciphertext of its %d bytes", e.path, n, want, enc.OriginalLength)
			}
		} else if n < 2*aes.BlockSize || n%aes.BlockSize != 0 {
			return nil, errors.Wrapf(ErrNotProtected, "%s is declared LCP-encrypted, but its %d bytes can't be an AES-CBC ciphertext", e.path, n)
		}
		checked[e.path] = c
	}
	return checked, nil
}

// checkResource returns the length of the resource of an entry, and the file
// it is read from: the length is the size of that file, so that it describes
// the file the resource is pinned to when copied.
func checkResource(ctx context.Context, f fetcher.Fetcher, e containerEntry, id fs.FileInfo) (*checkedResource, error) {
	res := f.Get(ctx, e.link)
	defer res.Close()
	if fb, ok := res.(fetcher.FileBacked); ok {
		info, err := fb.FileInfo()
		switch {
		case err == nil:
			// The file must be the one identified when the entries were
			// checked not to be licenses, which it is copied from.
			if id == nil || !os.SameFile(info, id) {
				return nil, errors.Errorf("lcp: %s changed since it was checked", e.path)
			}
			return &checkedResource{file: id, length: info.Size()}, nil
		case !errors.Is(err, fetcher.ErrNotFileBacked):
			return nil, errors.Wrapf(err, "lcp: failed identifying the file of %s", e.path)
		}
	}
	if id != nil {
		return nil, errors.Errorf("lcp: %s changed since it was checked", e.path)
	}
	n, rerr := res.Length(ctx)
	if rerr != nil {
		return nil, errors.Wrapf(rerr, "lcp: failed reading %s", e.path)
	}
	return &checkedResource{length: n}, nil
}

// writeAs writes the content of an entry, as it was read, with the storage
// method of the entry.
func (r remuxer) writeAs(path string, content []byte) error {
	res := r.f.Get(r.ctx, manifest.Link{Href: manifest.MustNewHREFFromString(path, false)})
	method := storageMethod(res, manifest.Link{})
	res.Close()
	return ziputil.WriteBytes(r.zw, path, content, method)
}

// copyChecked copies an entry as it was checked: from the file it was read
// from, if any, with the length it was checked to have.
//
// Only the entries of an archive read from a single version of it (see
// [fetcher.Stable]) are copied as they are, compressed, once a deflated one
// was checked to inflate to that length. Other resources, which may change
// while they are copied, such as files and remote resources, are first copied
// to a temporary file, up to that length, and checked there: the writer, e.g.
// a response to a client, never receives content which doesn't pass the
// check.
func (r remuxer) copyChecked(e containerEntry, c checkedResource) (retErr error) {
	res := r.f.Get(r.ctx, e.link)
	defer res.Close()
	if err := pinTo(res, c.file); err != nil {
		return errors.Wrapf(err, "lcp: %s can't be copied as it was checked", e.path)
	}
	method := storageMethod(res, e.link)
	if st, ok := res.(fetcher.Stable); ok && st.Stable() && c.file == nil {
		// The length checked is the one the entry declares, which its
		// compressed bytes may not inflate to.
		if cres, ok := res.(fetcher.CompressedResource); ok && cres.CompressedAs(archive.CompressionMethodDeflate) {
			if err := checkLength(r.ctx, res, io.Discard, c.length); err != nil {
				return errors.Wrapf(err, "lcp: %s isn't as it was checked", e.path)
			}
		}
		return errors.Wrapf(ziputil.CopyEntry(r.ctx, r.zw, e.path, res, method), "lcp: failed copying %s", e.path)
	}

	staged, err := os.CreateTemp("", "lcp-*")
	if err != nil {
		return errors.Wrap(err, "lcp: failed creating a temporary file")
	}
	defer func() {
		// Close before removing the staging file, including on Windows.
		if err := stderrors.Join(staged.Close(), os.Remove(staged.Name())); err != nil {
			retErr = stderrors.Join(retErr, errors.Wrap(err, "lcp: failed cleaning up staged resource"))
		}
	}()
	if err := checkLength(r.ctx, res, staged, c.length); err != nil {
		return errors.Wrapf(err, "lcp: %s changed since it was checked", e.path)
	}
	entry, err := ziputil.CreateEntry(r.zw, e.path, method)
	if err != nil {
		return errors.Wrapf(err, "lcp: failed creating %s", e.path)
	}
	_, err = io.CopyN(entry, io.NewSectionReader(staged, 0, c.length), c.length)
	return errors.Wrapf(err, "lcp: failed copying %s", e.path)
}

// errLongerThanChecked is returned by the writers of [checkLength] for the
// bytes beyond the length checked.
var errLongerThanChecked = errors.New("longer than checked")

// checkLength streams a resource to w, failing unless it has the given length:
// writing stops at the first byte beyond it.
func checkLength(ctx context.Context, res fetcher.Resource, w io.Writer, length int64) error {
	lw := &lengthWriter{w: w, remaining: length}
	n, rerr := res.Stream(ctx, lw, 0, 0)
	switch {
	case lw.exceeded:
		return errors.Errorf("more than %d bytes", length)
	case rerr != nil:
		return rerr
	case n != length:
		return errors.Errorf("%d bytes instead of %d", n, length)
	}
	return nil
}

// lengthWriter writes to w at most remaining bytes.
type lengthWriter struct {
	w         io.Writer
	remaining int64
	exceeded  bool
}

func (w *lengthWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		w.exceeded = true
		return 0, errLongerThanChecked
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	return n, err
}
