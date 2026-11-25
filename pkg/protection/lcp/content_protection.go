package lcp

import (
	"context"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
)

// ContentProtection unlocks LCP-protected publications in the streamer
// (see streamer.Config.ContentProtections): their resources are decrypted
// on the fly, using the content key of their license.
//
// The license embedded in the publication is used, unless [License] is set.
// It is validated with [Verify], then unlocked with the [UserKeys], then
// with the credentials given to the streamer, which are taken as the user
// passphrase. Passphrases only unlock licenses of the basic profile: see
// [UserKeyFromPassphrase].
//
// Publications which are not protected with LCP are left to other content
// protections, or opened as they are.
type ContentProtection struct {
	// UserKeys are tried in order to unlock the license, before the
	// passphrase given to the streamer.
	UserKeys []UserKey

	// License, when set, is used instead of the license embedded in the
	// publication, e.g. one stored separately.
	License *License

	// Verify validates the license before it is unlocked, usually with a
	// [Verifier]:
	//
	//	Verify: lcp.Verifier{Roots: roots, IsRevoked: lcp.RevokedBy(crl)}.Verify
	//
	// It is required, unless InsecureSkipVerify is set: opening an
	// LCP-protected publication fails otherwise.
	Verify func(*License) error

	// MaxInflatedLength is the maximum length of a deflated resource read
	// whole into memory, once inflated: reading it whole fails beyond, with
	// [ErrResourceTooLarge], whatever the length the publication declares.
	// Streaming it isn't limited. 0 means [DefaultMaxInflatedLength], and a
	// negative value no limit, for trusted publications only.
	MaxInflatedLength int64

	// InsecureSkipVerify allows using licenses without validating them when
	// Verify is nil, e.g. in tests or tools processing licenses known to be
	// genuine. Their signature, rights and provider certificate are then
	// not checked: anyone could have forged or altered them.
	InsecureSkipVerify bool
}

var _ protection.ContentProtection = ContentProtection{}

var errNoVerify = errors.New("lcp: ContentProtection.Verify is required to validate licenses, unless InsecureSkipVerify is set")

// Open implements protection.ContentProtection
func (p ContentProtection) Open(ctx context.Context, a asset.PublicationAsset, f fetcher.Fetcher, credentials string) (*protection.ProtectedAsset, error) {
	// The format is told once, and every step relies on it.
	format, err := assetFormat(ctx, a, f)
	if err != nil {
		return nil, err
	}
	license, err := p.findLicense(ctx, f, format)
	if err != nil || license == nil {
		return nil, err
	}
	switch {
	case p.Verify != nil:
		if err := p.Verify(license); err != nil {
			return nil, err
		}
	case !p.InsecureSkipVerify:
		return nil, errNoVerify
	}
	key, err := p.unlock(license, credentials)
	if err != nil {
		return nil, err
	}
	declared, err := packageDeclarations(ctx, f, format)
	if err != nil {
		return nil, err
	}
	return &protection.ProtectedAsset{
		Scheme: protection.LCP,
		OnCreatePublication: func(b *pub.Builder) error {
			return decryptBuilder(b, key, p.MaxInflatedLength, declared)
		},
	}, nil
}

// declarations are the encryption a package declares, as the remuxes read
// it: for its resources, the LCP-encrypted ones, by resource key, and for
// remote resources, every declaration, merged but not selected (see
// [lcpEncrypted]), by remoteKey.
type declarations struct {
	local  map[string]manifest.Encryption
	remote map[string]manifest.Encryption
}

