package lcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
)

// Profile identifies an LCP encryption profile: the algorithms protecting a
// publication and its license, and how the user key is derived from the user
// passphrase.
//
// The specification's registry of profiles lists the basic profile and the
// production profile 1.0. The Readium toolkits and License Server support the
// production profiles 2.0 to 2.9 as well.
//
// See https://readium.org/lcp-specs/registries/profiles.
type Profile string

const (
	// ProfileBasic is the basic encryption profile, whose user key is the
	// SHA-256 hash of the user passphrase. It is meant for testing.
	ProfileBasic Profile = "http://readium.org/lcp/basic-profile"

	// The production encryption profiles, defined by EDRLab. How their user
	// key is derived isn't public: see [UserKeyFromPassphrase].
	Profile10 Profile = "http://readium.org/lcp/profile-1.0"
	Profile20 Profile = "http://readium.org/lcp/profile-2.0"
	Profile21 Profile = "http://readium.org/lcp/profile-2.1"
	Profile22 Profile = "http://readium.org/lcp/profile-2.2"
	Profile23 Profile = "http://readium.org/lcp/profile-2.3"
	Profile24 Profile = "http://readium.org/lcp/profile-2.4"
	Profile25 Profile = "http://readium.org/lcp/profile-2.5"
	Profile26 Profile = "http://readium.org/lcp/profile-2.6"
	Profile27 Profile = "http://readium.org/lcp/profile-2.7"
	Profile28 Profile = "http://readium.org/lcp/profile-2.8"
	Profile29 Profile = "http://readium.org/lcp/profile-2.9"
)

// IsProduction reports whether the profile is one of the production profiles,
// rather than the basic profile or an unknown one.
func (p Profile) IsProduction() bool {
	switch p {
	case Profile10, Profile20, Profile21, Profile22, Profile23, Profile24,
		Profile25, Profile26, Profile27, Profile28, Profile29:
		return true
	}
	return false
}

// Validate returns [ErrUnknownProfile] when the profile is neither the basic
// profile nor a production profile.
func (p Profile) Validate() error {
	if p != ProfileBasic && !p.IsProduction() {
		return fmt.Errorf("%w %q", ErrUnknownProfile, string(p))
	}
	return nil
}

const (
	// AlgorithmSHA256 is the algorithm deriving the user key from the user
	// passphrase.
	AlgorithmSHA256 = "http://www.w3.org/2001/04/xmlenc#sha256"
	// SignatureAlgorithmRSA identifies licenses signed with RSA (PKCS #1
	// v1.5) and SHA-256.
	SignatureAlgorithmRSA = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	// SignatureAlgorithmECDSA identifies licenses signed with ECDSA and
	// SHA-256.
	SignatureAlgorithmECDSA = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha256"
	// SignatureAlgorithmEd25519 identifies licenses signed with pure
	// Ed25519 (RFC 8032), which signs the canonical license itself rather
	// than a digest. The identifier is defined by RFC 9231 for XML
	// Signature. The LCP specification doesn't define EdDSA signatures
	// (yet): this anticipates providers using Ed25519 certificates.
	SignatureAlgorithmEd25519 = "http://www.w3.org/2021/04/xmldsig-more#eddsa-ed25519"
)

// License is an LCP License Document, which grants a user access to an
// LCP-protected publication: it holds the content key of the publication,
// encrypted with the user key.
//
// See https://readium.org/lcp-specs/releases/lcp/latest.
type License struct {
	ID         string            `json:"id"`
	Issued     time.Time         `json:"issued"`
	Updated    *time.Time        `json:"updated,omitempty"`
	Provider   string            `json:"provider"`
	Encryption LicenseEncryption `json:"encryption"`
	Links      []LicenseLink     `json:"links,omitempty"`
	User       LicenseUser       `json:"user"`
	Rights     LicenseRights     `json:"rights"`
	Signature  *LicenseSignature `json:"signature,omitempty"`

	// raw is the JSON document the license was parsed from, kept verbatim to
	// be embedded in publications. Its canonical forms, which the signature
	// covers, are derived from it: see [License.canonicalForms].
	raw []byte
}

// LicenseEncryption describes how the content key is protected.
type LicenseEncryption struct {
	Profile    Profile           `json:"profile"`
	ContentKey LicenseContentKey `json:"content_key"`
	UserKey    LicenseUserKey    `json:"user_key"`
}

