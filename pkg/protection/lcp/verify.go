package lcp

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"math/big"
	"reflect"
	"time"

	"github.com/pkg/errors"
)

// VerifySignature checks that the license was signed by its provider, and
// that the provider certificate chains up to one of the given roots, such as
// EDRLab's LCP root certificates. It returns [ErrInvalidCertificate] when roots
// is nil: see [License.VerifyIntegrity] to check the signature alone.
//
// The certificate must have been valid when the license was last updated (or
// issued): a license stays valid after its provider certificate expires, and
// a license refreshed by the License Server is signed again with the current
// certificate. Certificate revocation is not checked: see [Verifier].
//
// The signature covers the document the license was parsed from: it doesn't
// verify when the License fields were changed since.
func (l *License) VerifySignature(roots *x509.CertPool) error {
	if roots == nil {
		return errors.Wrap(ErrInvalidCertificate, "no root certificates")
	}
	_, _, err := l.verifySignature(roots)
	return err
}

// VerifyIntegrity checks the signature of the license against the provider
// certificate it embeds, without checking the certificate. This proves that
// the license was not altered since it was signed, not who signed it: anyone
// can sign a license with a certificate of their own.
func (l *License) VerifyIntegrity() error {
	_, _, err := l.verifySignature(nil)
	return err
}

// verifySignature checks the signature of the license, and the provider
// certificate when roots is not nil. It returns the provider certificate,
// and the certificate of its issuer from the verified chain when roots is
// not nil.
func (l *License) verifySignature(roots *x509.CertPool) (provider, issuer *x509.Certificate, err error) {
	if l == nil {
		return nil, nil, errNilLicense
	}
	sig := l.Signature
	if sig == nil || len(sig.Value) == 0 {
		return nil, nil, errors.Wrap(ErrInvalidSignature, "missing signature")
	}
	if err := l.checkUnmodified(); err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(sig.Certificate)
	if err != nil {
		return nil, nil, errors.Wrap(ErrInvalidCertificate, err.Error())
	}

	if roots != nil {
		at := l.Issued
		if l.Updated != nil {
			at = *l.Updated
		}
		chains, err := cert.Verify(x509.VerifyOptions{
			Roots:       roots,
			CurrentTime: at,
			// Provider certificates are not restricted to a specific usage.
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		})
		if err != nil {
			return nil, nil, errors.Wrap(ErrInvalidCertificate, err.Error())
		}
		// The issuer is the certificate whose key signed the provider
		// certificate, the provider certificate itself when it is a root.
		issuer = chains[0][min(1, len(chains[0])-1)]
		// The chain verification ignores the key usage of the provider
		// certificate: one restricted e.g. to key encipherment can't sign
		// licenses (RFC 5280 §4.2.1.3).
		if cert.KeyUsage != 0 && cert.KeyUsage&(x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment) == 0 {
			return nil, nil, errors.Wrap(ErrInvalidCertificate, "the key usage of the provider certificate doesn't allow signatures")
		}
	}

	// RSA and ECDSA sign the SHA-256 digest of the canonical form, while
	// Ed25519 signs the canonical form itself.
	var verify func(message []byte) bool
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		if sig.Algorithm != SignatureAlgorithmRSA {
			return nil, nil, errors.Wrapf(ErrInvalidSignature, "algorithm %q doesn't match an RSA certificate", sig.Algorithm)
		}
		verify = func(message []byte) bool {
			hash := sha256.Sum256(message)
			return rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig.Value) == nil
		}
	case *ecdsa.PublicKey:
		if sig.Algorithm != SignatureAlgorithmECDSA {
			return nil, nil, errors.Wrapf(ErrInvalidSignature, "algorithm %q doesn't match an ECDSA certificate", sig.Algorithm)
		}
		verify = func(message []byte) bool {
			hash := sha256.Sum256(message)
			return verifyECDSA(pub, hash[:], sig.Value)
		}
	case ed25519.PublicKey:
		if sig.Algorithm != SignatureAlgorithmEd25519 {
			return nil, nil, errors.Wrapf(ErrInvalidSignature, "algorithm %q doesn't match an Ed25519 certificate", sig.Algorithm)
		}
		// Pure Ed25519 signs the message itself, hashing it internally.
		verify = func(message []byte) bool {
			return ed25519.Verify(pub, message, sig.Value)
		}
	default:
		return nil, nil, errors.Wrapf(ErrInvalidCertificate, "unsupported public key type %T", cert.PublicKey)
	}

	forms, err := l.canonicalForms()
	if err != nil {
		return nil, nil, err
	}
	for _, form := range forms {
		if verify(form) {
			return cert, issuer, nil
		}
	}
	return nil, nil, ErrInvalidSignature
}

