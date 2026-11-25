// Package lcp implements the content-level building blocks of Readium LCP:
// encrypting and decrypting publication resources with a content key, and
// reading and writing the metadata describing them.
//
// Resources are encrypted with AES-256-CBC: each one is stored as a random
// initialization vector followed by the ciphertext of its content, padded
// following the W3C XML Encryption scheme, optionally deflated first. The
// output is interoperable with the LCP specification, readium-lcp-server's
// lcpencrypt, and LCP reading systems.
//
// # Layers
//
// Primitives work on any 256-bit AES key: [Encrypt], [Decrypt],
// [NewEncryptWriter], [NewDecryptReader].
//
// Resources: [EncryptContent] and [DecryptContent] handle compression and
// produce the [manifest.Encryption] describing a resource. [Decryptor] is a
// [fetcher.ResourceTransformer] serving decrypted resources, with efficient
// random access to the ones that were not compressed.
//
// Publications, for the parse → transform → package pipeline:
// [EncryptPublication] and [DecryptPublication] return a publication whose
// resources and manifest are encrypted or decrypted, ready to be written by
// any packager, such as pkg/packager/epub. [DecryptBuilder] does the same
// from streamer.Config.OnCreatePublication, keeping the publication services.
//
// Containers, protected or unprotected in place: [EncryptEPUB],
// [DecryptEPUB], [EncryptRWP] and [DecryptRWP] remux an EPUB or a Readium Web
// Publication package, copying everything else byte for byte. This is the
// equivalent of lcpencrypt, without the License Server notification.
// [InjectLicense] embeds a license into a protected package, as a License
// Server does when delivering it.
//
// # Licenses
//
// A [License] grants a user access to a publication: it holds the content
// key, encrypted with the user key. [ReadLicense] and [ParseLicense] read
// it, [License.ContentKey] and [License.ContentKeyFromPassphrase] unlock it,
// and a [Verifier] validates it the way reading systems do: signature,
// provider certificate, revocation (see [RevokedBy]) and rights.
//
// [ContentProtection] plugs all of this into the streamer, which then opens
// LCP-protected publications given their user passphrase or user key. It
// requires a function validating licenses, unless their validation is
// explicitly skipped.
//
// # Untrusted input
//
// Publications and licenses are handled as untrusted input:
//
//   - Lengths declared by publications are not trusted to allocate memory.
//     The content of a resource can't exceed its declared original length,
//     and a deflated resource can't be read into memory, whole or in part,
//     beyond [DefaultMaxInflatedLength], while streaming it isn't limited.
//     Streamed ranges are never held in memory.
//   - A license is refused when its fields could be read differently from
//     what its signature covers, or when its canonical form exceeds 1 MiB,
//     and doesn't verify once its fields were changed. Without a revocation
//     list signed by its provider's issuer, whose key is taken from the
//     verified certificate chain rather than its name, and fully processed
//     (complete, without critical extensions), whether the provider
//     certificate was revoked is unknown rather than assumed: see
//     [RevokedBy].
//   - Resources are identified by the path the fetchers resolve. Encryption
//     fails rather than leaving a selected resource in the clear under
//     another name: a manifest link which can't be classified, such as a URL
//     template expanding to paths of the package, an entry under the paths
//     reserved for publication services ("~readium/", see
//     pub.ServiceHrefPrefix), or another name of the same file in an
//     exploded publication. Remote links, including network-path references
//     such as "//cdn.example.com/cover.jpg", never stand for the resources
//     of the package sharing their path, whether to encrypt or decrypt them.
//   - HTTP-backed publication resources are served from the URL checked for
//     that manifest entry. Their reads reject redirects, including through
//     the EPUB font transformer. The origin must serve the intended content
//     at each URL. Server-side aliases cannot be identified from URLs alone.
//   - A license doesn't make a package protected: [InjectLicense] requires
//     resources declared LCP-encrypted, all in the package, with the size
//     of their ciphertexts, and writes what it checked. A license is only
//     read from the path of the format of its package, and its JSON can't
//     nest more than 1000 levels deep.
//   - The DRM metadata files of EPUB containers, such as encryption.xml, are
//     read up to protection.MaxMetadataSize, and manifest.json up to 32 MiB,
//     including when detecting whether a publication is protected.
//   - Font obfuscation, which is no protection, only exempts fonts from
//     encryption: declaring it for another resource is an error.
//   - Decrypting a whole resource checks the end of its ciphertext, padding
//     included, even when the inflater needs no more of it. This detects
//     corruption, not tampering: publications are not authenticated.
//   - Exploded publications are confined to their directory, whatever the
//     symbolic links they hold. The resources served or copied in the clear
//     are pinned to the files checked not to be encrypted ones: a file
//     renamed or linked to their path afterwards isn't read.
//   - Provider certificates whose key usage doesn't allow signatures can't
//     sign licenses.
//
// # Scope
//
// Licenses are read, not issued: generating them, and their lifecycle
// (status documents, renewals, returns), are the License Server's job.
// Content keys are given by the caller, who is responsible for storing them,
// typically in a License Server. Every LCP encryption profile is recognized
// (see [Profile]), and licenses of other profiles are refused, but user keys
// can only be derived from passphrases for the basic profile: production
// profiles require EDRLab-certified libraries.
//
// The code is derived from readium-lcp-server (Copyright (c) 2016 Readium
// Foundation, BSD-3-Clause), rewritten to stream resources and to build on
// the toolkit's assets, fetchers, parsers and manifest model.
package lcp

import (
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// Referenced from the documentation.
var (
	_ manifest.Encryption
	_ fetcher.ResourceTransformer
)