// LicenseContentKey is the content key of the publication, encrypted with the
// user key.
type LicenseContentKey struct {
	Algorithm      string `json:"algorithm"`
	EncryptedValue []byte `json:"encrypted_value"`
}

// LicenseUserKey describes the user key.
type LicenseUserKey struct {
	// Algorithm derives the user key from the user passphrase.
	Algorithm string `json:"algorithm"`
	// TextHint helps the user remember their passphrase.
	TextHint string `json:"text_hint"`
	// KeyCheck is the license identifier encrypted with the user key.
	KeyCheck []byte `json:"key_check"`
}

// LicenseLink is a link of a license, e.g. to the publication ("publication"),
// to the passphrase hint page ("hint") or to the status document ("status").
type LicenseLink struct {
	Href      string           `json:"href"`
	Rels      manifest.Strings `json:"rel"`
	Type      string           `json:"type,omitempty"`
	Title     string           `json:"title,omitempty"`
	Profile   string           `json:"profile,omitempty"`
	Templated bool             `json:"templated,omitempty"`
	Length    int64            `json:"length,omitempty"`
	Hash      string           `json:"hash,omitempty"`
}

// LicenseUser identifies the user the license was issued to. The fields
// listed in Encrypted are encrypted with the user key: see
// [License.DecryptUserFields].
type LicenseUser struct {
	ID        string   `json:"id,omitempty"`
	Email     string   `json:"email,omitempty"`
	Name      string   `json:"name,omitempty"`
	Encrypted []string `json:"encrypted,omitempty"`
}

// LicenseRights restricts the use of the publication. Nil values are
// unrestricted.
type LicenseRights struct {
	// Print is the maximum number of pages that can be printed.
	Print *int64 `json:"print,omitempty"`
	// Copy is the maximum number of characters that can be copied.
	Copy  *int64     `json:"copy,omitempty"`
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// LicenseSignature is the signature of a license by its provider.
type LicenseSignature struct {
	Algorithm string `json:"algorithm"`
	// Certificate is the DER-encoded certificate of the provider.
	Certificate []byte `json:"certificate"`
	Value       []byte `json:"value"`
}

// ParseLicense parses an LCP License Document. It returns
// [ErrInvalidLicense] when the document is malformed, or lacks the
// information needed to unlock the publication. When its encryption profile
// isn't one of LCP's, the error is [ErrUnknownProfile] as well.
//
// The fields of the License are decoded from the canonical form of the
// document, which its signature covers, so that they hold exactly what was
// signed. To that end, documents are refused when an object has two members
// whose names only differ in case, or not at all: JSON parsers resolve
// duplicates differently, and Go matches struct fields regardless of case.
// Documents larger than 1 MiB are refused as well, or whose canonical form
// is; licenses weigh a few kilobytes.
//
// The license is not verified: see [License.VerifySignature] and
// [License.CheckRights].
func ParseLicense(data []byte) (*License, error) {
	if len(data) > maxLicenseSize {
		return nil, errors.Wrapf(ErrInvalidLicense, "larger than %s", extensions.FormatSize(maxLicenseSize))
	}
	if err := checkMemberNames(data); err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, err.Error())
	}
	doc, err := decodeDocument(data)
	if err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, err.Error())
	}
	signature, err := documentSignature(doc)
	if err != nil {
		return nil, err
	}
	delete(doc, "signature")
	canonical, err := canonicalize(doc)
	if err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, err.Error())
	}

	var l License
	if err := json.Unmarshal(canonical, (*licenseFields)(&l)); err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, err.Error())
	}
	l.Signature = signature
	l.raw = bytes.Clone(data)

	switch {
	case l.ID == "":
		return nil, errors.Wrap(ErrInvalidLicense, "missing id")
	case l.Issued.IsZero():
		return nil, errors.Wrap(ErrInvalidLicense, "missing issue date")
	case l.Provider == "":
		return nil, errors.Wrap(ErrInvalidLicense, "missing provider")
	case l.Encryption.Profile == "":
		return nil, errors.Wrap(ErrInvalidLicense, "missing encryption profile")
	case len(l.Encryption.ContentKey.EncryptedValue) == 0:
		return nil, errors.Wrap(ErrInvalidLicense, "missing encrypted content key")
	case len(l.Encryption.UserKey.KeyCheck) == 0:
		return nil, errors.Wrap(ErrInvalidLicense, "missing key check")
	}
	if err := l.Encryption.Profile.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidLicense, err)
	}
	return &l, nil
}