// errNilLicense is the error of verifying a nil license, e.g. decoded from
// JSON null into a *License.
var errNilLicense = errors.Wrap(ErrInvalidLicense, "no license")

// checkUnmodified returns [ErrInvalidSignature] when the fields of a parsed
// license no longer hold the document it was parsed from: the signature
// covers that document, not the fields, from which the provider certificate
// is checked, and the rights and content key are read. The signature must be
// the one of the document as well, which is what [License.Raw] returns and
// [InjectLicense] delivers: one attached afterwards would be verified in its
// place.
func (l *License) checkUnmodified() error {
	if l.raw == nil {
		return nil // the canonical form is computed from the fields
	}
	signed, err := ParseLicense(l.raw)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(signed, l) {
		return errors.Wrap(ErrInvalidSignature, "the license was modified after being parsed")
	}
	return nil
}

// verifyECDSA verifies an ECDSA signature encoded as the concatenation of r
// and s, each padded to the size of the curve, as XML Signature specifies
// and the LCP License Server produces. ASN.1-encoded signatures are accepted
// as well.
func verifyECDSA(pub *ecdsa.PublicKey, hash []byte, sig []byte) bool {
	size := (pub.Curve.Params().BitSize + 7) / 8
	if len(sig) == 2*size {
		r := new(big.Int).SetBytes(sig[:size])
		s := new(big.Int).SetBytes(sig[size:])
		if ecdsa.Verify(pub, hash, r, s) {
			return true
		}
	}
	return ecdsa.VerifyASN1(pub, hash, sig)
}

// canonicalJSON returns the canonical form of the license defined by the LCP
// specification, which the signature covers: the original document without
// its signature, serialized as [canonicalize] does.
//
// For a license which was not parsed, it is computed from the License
// fields.
func (l *License) canonicalJSON() ([]byte, error) {
	forms, err := l.canonicalForms()
	if err != nil {
		return nil, err
	}
	return forms[0], nil
}

// canonicalForms returns the canonical forms of the license a signature may
// cover: the one of the LCP specification, then the one readium-lcp-server
// produces, when it differs. Both are serializations of the same document,
// so accepting either doesn't change what a valid signature vouches for.
//
// They are derived from the document the license was parsed from, without
// its signature, or from the License fields for a license which was not
// parsed.
func (l *License) canonicalForms() ([][]byte, error) {
	data := l.raw
	if data == nil {
		unsigned := *l
		unsigned.Signature = nil
		var err error
		if data, err = json.Marshal(unsigned); err != nil {
			return nil, err
		}
	}
	doc, err := decodeDocument(data)
	if err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, err.Error())
	}
	delete(doc, "signature")
	canonical, err := canonicalize(doc)
	if err != nil {
		return nil, err
	}
	legacy, err := canonicalizeLegacy(doc)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(canonical, legacy) {
		return [][]byte{canonical}, nil
	}
	return [][]byte{canonical, legacy}, nil
}

// Verifier validates licenses the way reading systems do, e.g. for
// [ContentProtection.Verify]. It checks that a license was signed by a
// provider certified by one of the Roots, that the provider certificate is
// not revoked, and that the license is within its rights period:
//
//	lcp.ContentProtection{Verify: lcp.Verifier{Roots: roots, IsRevoked: lcp.RevokedBy(crl)}.Verify}
type Verifier struct {
	// Roots are the certificates of the authorities certifying the
	// providers, such as EDRLab's LCP root certificates. They are required.
	Roots *x509.CertPool

	// IsRevoked reports whether the certificate of a provider was revoked,
	// e.g. [RevokedBy] the certificate revocation list of the authority.
	// It receives the certificate of the issuer of the provider certificate
	// as well, from the chain verified against the Roots, which identifies
	// the authority by its key: renewed roots keep their name, with a new
	// key. The LCP specification requires reading systems to check it. It is
	// required, unless InsecureSkipRevocation is set.
	IsRevoked func(provider, issuer *x509.Certificate) (bool, error)

	// InsecureSkipRevocation accepts licenses without checking whether the
	// certificate of their provider was revoked, when IsRevoked is nil: a
	// provider whose key was compromised remains trusted.
	InsecureSkipRevocation bool

	// StrictProfile requires licenses to declare the algorithms their
	// encryption profile names, as the LCP specification does. Otherwise,
	// the signature algorithm only has to match the key of the provider
	// certificate, like EDRLab's License Server checks it: it signs licenses
	// with ECDSA when given an ECDSA key, whatever their profile.
	//
	// The specification only names the algorithms of the Basic profile:
	// AES-256-CBC for the content key, SHA-256 for the user key, and RSA with
	// SHA-256 for the signature. The algorithms of other profiles, such as
	// EDRLab's production profiles, are not checked.
	StrictProfile bool
}

