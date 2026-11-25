package lcp

import (
	"archive/zip"
	"context"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/ziputil"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// ErrNotProtected is returned when trying to decrypt a publication which is
// not protected with LCP.
var ErrNotProtected = errors.New("lcp: publication is not protected with LCP")

// containerEntry is a file of the source container.
type containerEntry struct {
	path string        // decoded path, used as ZIP entry name and resource key
	link manifest.Link // link from the fetcher, with a media type guessed from the extension
}

// containerEntries lists the files of the container, in their original order
// for archives. The fetcher must list all of them (see [fetcher.Exhaustive]):
// those it would leave out, e.g. in a remote HTTP folder, which can't be
// listed, would be missing from the output, or bypass its checks.
func containerEntries(ctx context.Context, f fetcher.Fetcher) ([]containerEntry, error) {
	if e, ok := f.(fetcher.Exhaustive); !ok || !e.Exhaustive() {
		return nil, errors.New("lcp: the files of the asset can't all be listed, which remuxing requires: use a packaged publication, or a directory or cloud storage prefix")
	}
	links, err := f.Links(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: failed listing the container entries")
	}
	entries := make([]containerEntry, 0, len(links))
	for _, link := range links {
		u := link.URL(nil, nil)
		if u == nil {
			continue
		}
		entries = append(entries, containerEntry{path: containerPath(u.Path()), link: link})
	}
	return entries, nil
}

// licenseLinks are links to the LCP licenses of a package, at the paths of
// both formats.
var licenseLinks = []manifest.Link{
	{Href: manifest.MustNewHREFFromString(protection.PathEPUBLCPLicense, false)},
	{Href: manifest.MustNewHREFFromString(protection.PathRWPLCPLicense, false)},
}

// checkLicenseAliases returns an error when one of the links is another name
// of the file of a license, according to the groups of links to the same
// files (see [sameFiles]), e.g. a hard link to it, or another spelling on a
// case-insensitive file system. The license isn't carried over, but this
// other name would be, disclosing the identifier and passphrase hint of its
// recipient. links must include licenseLinks, for licenses no other link
// points to.
func checkLicenseAliases(links []manifest.Link, groups [][]int) error {
	for _, group := range groups {
		var license, alias string
		for _, i := range group {
			if l := links[i]; isLicenseLink(l) {
				license = resourceKey(l.Href)
			} else {
				alias = l.Href.String()
			}
		}
		if license != "" && alias != "" {
			return errors.Errorf("lcp: %s is the same file as %s, an LCP license, which isn't carried over", alias, license)
		}
	}
	return nil
}

// checkEntryLicenseAliases returns an error when an entry of the container is
// another name of the file of one of its licenses: see checkLicenseAliases.
// Otherwise, it returns the identities of the files of the entries, by path,
// to pin them to when they are read (see [remuxer.pins]), so that a file
// replaced since, e.g. by the license, isn't read in place of the one
// checked.
func checkEntryLicenseAliases(ctx context.Context, f fetcher.Fetcher, entries []containerEntry) (map[string]fs.FileInfo, error) {
	return checkSameFiles(ctx, f, entries, nil)
}

// errOutsideLCP is the error of a link outside of the package declaring the
// LCP scheme: decrypting a publication to deliver it can't decrypt it, and
// would drop the license it needs, delivering its ciphertext as if it were
// in the clear.
func errOutsideLCP(href string, link manifest.Link) error {
	kind := "a remote resource"
	if !isRemote(link) {
		kind = "not a resource of the package"
	}
	return errors.Errorf("lcp: %s is declared LCP-encrypted, but is %s, which can't be decrypted", href, kind)
}

// errLicensePathSelected is the error of a resource selected for encryption
// at the path of an LCP license: the remuxes drop the entries at these paths,
// and the injection replaces them, so it would be declared encrypted, but
// never delivered.
func errLicensePathSelected(path string) error {
	return errors.Errorf("lcp: %s is selected for encryption, but is at the path of an LCP license, which the License Server delivers", path)
}

// checkDeclaredEntries returns an error when a resource declared
// LCP-encrypted, by resource key, isn't an entry the decryption decrypts:
// missing, e.g. named differently on a case-insensitive file system, or one
// of the entries it replaces. Its declaration would be dropped with the
// license, leaving the entry holding it encrypted in a package presented as
// unprotected, or the resource missing.
func checkDeclaredEntries(declared map[string]manifest.Encryption, entries []containerEntry, replaced func(path string) bool) error {
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		present[e.path] = true
	}
	for _, key := range slices.Sorted(maps.Keys(declared)) {
		switch {
		case replaced(key):
			return errors.Errorf("lcp: %s is declared LCP-encrypted, but is replaced by the decryption", key)
		case !present[key]:
			return errors.Errorf("lcp: %s is declared LCP-encrypted, but is not in the package", key)
		}
	}
	return nil
}