// ReadLicense reads and parses the license embedded in a publication
// package: at META-INF/license.lcpl in an EPUB, or license.lcpl in a Readium
// Web Publication package. It returns [ErrNoLicense] when there is none.
//
// The format of the package is the one its media type names, as the streamer
// and [InjectLicense] tell it, or else the one its files tell. Only the path
// of the license of that format is read, so that a license left at the path
// of the other format can't stand in for its own. Only
// [Options.ArchiveFactory] is used.
func ReadLicense(ctx context.Context, a asset.PublicationAsset, opts Options) (*License, error) {
	f, err := opts.createFetcher(ctx, a)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	format, err := assetFormat(ctx, a, f)
	if err != nil {
		return nil, err
	}
	return readLicense(ctx, f, format)
}

// readLicense is [ReadLicense] for a package of the given format.
func readLicense(ctx context.Context, f fetcher.Fetcher, format packageFormat) (*License, error) {
	path := format.licensePath()
	if path == "" {
		return nil, ErrNoLicense
	}
	data, err := readEntry(ctx, f, path, maxLicenseSize)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, ErrNoLicense
	}
	return ParseLicense(data)
}

// packageFormat is the format of a package, which tells where its license
// is, and how it declares its encrypted resources.
type packageFormat int

const (
	otherFormat packageFormat = iota
	// epubFormat is an EPUB, identified by its META-INF/container.xml.
	epubFormat
	// rwpFormat is a Readium Web Publication package, identified by its
	// manifest.json.
	rwpFormat
)

// formatMarkers are the files identifying the formats of packages, by
// precedence: a package holding a META-INF/container.xml is an EPUB,
// whatever else it holds. [EncryptRWP] refuses such packages, which its
// consumers would take for EPUBs.
var formatMarkers = []struct {
	path   string
	format packageFormat
}{
	{"META-INF/container.xml", epubFormat},
	{rwpManifestPath, rwpFormat},
}

// licensePath returns the path of the license of a package of the format:
// META-INF/license.lcpl in an EPUB, or license.lcpl in a Readium Web
// Publication package. It is "" for other publications.
func (f packageFormat) licensePath() string {
	switch f {
	case epubFormat:
		return protection.PathEPUBLCPLicense
	case rwpFormat:
		return protection.PathRWPLCPLicense
	}
	return ""
}

// detectFormat returns the format of a package, as its files tell: see
// [formatMarkers]. A single operation should tell it once, and rely on it
// throughout, so that a package changing meanwhile doesn't get different
// answers.
func detectFormat(ctx context.Context, f fetcher.Fetcher) (packageFormat, error) {
	for _, m := range formatMarkers {
		res := f.Get(ctx, manifest.Link{Href: manifest.MustNewHREFFromString(m.path, false)})
		_, rerr := res.Length(ctx)
		res.Close()
		switch {
		case rerr == nil:
			return m.format, nil
		case rerr.Code != fetcher.CodeNotFound:
			return otherFormat, errors.Wrapf(rerr, "lcp: failed reading %s", m.path)
		}
	}
	return otherFormat, nil
}

// assetFormat returns the format of a package given as an asset: the one its
// media type names, which is what the streamer parses it as, or else the one
// its files tell.
func assetFormat(ctx context.Context, a asset.PublicationAsset, f fetcher.Fetcher) (packageFormat, error) {
	mt := a.MediaType(ctx)
	switch {
	case mt.Matches(&mediatype.EPUB):
		return epubFormat, nil
	case mt.Matches(&mediatype.ReadiumWebpub, &mediatype.ReadiumAudiobook, &mediatype.LCPProtectedAudiobook, &mediatype.ReadiumDivina, &mediatype.LCPProtectedPDF):
		return rwpFormat, nil
	}
	return detectFormat(ctx, f)
}

// licenseFields decodes the fields of a [License] from JSON, without the
// checks of [License.UnmarshalJSON], from a canonical form already checked.
type licenseFields License