// Verify validates a license: see [Verifier]. It returns
// [ErrUnknownProfile], [ErrInvalidSignature], [ErrInvalidCertificate],
// [ErrCertificateRevoked], [ErrLicenseNotStarted] or [ErrLicenseExpired] when
// the license is not valid.
func (v Verifier) Verify(l *License) error {
	if l == nil {
		return errNilLicense
	}
	if v.Roots == nil {
		return errors.Wrap(ErrInvalidCertificate, "no root certificates")
	}
	if v.IsRevoked == nil && !v.InsecureSkipRevocation {
		return errors.New("lcp: Verifier.IsRevoked is required to check the revocation of provider certificates, unless InsecureSkipRevocation is set")
	}
	// The profile names the algorithm of the signature.
	if err := l.Encryption.Profile.Validate(); err != nil {
		return err
	}
	cert, issuer, err := l.verifySignature(v.Roots)
	if err != nil {
		return err
	}
	if v.StrictProfile {
		if err := checkProfileAlgorithms(l); err != nil {
			return err
		}
	}
	if v.IsRevoked != nil {
		revoked, err := v.IsRevoked(cert, issuer)
		if err != nil {
			return errors.Wrap(err, "lcp: failed checking the revocation of the provider certificate")
		}
		if revoked {
			return errors.Wrapf(ErrCertificateRevoked, "provider certificate %s", cert.Subject)
		}
	}
	return l.CheckRights(time.Now())
}

// checkProfileAlgorithms returns an error when a license declares other
// algorithms than the ones its encryption profile names, for the profiles
// whose algorithms are known: see [Verifier.StrictProfile].
func checkProfileAlgorithms(l *License) error {
	if l.Encryption.Profile != ProfileBasic {
		return nil
	}
	if a := l.Encryption.ContentKey.Algorithm; a != AlgorithmAES256CBC {
		return errors.Wrapf(ErrInvalidLicense, "content key algorithm %q isn't the one of the Basic profile, %q", a, AlgorithmAES256CBC)
	}
	if a := l.Encryption.UserKey.Algorithm; a != AlgorithmSHA256 {
		return errors.Wrapf(ErrInvalidLicense, "user key algorithm %q isn't the one of the Basic profile, %q", a, AlgorithmSHA256)
	}
	if a := l.Signature.Algorithm; a != SignatureAlgorithmRSA {
		return errors.Wrapf(ErrInvalidSignature, "algorithm %q isn't the one of the Basic profile, %q", a, SignatureAlgorithmRSA)
	}
	return nil
}

// RevokedBy returns an IsRevoked function for [Verifier], looking up the
// certificates of providers in certificate revocation lists, such as the one
// EDRLab maintains for LCP. A certificate is revoked when a list signed by
// its issuer holds its serial number.
//
// Issuers are identified by their key, which must have signed the list:
// names don't identify them, since renewed roots keep their name with a new
// key, and identifiers of keys are optional (EDRLab's provider certificates
// don't have one). So lists don't need to be checked beforehand: a list
// signed by another key, or forged, doesn't apply.
//
// When none of the lists was issued by the issuer of a certificate, whether
// it was revoked is unknown: the function returns [ErrNoRevocationList], and
// [Verifier.Verify] fails. Accepting such certificates is a policy decision
// for the caller to make explicitly, e.g. while the list of their issuer
// can't be downloaded:
//
//	isRevoked := lcp.RevokedBy(lists...)
//	verifier := lcp.Verifier{Roots: roots, IsRevoked: func(provider, issuer *x509.Certificate) (bool, error) {
//		revoked, err := isRevoked(provider, issuer)
//		if errors.Is(err, lcp.ErrNoRevocationList) {
//			return false, nil // or only while offline
//		}
//		return revoked, err
//	}}
//
// Only the lists this function fully processes apply: complete lists without
// critical extensions. Delta lists (RFC 5280 §5.2.4) only hold the changes
// since a base list, and may unrevoke certificates (removeFromCRL), lists
// with an issuing distribution point (§5.2.5) may only cover some
// certificates or reasons, and RFC 5280 forbids using lists with critical
// extensions a reader can't process, on the list or on its entries: such
// lists don't apply at all, neither to revoke a certificate nor to tell it
// isn't revoked. EDRLab's list for LCP applies.
//
// The lists are parsed again from their DER, so that changing or replacing
// them afterwards, e.g. when refreshing them, doesn't affect the function.
// Lists which weren't parsed with [x509.ParseRevocationList] don't apply.
// Keeping them current is up to the caller: the LCP specification asks
// reading systems to update them regularly when online, without preventing
// users from opening publications offline.
func RevokedBy(lists ...*x509.RevocationList) func(provider, issuer *x509.Certificate) (bool, error) {
	snapshots := make([]revocationList, 0, len(lists))
	for _, list := range lists {
		if list == nil {
			continue
		}
		if snapshot, err := newRevocationList(list.Raw); err == nil {
			snapshots = append(snapshots, snapshot)
		}
	}
	return func(provider, issuer *x509.Certificate) (bool, error) {
		if issuer == nil {
			return false, errors.Wrapf(ErrNoRevocationList, "unknown issuer of %s", provider.Subject)
		}
		covered := false
		for _, list := range snapshots {
			if !list.signedBy(issuer) {
				continue
			}
			if list.serials[provider.SerialNumber.String()] {
				return true, nil
			}
			covered = true
		}
		if !covered {
			return false, errors.Wrapf(ErrNoRevocationList, "issuer %s", provider.Issuer)
		}
		return false, nil
	}
}

