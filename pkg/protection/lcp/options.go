package lcp

import (
	"context"
	"crypto/rand"
	"io"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// Options configures the encryption of a publication, and the remuxing
// functions.
type Options struct {
	// Encrypt reports whether the resource of a link is encrypted. When nil,
	// the default selection of the publication's format is used: see
	// [EncryptEPUB] and [EncryptRWP]. It is not consulted for obfuscated
	// fonts, which are never encrypted: they keep their obfuscation. Font
	// obfuscation declared for a resource which is not a font is an error.
	//
	// A resource several links point to is encrypted, or not, as the first
	// of them says, in the reading order then the resources.
	Encrypt func(link manifest.Link) bool

	// Compress reports whether the resource of a link is deflated before
	// being encrypted. When nil, [ShouldCompress] decides from the link's
	// media type. As with Encrypt, it is asked about the first link to a
	// resource.
	Compress func(link manifest.Link) bool

	// Random is the source of the initialization vectors and padding bytes,
	// crypto/rand by default. It is read sequentially, by one goroutine at a
	// time: [EncryptPublication] reads it when the resources of the returned
	// publication are fetched, which may be concurrently, so it serializes
	// the reads, each resource drawing its bytes at once. See [WithRandom]
	// before setting it.
	Random io.Reader

	// ArchiveFactory opens the assets given to the remuxing functions
	// ([EncryptEPUB], [DecryptEPUB], [EncryptRWP], [DecryptRWP]). When nil,
	// archive.NewArchiveFactory() is used, which only opens local files: set
	// it to open assets stored remotely (HTTP, S3, GCS).
	ArchiveFactory archive.ArchiveFactory
}

func (o Options) createFetcher(ctx context.Context, a asset.PublicationAsset) (fetcher.Fetcher, error) {
	factory := o.ArchiveFactory
	if factory == nil {
		factory = archive.NewArchiveFactory()
	}
	f, err := a.CreateFetcher(ctx, asset.Dependencies{ArchiveFactory: factory}, "")
	if err != nil {
		return nil, errors.Wrap(err, "lcp: failed opening asset")
	}
	return f, nil
}

func (o Options) compress(link manifest.Link) bool {
	if o.Compress != nil {
		return o.Compress(link)
	}
	return ShouldCompress(link.MediaType)
}

func (o Options) random() io.Reader {
	if o.Random != nil {
		return o.Random
	}
	return rand.Reader
}

func (o Options) cbcOptions() []CBCOption {
	return []CBCOption{WithRandom(o.random())}
}

// defaultEPUBSelection reports whether an EPUB resource is encrypted by
// default: every publication resource except those reading systems need to
// present the publication before it is unlocked (cover, navigation document,
// NCX, page map).
func defaultEPUBSelection(link manifest.Link) bool {
	if slices.Contains(link.Rels, "cover") || slices.Contains(link.Rels, "contents") {
		return false
	}
	if mt := link.MediaType; mt != nil && mt.Matches(&mediatype.NCX, &pageMapMediaType) {
		return false
	}
	return true
}

var pageMapMediaType, _ = mediatype.NewOfString("application/oebps-page-map+xml")

// defaultRWPSelection returns the default selection of the encrypted
// resources of a Readium Web Publication: every resource except the cover,
// which reading systems display before the publication is unlocked, and
// which the LCP profiles for audiobooks and PDF require to stay in the
// clear.
//
// A resource is the cover when any link to it in the manifest has the
// "cover" relation, e.g. a link of `links` pointing to an image listed in
// `resources`.
func defaultRWPSelection(m manifest.Manifest) func(manifest.Link) bool {
	covers := make(map[string]bool)
	collect := func(l manifest.Link) {
		// A remote cover may share its path with a local resource.
		if slices.Contains(l.Rels, "cover") && !outsidePackage(l) {
			covers[resourceKey(l.Href)] = true
		}
	}
	walkLinks(m.Links, collect)
	walkLinks(m.ReadingOrder, collect)
	walkLinks(m.Resources, collect)
	return func(link manifest.Link) bool {
		return !covers[resourceKey(link.Href)]
	}
}

// obfuscatedFonts returns the resource keys of the obfuscated fonts of a
// manifest: the local resources a link declares font obfuscation for.
// Obfuscated fonts are never encrypted: reading systems deobfuscate them
// with a key derived from the publication identifier, which encryption
// would break, and they are carried over as they are. See [checkFonts].
func obfuscatedFonts(m manifest.Manifest) (map[string]bool, error) {
	fonts := make(map[string]bool)
	collect := func(l manifest.Link) {
		if enc := l.Properties.Encryption(); enc != nil && protection.IsFontObfuscation(enc.Algorithm) && !outsidePackage(l) {
			fonts[resourceKey(l.Href)] = true
		}
	}
	walkLinks(m.ReadingOrder, collect)
	walkLinks(m.Resources, collect)
	walkLinks(m.Links, collect)
	return fonts, checkFonts(m, fonts)
}

// checkFonts returns an error when a link of the manifest to one of the
// given obfuscated resources is not a font.
//
// Obfuscation is no protection: only the first kilobyte of a resource is
// obfuscated, with a key anyone can derive. Only fonts are exempt from
// encryption, since obfuscation is only defined for them: declaring it for
// a chapter must not leave the chapter in the clear.
func checkFonts(m manifest.Manifest, fonts map[string]bool) error {
	var err error
	check := func(l manifest.Link) {
		if err == nil && !outsidePackage(l) && fonts[resourceKey(l.Href)] && (l.MediaType == nil || !l.MediaType.IsFont()) {
			err = notFontError(l)
		}
	}
	walkLinks(m.ReadingOrder, check)
	walkLinks(m.Resources, check)
	walkLinks(m.Links, check)
	return err
}

func notFontError(l manifest.Link) error {
	mt := "no media type"
	if l.MediaType != nil {
		mt = l.MediaType.String()
	}
	return errors.Errorf("lcp: %s is declared as an obfuscated font, but it is not a font (%s): only fonts may stay obfuscated rather than be encrypted", l.Href, mt)
}

// outsidePackage reports whether a link points to something else than a
// resource of the publication package: a remote resource, or a resource
// generated by a publication service (see [pub.ServiceHrefPrefix]), e.g. the
// media overlay alternates the media overlay service puts in place of SMIL
// documents.
//
// A URL template isn't taken for a resource of the package either, since it
// may expand to any of them: [EncryptRWP] refuses those expanding inside the
// package (see [isLocalTemplate]), and [EncryptPublication] doesn't serve
// them.
//
// Every decision keyed by [resourceKey] must check this first: the key of a
// remote link is its path, which may be the one of a resource of the
// package.
func outsidePackage(link manifest.Link) bool {
	if link.Href.IsTemplated() || strings.HasPrefix(link.Href.String(), pub.ServiceHrefPrefix) {
		return true
	}
	return isRemote(link)
}

// isLocalTemplate reports whether a link is a URL template expanding to the
// path of a resource of the package, rather than to a remote resource or a
// publication service.
func isLocalTemplate(link manifest.Link) bool {
	return link.Href.IsTemplated() && !strings.HasPrefix(link.Href.String(), pub.ServiceHrefPrefix) && !isRemote(link)
}

// isRemote reports whether a link points outside of the package, expanding it
// without parameters if it is templated: to an absolute URL, or to a
// network-path reference such as "//cdn.example.com/cover.jpg", which has an
// authority without a scheme (RFC 3986 §4.2) and so is relative, but to
// another host. Links which don't resolve to a URL are taken for remote
// ones, so that they are never taken for resources of the package.
func isRemote(link manifest.Link) bool {
	u := link.URL(nil, nil)
	if u == nil {
		return true
	}
	return url.IsRemoteReference(u)
}

// mapLinks returns a copy of ll where fn was applied to each link, and
// recursively to their alternates and children. The original list is left
// untouched.
func mapLinks(ll manifest.LinkList, fn func(manifest.Link) manifest.Link) manifest.LinkList {
	if ll == nil {
		return nil
	}
	out := make(manifest.LinkList, len(ll))
	for i, link := range ll {
		link.Alternates = mapLinks(link.Alternates, fn)
		link.Children = mapLinks(link.Children, fn)
		out[i] = fn(link)
	}
	return out
}

// walkDeclarations calls fn on the links of a manifest which may declare the
// encryption of a resource: those of its reading order, resources and links,
// with their alternates and children. Decryption, the detection of the
// protection and the validation of protected packages consider the same
// ones.
func walkDeclarations(m manifest.Manifest, fn func(manifest.Link)) {
	walkLinks(m.ReadingOrder, fn)
	walkLinks(m.Resources, fn)
	walkLinks(m.Links, fn)
}

// walkLinks calls fn on each link of ll, and recursively on their alternates
// and children.
func walkLinks(ll manifest.LinkList, fn func(manifest.Link)) {
	for _, link := range ll {
		fn(link)
		walkLinks(link.Alternates, fn)
		walkLinks(link.Children, fn)
	}
}

// withEncryption returns a copy of the properties where the encryption is set,
// or removed when enc is nil.
func withEncryption(p manifest.Properties, enc *manifest.Encryption) manifest.Properties {
	out := make(manifest.Properties, len(p)+1)
	for k, v := range p {
		out[k] = v
	}
	if enc == nil {
		delete(out, "encrypted")
		if len(out) == 0 {
			return nil
		}
	} else {
		out["encrypted"] = enc.ToMap()
	}
	return out
}