// UnmarshalJSON parses the license as [ParseLicense] does, keeping its
// document, which the signature covers, and refusing what ParseLicense
// refuses, such as duplicate members. Otherwise a license decoded with
// encoding/json would be verified from its fields, ignoring the members they
// don't hold, while the document holding them is trusted, or forwarded.
//
// JSON null is refused too, rather than leaving the license as it is, as
// encoding/json does for other types: a License reused for decoding would
// otherwise keep the previous license in force. Decoded into a *License,
// null sets it to nil, which the verifications refuse.
func (l *License) UnmarshalJSON(data []byte) error {
	parsed, err := ParseLicense(data)
	if err != nil {
		return err
	}
	*l = *parsed
	return nil
}

// documentSignature returns the signature of a decoded license document, or
// nil when it has none.
func documentSignature(doc map[string]any) (*LicenseSignature, error) {
	signature, signed := doc["signature"]
	if !signed {
		return nil, nil
	}
	var s *LicenseSignature
	data, err := json.Marshal(signature)
	if err == nil {
		err = json.Unmarshal(data, &s)
	}
	if err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, "invalid signature: "+err.Error())
	}
	return s, nil
}

// Raw returns a copy of the JSON document the license was parsed from, or nil
// for a license which was not parsed. Unlike a serialization of the License,
// it is covered by the license signature.
func (l *License) Raw() []byte {
	return bytes.Clone(l.raw)
}

// parsedCopy returns a copy of the license parsed from its document, or
// from its serialization for a license which was not parsed.
func (l *License) parsedCopy() (*License, error) {
	data := l.Raw()
	if data == nil {
		var err error
		if data, err = json.Marshal(l); err != nil {
			return nil, errors.Wrap(ErrInvalidLicense, err.Error())
		}
	}
	return ParseLicense(data)
}

// Link returns the first link of the license with the given relation, or nil.
func (l *License) Link(rel string) *LicenseLink {
	for i, link := range l.Links {
		for _, r := range link.Rels {
			if r == rel {
				return &l.Links[i]
			}
		}
	}
	return nil
}

// decodeDocument decodes a JSON object, keeping numbers as they are written.
func decodeDocument(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("not a JSON object")
	}
	return doc, nil
}

// canonicalize serializes a JSON document in the canonical form of LCP
// licenses (LCP specification §5.3): object members sorted by Unicode code
// point, array elements in order, no insignificant whitespace, strings
// escaping only quotation marks, backslashes and control characters (with
// upper-case hexadecimal digits), and numbers without insignificant zeroes,
// in normalized scientific notation for non-integers.
//
// The canonical form may be much larger than the document: an exponent
// expands into zeroes (1e1000 takes 6 bytes, its canonical form 1001), and
// short escapes into long ones (\n takes 2 bytes, \u000A 6). It is refused
// beyond maxCanonicalSize.
func canonicalize(doc map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, doc); err != nil {
		return nil, err
	}
	if buf.Len() > maxCanonicalSize {
		return nil, errCanonicalTooLarge
	}
	return buf.Bytes(), nil
}

// maxCanonicalSize is the maximum size of the canonical form of a license.
const maxCanonicalSize = maxLicenseSize // 1 MiB

var errCanonicalTooLarge = errors.Errorf("canonical form larger than %s", extensions.FormatSize(maxCanonicalSize))

// writeCanonical writes the canonical form of a JSON value to buf. It stops
// once buf exceeds maxCanonicalSize, which it overshoots by one string or
// number at most.
func writeCanonical(buf *bytes.Buffer, v any) error {
	if buf.Len() > maxCanonicalSize {
		return errCanonicalTooLarge
	}
	switch v := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		buf.WriteString(strconv.FormatBool(v))
	case json.Number:
		n, err := canonicalNumber(string(v))
		if err != nil {
			return err
		}
		buf.WriteString(n)
	case string:
		writeCanonicalString(buf, v)
	case []any:
		buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		// The byte order of UTF-8 strings is the order of their code points.
		keys := slices.Sorted(maps.Keys(v))
		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalString(buf, key)
			buf.WriteByte(':')
			if err := writeCanonical(buf, v[key]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return errors.Errorf("unexpected JSON value %T", v)
	}
	return nil
}