// Extensions of certificate revocation lists covering only some of the
// certificates of their issuer, which may be marked non-critical by mistake.
var (
	oidDeltaCRLIndicator        = asn1.ObjectIdentifier{2, 5, 29, 27} // RFC 5280 §5.2.4
	oidIssuingDistributionPoint = asn1.ObjectIdentifier{2, 5, 29, 28} // RFC 5280 §5.2.5
)

// reasonRemoveFromCRL is the reason code of the entries of delta lists
// unrevoking a certificate (RFC 5280 §5.3.1).
const reasonRemoveFromCRL = 8

// revocationList is a private copy of a certificate revocation list, whose
// revoked serial numbers are indexed from the same parsed list whose
// signature is checked.
type revocationList struct {
	list    *x509.RevocationList
	serials map[string]bool
}

// newRevocationList parses a certificate revocation list, failing for the
// lists [RevokedBy] doesn't fully process.
func newRevocationList(der []byte) (revocationList, error) {
	list, err := x509.ParseRevocationList(bytes.Clone(der))
	if err != nil {
		return revocationList{}, err
	}
	for _, ext := range list.Extensions {
		switch {
		case ext.Id.Equal(oidDeltaCRLIndicator):
			return revocationList{}, errors.New("delta list")
		case ext.Id.Equal(oidIssuingDistributionPoint):
			return revocationList{}, errors.New("list with an issuing distribution point")
		case ext.Critical:
			return revocationList{}, errors.Errorf("unsupported critical extension %s", ext.Id)
		}
	}
	l := revocationList{list: list, serials: make(map[string]bool, len(list.RevokedCertificateEntries))}
	for _, entry := range list.RevokedCertificateEntries {
		if entry.ReasonCode == reasonRemoveFromCRL {
			return revocationList{}, errors.New("entry removed from the list")
		}
		for _, ext := range entry.Extensions {
			if ext.Critical {
				return revocationList{}, errors.Errorf("unsupported critical entry extension %s", ext.Id)
			}
		}
		l.serials[entry.SerialNumber.String()] = true
	}
	return l, nil
}

// signedBy reports whether the list was signed by the key of issuer.
func (l revocationList) signedBy(issuer *x509.Certificate) bool {
	// The name is compared first, as it is cheaper than the signature.
	return bytes.Equal(l.list.RawIssuer, issuer.RawSubject) && l.list.CheckSignatureFrom(issuer) == nil
}

// CheckRights returns [ErrLicenseNotStarted] or [ErrLicenseExpired] when the
// given time is outside of the license's rights period.
//
// It trusts the License fields: check the signature first, as [Verifier]
// does, which fails when they were changed since the license was parsed.
//
// The end of a license can be extended by its License Server, which
// publishes the current dates in its status document: this only checks
// the license itself.
func (l *License) CheckRights(now time.Time) error {
	if l == nil {
		return errNilLicense
	}
	if start := l.Rights.Start; start != nil && now.Before(*start) {
		return errors.Wrapf(ErrLicenseNotStarted, "license starts on %s", start.Format(time.RFC3339))
	}
	if end := l.Rights.End; end != nil && now.After(*end) {
		return errors.Wrapf(ErrLicenseExpired, "license ended on %s", end.Format(time.RFC3339))
	}
	return nil
}