// packageDeclarations returns the encryption a package declares, from its
// EPUB encryption.xml, or its Readium Web Publication manifest. The parsed
// manifest doesn't hold all of it: the EPUB parser only declares the
// resources of the package document, and the manifest parser drops the links
// without a media type, remote ones included.
func packageDeclarations(ctx context.Context, f fetcher.Fetcher, format packageFormat) (declarations, error) {
	switch format {
	case epubFormat:
		document, err := readEntry(ctx, f, protection.PathEPUBEncryption, protection.MaxMetadataSize)
		if err != nil {
			return declarations{}, err
		}
		local, remote, err := epubDeclarations(document)
		if err != nil {
			return declarations{}, err
		}
		local, err = lcpEncrypted(local)
		return declarations{local: local, remote: remote}, err
	case rwpFormat:
		raw, err := readEntry(ctx, f, rwpManifestPath, maxManifestSize)
		if err != nil || raw == nil {
			return declarations{}, err
		}
		declared, outside, err := rwpDeclarations(raw)
		if err != nil {
			return declarations{}, err
		}
		local, err := lcpEncrypted(declared)
		if err != nil {
			return declarations{}, err
		}
		remote := make(map[string]manifest.Encryption)
		for _, d := range outside {
			if isRemote(d.link) {
				if err := declare(remote, remoteKey(d.link), d.enc); err != nil {
					return declarations{}, err
				}
			}
		}
		return declarations{local: local, remote: remote}, nil
	}
	return declarations{}, nil
}

// findLicense returns the license of an LCP-protected publication, or nil
// when the publication is not protected with LCP.
func (p ContentProtection) findLicense(ctx context.Context, f fetcher.Fetcher, format packageFormat) (*License, error) {
	embedded, err := readLicense(ctx, f, format)
	if errors.Is(err, ErrNoLicense) {
		protected, perr := isLCPProtected(ctx, f, format)
		if perr != nil || !protected {
			return nil, perr
		}
	}
	if p.License != nil {
		// The license is validated and unlocked from its own copy, parsed
		// from its document: the caller's could be changed meanwhile.
		return p.License.parsedCopy()
	}
	if errors.Is(err, ErrNoLicense) {
		return nil, errors.Wrap(err, "the publication is protected with LCP but holds no license")
	}
	return embedded, err
}

// unlock returns the content key of the license, trying the user keys then
// the passphrase.
func (p ContentProtection) unlock(license *License, passphrase string) (ContentKey, error) {
	for _, uk := range p.UserKeys {
		key, err := license.ContentKey(uk)
		if !errors.Is(err, ErrWrongUserKey) {
			return key, err
		}
	}
	if passphrase != "" {
		return license.ContentKeyFromPassphrase(passphrase)
	}
	if len(p.UserKeys) > 0 {
		return nil, ErrWrongUserKey
	}
	return nil, errors.Wrapf(ErrNoCredentials, "passphrase hint: %q", license.Encryption.UserKey.TextHint)
}

// isLCPProtected reports whether a publication without an embedded license
// declares LCP-encrypted resources, in its EPUB encryption.xml or Readium Web
// Publication manifest, given its format.
func isLCPProtected(ctx context.Context, f fetcher.Fetcher, format packageFormat) (bool, error) {
	switch format {
	case epubFormat:
		scheme, _, err := protection.IdentifyEPUBProtection(ctx, f)
		return scheme == protection.LCP, err
	case rwpFormat:
		// Only an invalid manifest means that the publication is not a
		// Readium Web Publication package. A manifest which can't be read,
		// such as one exceeding its size limit, is an error: the parser
		// would read it whole otherwise.
		raw, err := readEntry(ctx, f, rwpManifestPath, maxManifestSize)
		if err != nil || raw == nil {
			return false, err
		}
		if _, err := parseRWPManifest(raw); err != nil {
			return false, nil
		}
		// The declarations are read from the raw manifest, as the remuxes
		// do: the parsed one drops the links without a media type. Remote
		// resources declared LCP-encrypted make the publication protected
		// as well, though they aren't resources of the package.
		declared, outside, err := rwpDeclarations(raw)
		if err != nil {
			return false, err
		}
		local, err := lcpEncrypted(declared)
		if err != nil || len(local) > 0 {
			return len(local) > 0, err
		}
		for _, d := range outside {
			if requiresLCPLicense(&d.enc) {
				return true, nil
			}
		}
		return false, nil
	}
	return false, nil
}