// writeCanonicalString writes a JSON string escaping only what JSON requires
// to: quotation marks, backslashes and control characters. Decoded strings
// are valid UTF-8.
func writeCanonicalString(buf *bytes.Buffer, s string) {
	const hex = "0123456789ABCDEF"
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\\':
			buf.WriteByte('\\')
			buf.WriteByte(c)
		case c < 0x20:
			buf.WriteString(`\u00`)
			buf.WriteByte(hex[c>>4])
			buf.WriteByte(hex[c&0xF])
		default:
			buf.WriteByte(c)
		}
	}
	buf.WriteByte('"')
}

// canonicalNumber returns the canonical form of a JSON number: integers in
// decimal notation without leading zeroes, and other numbers in normalized
// scientific notation, without insignificant zeroes, such as 1.25E-3.
func canonicalNumber(literal string) (string, error) {
	negative := strings.HasPrefix(literal, "-")
	mantissa := strings.TrimPrefix(literal, "-")
	var exponent int64
	if i := strings.IndexAny(mantissa, "eE"); i >= 0 {
		// Within 32 bits, so that the adjustments below, by at most the
		// length of the literal, can't overflow.
		e, err := strconv.ParseInt(mantissa[i+1:], 10, 32)
		if err != nil {
			if errors.Is(err, strconv.ErrRange) {
				return "", errors.Errorf("number %s out of range", literal)
			}
			return "", errors.Errorf("invalid number %s", literal)
		}
		mantissa, exponent = mantissa[:i], e
	}
	integer, fraction, _ := strings.Cut(mantissa, ".")
	// The number is digits × 10^exponent.
	digits := strings.TrimLeft(integer+fraction, "0")
	exponent -= int64(len(fraction))
	if digits == "" {
		return "0", nil
	}
	trimmed := strings.TrimRight(digits, "0")
	exponent += int64(len(digits) - len(trimmed))
	digits = trimmed

	var n string
	switch {
	case exponent > 1000:
		return "", errors.Errorf("number %s out of range", literal)
	case exponent >= 0:
		n = digits + strings.Repeat("0", int(exponent))
	default:
		n = digits[:1]
		if len(digits) > 1 {
			n += "." + digits[1:]
		}
		n += "E" + strconv.FormatInt(exponent+int64(len(digits))-1, 10)
	}
	if negative {
		n = "-" + n
	}
	return n, nil
}

// canonicalizeLegacy serializes a JSON document in the canonical form
// readium-lcp-server produces, with Go's JSON encoder: unlike the
// specification, it escapes U+2028 and U+2029, escapes some control
// characters with short forms or lower-case hexadecimal digits, and keeps
// numbers as they are written.
func canonicalizeLegacy(doc map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// checkMemberNames returns an error when the JSON document is malformed, or
// when one of its objects has two members whose names are equal regardless
// of case.
func checkMemberNames(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := checkValueMemberNames(dec, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("unexpected data after the JSON document")
	}
	return nil
}

// maxJSONDepth is the nesting limit of the JSON documents of licenses, far
// deeper than they are. Each level takes a call of checkValueMemberNames, and
// the token reader of encoding/json doesn't limit them in every version of
// Go: it is below the limit of its decoder, so that it applies first.
const maxJSONDepth = 1000

func checkValueMemberNames(dec *json.Decoder, depth int) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == json.Delim('{') || tok == json.Delim('[') {
		if depth++; depth > maxJSONDepth {
			return errors.Errorf("nested more than %d levels deep", maxJSONDepth)
		}
	}
	switch tok {
	case json.Delim('{'):
		seen := make(map[string]bool)
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				return err
			}
			name, _ := tok.(string)
			folded := foldName(name)
			if seen[folded] {
				return errors.Errorf("duplicate member %q", name)
			}
			seen[folded] = true
			if err := checkValueMemberNames(dec, depth); err != nil {
				return err
			}
		}
	case json.Delim('['):
		for dec.More() {
			if err := checkValueMemberNames(dec, depth); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	_, err = dec.Token() // closing delimiter
	return err
}

// foldName maps each rune of a name to the smallest rune it is equal to
// regardless of case, so that two names are equal regardless of case
// exactly when their folded forms are equal.
func foldName(name string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			smallest = min(smallest, f)
		}
		return smallest
	}, name)
}
