package lcp

import (
	"errors"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/protection"
)

const (
	// Scheme identifies LCP-encrypted resources in [manifest.Encryption].
	Scheme = protection.SchemeLCP
	// AlgorithmAES256CBC is the algorithm LCP uses to encrypt publication
	// resources: AES-256 in CBC mode, with W3C XML Encryption padding.
	AlgorithmAES256CBC = "http://www.w3.org/2001/04/xmlenc#aes256-cbc"
	// CompressionDeflate is the [manifest.Encryption] compression value of
	// resources deflated before being encrypted.
	CompressionDeflate = protection.CompressionDeflate
)

var (
	// ErrInvalidKey is returned when a key is not a 256-bit AES key.
	ErrInvalidKey = errors.New("lcp: key must be 32 bytes long")
	// ErrInvalidCiphertext is returned when a ciphertext is too short or its
	// length is not a multiple of the AES block size.
	ErrInvalidCiphertext = errors.New("lcp: invalid ciphertext length")
	// ErrInvalidPadding is returned when the padding of a decrypted
	// ciphertext is invalid, which usually means that the key is wrong or the
	// data corrupted.
	//
	// The padding is the only integrity check of AES-CBC, so a wrong key goes
	// undetected about once in sixteen attempts: decryption then "succeeds"
	// with garbage.
	ErrInvalidPadding = errors.New("lcp: invalid padding (wrong key or corrupted data)")
	// ErrResourceTooLarge is returned when reading into memory, whole or in
	// part, a deflated resource which inflates beyond the maximum inflated
	// length: see [DefaultMaxInflatedLength]. It can be streamed instead.
	ErrResourceTooLarge = fetcher.ErrResourceTooLarge
	// ErrAlreadyProtected is returned when trying to encrypt a publication or
	// resource which is already protected.
	ErrAlreadyProtected = errors.New("lcp: publication is already protected")

	// ErrInvalidLicense is returned when a License Document is malformed or
	// misses required information.
	ErrInvalidLicense = errors.New("lcp: invalid license")
	// ErrNoLicense is returned when a publication holds no license.
	ErrNoLicense = errors.New("lcp: no license found")
	// ErrUnknownProfile is returned when the encryption profile of a license
	// is not one of LCP's: see [Profile].
	ErrUnknownProfile = errors.New("lcp: unknown encryption profile")
	// ErrUnsupportedProfile is returned when deriving a user key from a
	// passphrase for a production profile, rather than the basic profile.
	// Production profiles require EDRLab-certified libraries: their user key
	// must be obtained by other means.
	ErrUnsupportedProfile = errors.New("lcp: unsupported encryption profile")
	// ErrWrongUserKey is returned when a user key, or the passphrase it was
	// derived from, doesn't unlock a license.
	ErrWrongUserKey = errors.New("lcp: wrong user key or passphrase")
	// ErrNoCredentials is returned when a license can't be unlocked because
	// neither a passphrase nor a user key was provided.
	ErrNoCredentials = errors.New("lcp: a passphrase or user key is required")
	// ErrInvalidSignature is returned when the signature of a license is
	// missing or doesn't match its content.
	ErrInvalidSignature = errors.New("lcp: invalid license signature")
	// ErrInvalidCertificate is returned when the provider certificate of a
	// license is not trusted, or was not valid when the license was issued.
	ErrInvalidCertificate = errors.New("lcp: invalid provider certificate")
	// ErrCertificateRevoked is returned when the provider certificate of a
	// license was revoked by its authority.
	ErrCertificateRevoked = errors.New("lcp: provider certificate revoked")
	// ErrNoRevocationList is returned by the functions of [RevokedBy] when
	// none of their lists was issued by the issuer of a certificate, so that
	// whether it was revoked is unknown.
	ErrNoRevocationList = errors.New("lcp: no certificate revocation list of the issuer")
	// ErrLicenseNotStarted is returned when the rights of a license start in
	// the future.
	ErrLicenseNotStarted = errors.New("lcp: license is not valid yet")
	// ErrLicenseExpired is returned when the rights of a license have ended.
	ErrLicenseExpired = errors.New("lcp: license has expired")
)

// IsLCPEncrypted reports whether the encryption describes a resource
// encrypted with an LCP content key, which this package can decrypt.
//
// The algorithm is checked as well as the scheme, so that other resources
// declared in the encryption metadata of an LCP publication, such as
// obfuscated fonts, are never mistaken for LCP-encrypted ones.
func IsLCPEncrypted(enc *manifest.Encryption) bool {
	return enc != nil && enc.Scheme == Scheme && enc.Algorithm == AlgorithmAES256CBC
}