// checkReservedEntries returns an error when an entry of the container is
// under [pub.ServiceHrefPrefix], the paths of the resources generated by
// publication services: links to it are taken for service links, which
// encryption leaves out.
func checkReservedEntries(entries []containerEntry) error {
	for _, e := range entries {
		if strings.HasPrefix(e.path, pub.ServiceHrefPrefix) {
			return errors.Errorf("lcp: %s is under %s, which is reserved for publication services", e.path, pub.ServiceHrefPrefix)
		}
	}
	return nil
}

// manifestLinks indexes the local links of a manifest (reading order,
// resources and links, with their alternates and children) by resource key.
// The first link to a resource wins.
func manifestLinks(m manifest.Manifest) map[string]manifest.Link {
	links := make(map[string]manifest.Link)
	index := func(l manifest.Link) {
		if !outsidePackage(l) {
			if _, ok := links[resourceKey(l.Href)]; !ok {
				links[resourceKey(l.Href)] = l
			}
		}
	}
	walkLinks(m.ReadingOrder, index)
	walkLinks(m.Resources, index)
	walkLinks(m.Links, index)
	return links
}

// isLicense reports whether an entry is at the path of an LCP license, in an
// EPUB or a Readium Web Publication package. Licenses are never carried over
// by a remux, whichever the format of the package: one at the path of the
// other format would be ambiguous, and would disclose a previous recipient's
// identifier and passphrase hint.
func isLicense(path string) bool {
	return path == protection.PathEPUBLCPLicense || path == protection.PathRWPLCPLicense
}

// remuxer writes a new ZIP container from the entries of a source container.
type remuxer struct {
	ctx context.Context
	f   fetcher.Fetcher
	zw  *zip.Writer
	key ContentKey
	// pins are the identities of the files of the entries, by path, when
	// they were checked, e.g. not to be encrypted ones when copied in the
	// clear, or licenses: the entries are read from these files.
	pins map[string]fs.FileInfo
}

// open opens an entry, pinned to the file it was checked to be, if any.
func (r remuxer) open(e containerEntry) (fetcher.Resource, error) {
	res := r.f.Get(r.ctx, e.link)
	if id, ok := r.pins[e.path]; ok {
		if err := pinTo(res, id); err != nil {
			res.Close()
			return nil, errors.Wrapf(err, "lcp: %s can't be read", e.path)
		}
	}
	return res, nil
}

// copy copies an entry as-is. Deflated archive entries are copied without
// being recompressed, stored entries stay stored.
func (r remuxer) copy(e containerEntry, link manifest.Link) error {
	res, err := r.open(e)
	if err != nil {
		return err
	}
	defer res.Close()
	if err := ziputil.CopyEntry(r.ctx, r.zw, e.path, res, storageMethod(res, link)); err != nil {
		return errors.Wrapf(err, "lcp: failed copying %s", e.path)
	}
	return nil
}

// encrypt writes the ciphertext of an entry, stored since it can't be
// compressed any further.
func (r remuxer) encrypt(e containerEntry, compress bool, opts Options) (manifest.Encryption, error) {
	res, err := r.open(e)
	if err != nil {
		return manifest.Encryption{}, err
	}
	defer res.Close()
	w, err := ziputil.CreateEntry(r.zw, e.path, zip.Store)
	if err != nil {
		return manifest.Encryption{}, err
	}
	var enc manifest.Encryption
	_, rerr := streamThrough(r.ctx, res, func(src io.Reader) (int64, error) {
		var err error
		enc, err = EncryptContent(r.key, src, w, compress, opts.cbcOptions()...)
		return 0, err
	})
	if rerr != nil {
		return enc, errors.Wrapf(rerr, "lcp: failed encrypting %s", e.path)
	}
	return enc, nil
}

// decrypt writes the plaintext of an entry.
func (r remuxer) decrypt(e containerEntry, link manifest.Link, enc manifest.Encryption) error {
	res, err := r.open(e)
	if err != nil {
		return err
	}
	defer res.Close()
	method := ziputil.Method(link.MediaType != nil && link.MediaType.IsPrecompressed())
	w, err := ziputil.CreateEntry(r.zw, e.path, method)
	if err != nil {
		return err
	}
	_, rerr := streamThrough(r.ctx, res, func(src io.Reader) (int64, error) {
		return DecryptContent(r.key, src, w, enc)
	})
	if rerr != nil {
		return errors.Wrapf(rerr, "lcp: failed decrypting %s", e.path)
	}
	return nil
}

// storageMethod returns the compression method of a copied entry: the
// original one for archive entries, otherwise a choice based on the media
// type.
func storageMethod(res fetcher.Resource, link manifest.Link) uint16 {
	if cres, ok := res.(fetcher.CompressedResource); ok {
		if cres.CompressedAs(archive.CompressionMethodStore) {
			return zip.Store
		}
		if cres.CompressedAs(archive.CompressionMethodDeflate) {
			return zip.Deflate
		}
	}
	return ziputil.Method(link.MediaType != nil && link.MediaType.IsPrecompressed())
}

// Size limits of the documents read whole from a container, far above the
// size of legitimate ones, so that a crafted package can't exhaust the
// memory.
const (
	// maxLicenseSize is the size limit of a license, whose size is a few
	// kilobytes.
	maxLicenseSize = 1 << 20 // 1 MiB
	// maxManifestSize is the size limit of a Readium Web Publication
	// manifest, the same as the parser's.
	maxManifestSize = manifest.MaxManifestSize
)

// readEntry reads a whole entry of the container, failing when it is larger
// than limit. It returns nil, without error, when the entry doesn't exist.
func readEntry(ctx context.Context, f fetcher.Fetcher, p string, limit int64) ([]byte, error) {
	href, err := manifest.NewHREFFromString(p, false)
	if err != nil {
		return nil, err
	}
	res := f.Get(ctx, manifest.Link{Href: href})
	defer res.Close()
	if isLicense(p) {
		if _, err := fetcher.PinLocation(res); err != nil {
			return nil, errors.Wrapf(err, "lcp: failed pinning %s", p)
		}
	}
	data, rerr := fetcher.ReadResourceUpTo(ctx, res, limit)
	if rerr != nil {
		if rerr.Code == fetcher.CodeNotFound {
			return nil, nil
		}
		return nil, errors.Wrapf(rerr, "lcp: failed reading %s", p)
	}
	if data == nil {
		return []byte{}, nil
	}
	return data, nil
}

// fileIdentity allows missing resources during planning. If they appear
// later, pinTo refuses to serve them as the files originally checked.
func fileIdentity(res fetcher.Resource) (fs.FileInfo, error) {
	fb, ok := res.(fetcher.FileBacked)
	if !ok {
		return nil, nil
	}
	id, err := fb.FileInfo()
	if err == nil && id == nil {
		return nil, errors.New("file identity is missing")
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fetcher.ErrNotFileBacked) {
		return nil, nil
	}
	return id, err
}

func identifyFiles(ctx context.Context, f fetcher.Fetcher, links []manifest.Link) ([]fs.FileInfo, error) {
	ids := make([]fs.FileInfo, len(links))
	for i, l := range links {
		res := f.Get(ctx, l)
		id, err := fileIdentity(res)
		res.Close()
		if err != nil {
			return nil, errors.Wrapf(err, "lcp: failed identifying the file of %s", l.Href)
		}
		ids[i] = id
	}
	return ids, nil
}

// locationKey compares URLs conservatively for license and encryption alias
// checks: a file-serving origin may use only the decoded path to find a file.
func locationKey(location url.URL) string {
	if location == nil {
		return ""
	}
	u := *location.Normalize().Raw()
	u.Host = strings.ToLower(u.Host)
	switch u.Scheme {
	case "http":
		u.Host = strings.TrimSuffix(u.Host, ":80")
	case "https":
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	u.User = nil
	u.RawPath, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = "", "", false, "", ""
	return u.String()
}

// sameLocations returns the groups of resources, as indexes, read from the
// same URL, given their locations: links spelled differently, such as a
// relative and a root-absolute reference, which a fetcher resolves to the
// same resource. Resources not read from a URL are never grouped.
func sameLocations(locations []string) [][]int {
	byLocation := make(map[string][]int)
	var order []string
	for i, l := range locations {
		if l == "" {
			continue
		}
		if _, ok := byLocation[l]; !ok {
			order = append(order, l)
		}
		byLocation[l] = append(byLocation[l], i)
	}
	var groups [][]int
	for _, l := range order {
		if len(byLocation[l]) > 1 {
			groups = append(groups, byLocation[l])
		}
	}
	return groups
}

// pinTo pins a resource to the file it was identified as when it was checked
// (see [identifyFiles]), so that it fails rather than read another file its
// path names since, e.g. after the file was renamed or replaced by a link to
// a resource which is encrypted. A resource which wasn't identified as a file
// can't be read from one: a file missing when it was checked, whose path is
// taken since, isn't read.
func pinTo(res fetcher.Resource, id fs.FileInfo) error {
	if id == nil {
		if fb, ok := res.(fetcher.FileBacked); ok {
			if _, err := fb.FileInfo(); !errors.Is(err, fetcher.ErrNotFileBacked) {
				return errors.New("its file couldn't be identified when it was checked")
			}
		}
		return nil
	}
	p, ok := res.(fetcher.FilePinner)
	if !ok {
		return errors.New("it can't be pinned to its file")
	}
	return p.PinFile(id)
}

// sameFiles returns the groups of resources, as indexes, which are the same
// file of the file system, e.g. through symbolic or hard links in an
// exploded package, given their identities. Encryption is decided by name:
// two names of a file may otherwise be decided differently, exposing it in
// the clear. Resources which are not files, such as archive entries, are
// never grouped.
//
// The identities must be the ones the resources are then pinned to, so that
// the file checked is the file read.
func sameFiles(ctx context.Context, ids []fs.FileInfo) ([][]int, error) {
	// Where the platform identifies files, e.g. by device and inode, as
	// os.SameFile does, they are grouped by identity.
	byKey := make(map[any][]int)
	var unkeyed []int
	for i, id := range ids {
		if id == nil {
			continue
		}
		if key, ok := fileKey(id); ok {
			byKey[key] = append(byKey[key], i)
		} else {
			unkeyed = append(unkeyed, i)
		}
	}
	var groups [][]int
	for _, group := range byKey {
		if len(group) > 1 {
			groups = append(groups, group)
		}
	}

	// Otherwise, files are compared in pairs, but only with those of the same
	// size and modification time, which they share with their other names.
	// Publications extracted with normalized times may have many: the
	// comparisons are bounded.
	candidates := make(map[[2]int64][]int)
	for _, i := range unkeyed {
		key := [2]int64{ids[i].Size(), ids[i].ModTime().UnixNano()}
		candidates[key] = append(candidates[key], i)
	}
	comparisons := 0
	for _, files := range candidates {
		grouped := make([]bool, len(files))
		for i := range files {
			if grouped[i] {
				continue
			}
			group := []int{files[i]}
			for j := i + 1; j < len(files); j++ {
				if comparisons++; comparisons%4096 == 0 {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
				}
				if comparisons > maxFileComparisons {
					return nil, errors.New("lcp: too many files of the same size and modification time to tell whether some are the same file")
				}
				if !grouped[j] && os.SameFile(ids[files[i]], ids[files[j]]) {
					grouped[j] = true
					group = append(group, files[j])
				}
			}
			if len(group) > 1 {
				groups = append(groups, group)
			}
		}
	}
	for _, group := range groups {
		slices.Sort(group)
	}
	slices.SortFunc(groups, func(a, b []int) int { return a[0] - b[0] })
	return groups, nil
}

// maxFileComparisons bounds the comparisons of files of [sameFiles] on the
// platforms which don't identify them, such as Windows: tens of millions
// take a few seconds. It is a variable for tests.
var maxFileComparisons = 50_000_000

// checkSameFiles returns an error when a file of the container has several
// names, and some of them are encrypted while others are written in the
// clear. Names neither encrypted nor written, e.g. regenerated files, are
// ignored. It returns the identity of the file of each entry, by path, to pin
// the entries written in the clear to.
func checkSameFiles(ctx context.Context, f fetcher.Fetcher, entries []containerEntry, encrypted func(e containerEntry) (encrypt bool, written bool)) (map[string]fs.FileInfo, error) {
	links := make([]manifest.Link, len(entries), len(entries)+len(licenseLinks))
	for i, e := range entries {
		links[i] = e.link
	}
	links = append(links, licenseLinks...)
	ids, err := identifyFiles(ctx, f, links)
	if err != nil {
		return nil, err
	}
	groups, err := sameFiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	if err := checkLicenseAliases(links, groups); err != nil {
		return nil, err
	}
	for _, group := range groups {
		var inClear, ciphered string
		for _, i := range group {
			if encrypted == nil || i >= len(entries) {
				continue
			}
			switch encrypt, written := encrypted(entries[i]); {
			case !written:
			case encrypt:
				ciphered = entries[i].path
			default:
				inClear = entries[i].path
			}
		}
		if inClear != "" && ciphered != "" {
			return nil, errors.Errorf("lcp: %s and %s are the same file, which would be both encrypted and in the clear", ciphered, inClear)
		}
	}
	byPath := make(map[string]fs.FileInfo, len(entries))
	for i, e := range entries {
		byPath[e.path] = ids[i]
	}
	return byPath, nil
}
