package lcp

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The license fixtures:
//
//   - license-rsa.lcpl and license-ecdsa.lcpl were generated with the
//     License Server's own code (github.com/edrlab/lcp-server/pkg/lic), for
//     the content key of the compat fixtures and testPassphrase. They are
//     signed by provider certificates issued by test-root-ca.pem.
//   - edrlab-license.lcpl is a license issued by EDRLab's test License
//     Server (lcp-server's test/license1.lcpl), whose passphrase is unknown.
//     Its provider certificate was issued by EDRLab's 2016 test root
//     (edrlab-test-root-ca-2016.pem), since replaced by a root with a new key
//     (edrlab-test-root-ca-2026.pem).
const testPassphrase = "mot de passe très secret"

func loadLicense(t *testing.T, name string) *License {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	l, err := ParseLicense(data)
	require.NoError(t, err)
	return l
}

func loadRoots(t *testing.T, names ...string) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	for _, name := range names {
		data, err := os.ReadFile("testdata/" + name)
		require.NoError(t, err)
		block, _ := pem.Decode(data)
		require.NotNil(t, block, name)
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		pool.AddCert(cert)
	}
	return pool
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParseLicense(t *testing.T) {
	l := loadLicense(t, "license-ecdsa.lcpl")
	assert.Equal(t, "7c1a6f0e-2b3d-4e5f-8a9b-0c1d2e3f4a5b", l.ID)
	assert.Equal(t, "https://www.example.com", l.Provider)
	assert.Equal(t, mustTime("2025-01-01T10:00:00Z"), l.Issued)
	require.NotNil(t, l.Updated)
	assert.Equal(t, mustTime("2025-06-15T12:30:00Z"), *l.Updated)
	assert.Equal(t, ProfileBasic, l.Encryption.Profile)
	assert.Equal(t, AlgorithmAES256CBC, l.Encryption.ContentKey.Algorithm)
	assert.Len(t, l.Encryption.ContentKey.EncryptedValue, 64)
	assert.Equal(t, AlgorithmSHA256, l.Encryption.UserKey.Algorithm)
	assert.Equal(t, "Ton mot de passe préféré", l.Encryption.UserKey.TextHint)
	assert.Equal(t, []string{"email", "name"}, l.User.Encrypted)
	assert.Equal(t, "da88987e-d012-4173-8c74-64af5f33f838", l.User.ID)
	require.NotNil(t, l.Rights.Print)
	assert.EqualValues(t, 10, *l.Rights.Print)
	assert.EqualValues(t, 2000, *l.Rights.Copy)
	assert.Equal(t, mustTime("2045-01-01T00:00:00Z"), *l.Rights.End)
	assert.Equal(t, SignatureAlgorithmECDSA, l.Signature.Algorithm)

	publication := l.Link("publication")
	require.NotNil(t, publication)
	assert.Equal(t, "application/epub+zip", publication.Type)
	assert.EqualValues(t, 14958, publication.Length)
	assert.Equal(t, "text/html", l.Link("hint").Type)
	assert.Nil(t, l.Link("unknown"))

	data, err := os.ReadFile("testdata/license-ecdsa.lcpl")
	require.NoError(t, err)
	assert.Equal(t, data, l.Raw())
}

func TestParseLicenseRelArray(t *testing.T) {
	data, err := os.ReadFile("testdata/license-rsa.lcpl")
	require.NoError(t, err)
	data = bytes.Replace(data, []byte(`"rel": "hint"`), []byte(`"rel": ["hint", "help"]`), 1)
	l, err := ParseLicense(data)
	require.NoError(t, err)
	require.NotNil(t, l.Link("help"))
	assert.Same(t, l.Link("hint"), l.Link("help"))
}

func TestParseLicenseInvalid(t *testing.T) {
	data, err := os.ReadFile("testdata/license-rsa.lcpl")
	require.NoError(t, err)

	without := func(path ...string) []byte {
		var clone map[string]any
		require.NoError(t, json.Unmarshal(data, &clone))
		m := clone
		for _, key := range path[:len(path)-1] {
			m = m[key].(map[string]any)
		}
		delete(m, path[len(path)-1])
		out, err := json.Marshal(clone)
		require.NoError(t, err)
		return out
	}
	for name, input := range map[string][]byte{
		"not JSON":       []byte("not json"),
		"not an object":  []byte(`[]`),
		"no id":          without("id"),
		"no issued":      without("issued"),
		"no provider":    without("provider"),
		"no profile":     without("encryption", "profile"),
		"no content key": without("encryption", "content_key"),
		"no key check":   without("encryption", "user_key", "key_check"),
		"bad date":       bytes.Replace(data, []byte(`"2025-01-01T10:00:00Z"`), []byte(`"yesterday"`), 1),
	} {
		_, err := ParseLicense(input)
		assert.ErrorIs(t, err, ErrInvalidLicense, name)
	}
}

func TestLicenseContentKey(t *testing.T) {
	for _, name := range []string{"license-rsa.lcpl", "license-ecdsa.lcpl"} {
		l := loadLicense(t, name)

		key, err := l.ContentKeyFromPassphrase(testPassphrase)
		require.NoError(t, err, name)
		assert.Equal(t, testKey("lcp-compat"), key, name)

		// The hex-encoded hashed passphrase, as providers know it.
		hash := sha256.Sum256([]byte(testPassphrase))
		uk, err := UserKeyFromHex(hex.EncodeToString(hash[:]))
		require.NoError(t, err)
		assert.NoError(t, l.CheckUserKey(uk))
		key, err = l.ContentKey(uk)
		require.NoError(t, err)
		assert.Equal(t, testKey("lcp-compat"), key)

		_, err = l.ContentKeyFromPassphrase("wrong passphrase")
		assert.ErrorIs(t, err, ErrWrongUserKey, name)
		_, err = l.ContentKey(UserKey(testKey("wrong")))
		assert.ErrorIs(t, err, ErrWrongUserKey, name)
		_, err = l.ContentKey(UserKey("short"))
		assert.ErrorIs(t, err, ErrInvalidKey, name)
	}
}

func TestLicenseDecryptUserFields(t *testing.T) {
	l := loadLicense(t, "license-rsa.lcpl")
	uk, err := UserKeyFromPassphrase(testPassphrase, ProfileBasic)
	require.NoError(t, err)
	fields, err := l.DecryptUserFields(uk)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"email": "reader@example.com", "name": "Jöhn Reader"}, fields)

	_, err = l.DecryptUserFields(UserKey(testKey("wrong")))
	assert.ErrorIs(t, err, ErrWrongUserKey)
}

// productionProfiles are the identifiers of the production profiles, spelled
// out rather than taken from the constants they check.
var productionProfiles = []string{
	"http://readium.org/lcp/profile-1.0",
	"http://readium.org/lcp/profile-2.0",
	"http://readium.org/lcp/profile-2.1",
	"http://readium.org/lcp/profile-2.2",
	"http://readium.org/lcp/profile-2.3",
	"http://readium.org/lcp/profile-2.4",
	"http://readium.org/lcp/profile-2.5",
	"http://readium.org/lcp/profile-2.6",
	"http://readium.org/lcp/profile-2.7",
	"http://readium.org/lcp/profile-2.8",
	"http://readium.org/lcp/profile-2.9",
}

// unknownProfiles resemble profiles, but aren't. The License Server takes
// "2.x" from its configuration, and replaces it in each license.
var unknownProfiles = []string{
	"http://readium.org/lcp/profile-2.x",
	"http://readium.org/lcp/profile-1.1",
	"http://readium.org/lcp/profile-2.10",
	"http://readium.org/lcp/profile-3.0",
	"http://readium.org/lcp/profile-",
	"http://readium.org/lcp/basic-profile/",
	"https://readium.org/lcp/basic-profile",
	"HTTP://readium.org/lcp/basic-profile",
	"basic",
}

func TestProfile(t *testing.T) {
	assert.NoError(t, Profile("http://readium.org/lcp/basic-profile").Validate())
	assert.False(t, ProfileBasic.IsProduction())
	for _, p := range productionProfiles {
		assert.NoError(t, Profile(p).Validate(), p)
		assert.True(t, Profile(p).IsProduction(), p)
	}
	for _, p := range append(unknownProfiles, "") {
		assert.ErrorIs(t, Profile(p).Validate(), ErrUnknownProfile, p)
		assert.False(t, Profile(p).IsProduction(), p)
	}
}

func TestParseLicenseProfile(t *testing.T) {
	data, err := os.ReadFile("testdata/license-rsa.lcpl")
	require.NoError(t, err)
	parse := func(profile string) (*License, error) {
		changed := bytes.Replace(data, []byte(`"http://readium.org/lcp/basic-profile"`), []byte(`"`+profile+`"`), 1)
		require.Contains(t, string(changed), `"`+profile+`"`)
		return ParseLicense(changed)
	}
	for _, p := range append(productionProfiles, "http://readium.org/lcp/basic-profile") {
		l, err := parse(p)
		require.NoError(t, err, p)
		assert.Equal(t, Profile(p), l.Encryption.Profile)
	}
	for _, p := range unknownProfiles {
		_, err := parse(p)
		assert.ErrorIs(t, err, ErrInvalidLicense, p)
		assert.ErrorIs(t, err, ErrUnknownProfile, p)
		assert.NotErrorIs(t, err, ErrUnsupportedProfile, p)
		assert.ErrorContains(t, err, strconv.Quote(p))
	}
}

func TestLicenseProfile(t *testing.T) {
	uk, err := UserKeyFromPassphrase(testPassphrase, ProfileBasic)
	require.NoError(t, err)
	l := loadLicense(t, "license-rsa.lcpl")
	contentKey, err := l.ContentKey(uk)
	require.NoError(t, err)

	// The user key of production profiles can't be derived, whatever its
	// algorithm, but it unlocks their licenses.
	for _, p := range productionProfiles {
		_, err := UserKeyFromPassphrase(testPassphrase, Profile(p))
		assert.ErrorIs(t, err, ErrUnsupportedProfile, p)
		assert.NotErrorIs(t, err, ErrUnknownProfile, p)

		l := loadLicense(t, "license-rsa.lcpl")
		l.Encryption.Profile = Profile(p)
		l.Encryption.UserKey.Algorithm = "http://www.example.com/algorithm"
		_, err = l.ContentKeyFromPassphrase(testPassphrase)
		assert.ErrorIs(t, err, ErrUnsupportedProfile, p)
		assert.ErrorContains(t, err, "the user key of the production profile "+p)
		unlocked, err := l.ContentKey(uk)
		require.NoError(t, err, p)
		assert.Equal(t, contentKey, unlocked, p)
	}

	// Nor is anything assumed of the algorithms of unknown profiles, for
	// licenses which weren't parsed.
	for _, p := range unknownProfiles {
		_, err := UserKeyFromPassphrase(testPassphrase, Profile(p))
		assert.ErrorIs(t, err, ErrUnknownProfile, p)
		assert.NotErrorIs(t, err, ErrUnsupportedProfile, p)

		l := loadLicense(t, "license-rsa.lcpl")
		l.raw, l.Encryption.Profile = nil, Profile(p)
		_, err = l.ContentKeyFromPassphrase(testPassphrase)
		assert.ErrorIs(t, err, ErrUnknownProfile, p)
		_, err = l.ContentKey(uk)
		assert.ErrorIs(t, err, ErrUnknownProfile, p)
		_, err = l.DecryptUserFields(uk)
		assert.ErrorIs(t, err, ErrUnknownProfile, p)
		verifier := Verifier{Roots: loadRoots(t, "test-root-ca.pem"), InsecureSkipRevocation: true}
		assert.ErrorIs(t, verifier.Verify(l), ErrUnknownProfile, p)
	}
}

func TestStreamerReportsUnknownProfiles(t *testing.T) {
	files := zipContents(t, licensedEPUB(t, "license-rsa.lcpl"))
	license := string(files[protection.PathEPUBLCPLicense])
	changed := strings.Replace(license, "http://readium.org/lcp/basic-profile", "http://readium.org/lcp/profile-2.x", 1)
	require.NotEqual(t, license, changed)
	files[protection.PathEPUBLCPLicense] = []byte(changed)
	_, err := openWithLCP(t, zipFiles(t, "unknown.epub", files))
	assert.ErrorIs(t, err, ErrUnknownProfile)
	assert.ErrorContains(t, err, "http://readium.org/lcp/profile-2.x")
}

func TestUserKey(t *testing.T) {
	_, err := UserKeyFromHex("zz")
	assert.Error(t, err)
	_, err = UserKeyFromHex("abcd")
	assert.ErrorIs(t, err, ErrInvalidKey)
}

func TestLicenseVerifySignature(t *testing.T) {
	testRoot := loadRoots(t, "test-root-ca.pem")
	for _, name := range []string{"license-rsa.lcpl", "license-ecdsa.lcpl"} {
		l := loadLicense(t, name)
		assert.NoError(t, l.VerifySignature(testRoot), name)
		assert.NoError(t, l.VerifyIntegrity(), name)
		assert.ErrorIs(t, l.VerifySignature(loadRoots(t, "edrlab-test-root-ca-2016.pem")), ErrInvalidCertificate, name)
	}

	// A license issued by EDRLab's test License Server. Its provider
	// certificate has expired since, which doesn't matter: it was valid
	// when the license was issued.
	edrlab := loadLicense(t, "edrlab-license.lcpl")
	assert.NoError(t, edrlab.VerifyIntegrity())
	assert.NoError(t, edrlab.VerifySignature(loadRoots(t, "edrlab-test-root-ca-2016.pem")))
	assert.NoError(t, edrlab.VerifySignature(loadRoots(t, "edrlab-test-root-ca-2016.pem", "edrlab-test-root-ca-2026.pem")))
	// The renewed root has a new key.
	assert.ErrorIs(t, edrlab.VerifySignature(loadRoots(t, "edrlab-test-root-ca-2026.pem")), ErrInvalidCertificate)
}

func TestLicenseVerifySignatureCanonicalization(t *testing.T) {
	for _, name := range []string{"license-rsa.lcpl", "edrlab-license.lcpl"} {
		data, err := os.ReadFile("testdata/" + name)
		require.NoError(t, err)

		// Formatting doesn't matter.
		var compact bytes.Buffer
		require.NoError(t, json.Compact(&compact, data))
		l, err := ParseLicense(compact.Bytes())
		require.NoError(t, err)
		assert.NoError(t, l.VerifyIntegrity(), name)

		// The signature covers the whole document.
		for _, tamper := range [][2]string{
			{`"provider": "https://www.`, `"provider": "https://evil.`},
			{`"rights": {`, `"rights": {"tts": true, `},
		} {
			tampered := strings.Replace(string(data), tamper[0], tamper[1], 1)
			require.NotEqual(t, string(data), tampered)
			l, err := ParseLicense([]byte(tampered))
			require.NoError(t, err)
			assert.ErrorIs(t, l.VerifyIntegrity(), ErrInvalidSignature, "%s: %s", name, tamper[1])
		}
	}

	l := loadLicense(t, "license-rsa.lcpl")
	l.Signature = nil
	assert.ErrorIs(t, l.VerifyIntegrity(), ErrInvalidSignature)

	l = loadLicense(t, "license-rsa.lcpl")
	l.Signature.Algorithm = SignatureAlgorithmECDSA
	assert.ErrorIs(t, l.VerifyIntegrity(), ErrInvalidSignature)
}

func TestLicenseCheckRights(t *testing.T) {
	l := loadLicense(t, "edrlab-license.lcpl")
	assert.NoError(t, l.CheckRights(mustTime("2023-01-01T00:00:00Z")))
	assert.ErrorIs(t, l.CheckRights(mustTime("2022-01-01T00:00:00Z")), ErrLicenseNotStarted)
	assert.ErrorIs(t, l.CheckRights(time.Now()), ErrLicenseExpired)

	assert.NoError(t, (&License{}).CheckRights(time.Now()), "no rights restriction")
}

// ed25519License re-signs license-rsa.lcpl with an Ed25519 provider
// certificate, issued by a new Ed25519 root which it returns.
func ed25519License(t *testing.T) (*License, *x509.CertPool) {
	t.Helper()
	l, root, _ := ed25519Fixture(t)
	pool := x509.NewCertPool()
	pool.AddCert(root)
	return l, pool
}

// ed25519Fixture re-signs license-rsa.lcpl with an Ed25519 provider
// certificate, issued by a new Ed25519 root. It returns the license, and the
// root certificate with its key.
func ed25519Fixture(t *testing.T) (*License, *x509.Certificate, ed25519.PrivateKey) {
	t.Helper()
	p := newEd25519Provider(t)
	l := loadLicense(t, "license-rsa.lcpl")
	canonical, err := l.canonicalJSON()
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(l.Raw(), &doc))
	return p.sign(t, doc, canonical), p.root, p.rootKey
}

// ed25519Provider is a provider with an Ed25519 certificate, issued by an
// Ed25519 root.
type ed25519Provider struct {
	key     ed25519.PrivateKey
	der     []byte // provider certificate
	root    *x509.Certificate
	rootKey ed25519.PrivateKey
}

func newEd25519Provider(t *testing.T) ed25519Provider {
	t.Helper()
	rootPub, rootKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Ed25519 test CA"},
		NotBefore:             mustTime("2020-01-01T00:00:00Z"),
		NotAfter:              mustTime("2060-01-01T00:00:00Z"),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	rootDER, err := x509.CreateCertificate(nil, rootTmpl, rootTmpl, rootPub, rootKey)
	require.NoError(t, err)
	root, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	providerPub, providerKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	providerDER, err := x509.CreateCertificate(nil, &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Ed25519 test provider"},
		NotBefore:    mustTime("2021-01-01T00:00:00Z"),
		NotAfter:     mustTime("2050-01-01T00:00:00Z"),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}, root, providerPub, rootKey)
	require.NoError(t, err)
	return ed25519Provider{key: providerKey, der: providerDER, root: root, rootKey: rootKey}
}

// sign returns the license document doc, with the signature of message.
func (p ed25519Provider) sign(t *testing.T, doc map[string]any, message []byte) *License {
	t.Helper()
	doc["signature"] = map[string]any{
		"algorithm":   SignatureAlgorithmEd25519,
		"certificate": p.der,
		"value":       ed25519.Sign(p.key, message),
	}
	data, err := json.Marshal(doc)
	require.NoError(t, err)
	signed, err := ParseLicense(data)
	require.NoError(t, err)
	return signed
}

func TestLicenseVerifySignatureEd25519(t *testing.T) {
	l, roots := ed25519License(t)
	assert.Equal(t, SignatureAlgorithmEd25519, l.Signature.Algorithm)
	assert.NoError(t, l.VerifySignature(roots))
	assert.NoError(t, l.VerifyIntegrity())
	assert.ErrorIs(t, l.VerifySignature(loadRoots(t, "test-root-ca.pem")), ErrInvalidCertificate)

	// The signature covers the canonical license.
	tampered, err := ParseLicense(bytes.Replace(l.Raw(), []byte(`"https://www.example.com"`), []byte(`"https://evil.example.com"`), 1))
	require.NoError(t, err)
	assert.ErrorIs(t, tampered.VerifyIntegrity(), ErrInvalidSignature)

	// The declared algorithm must match the key.
	l.Signature.Algorithm = SignatureAlgorithmECDSA
	assert.ErrorIs(t, l.VerifyIntegrity(), ErrInvalidSignature)
	rsaLicense := loadLicense(t, "license-rsa.lcpl")
	rsaLicense.Signature.Algorithm = SignatureAlgorithmEd25519
	assert.ErrorIs(t, rsaLicense.VerifyIntegrity(), ErrInvalidSignature)

	// The license still unlocks as before.
	l, _ = ed25519License(t)
	key, err := l.ContentKeyFromPassphrase(testPassphrase)
	require.NoError(t, err)
	assert.Equal(t, testKey("lcp-compat"), key)
}

func TestRepeatedEncryptedFieldNames(t *testing.T) {
	l := loadLicense(t, "license-rsa.lcpl")
	uk, err := UserKeyFromPassphrase(testPassphrase, ProfileBasic)
	require.NoError(t, err)
	decrypt := func() {
		fields, err := l.DecryptUserFields(uk)
		if err != nil || fields["email"] != "reader@example.com" {
			t.Fatal("failed to decrypt the user field", err)
		}
	}
	const duplicates = 64
	once := testing.AllocsPerRun(8, decrypt)
	for range duplicates {
		l.User.Encrypted = append(l.User.Encrypted, "email")
	}
	repeated := testing.AllocsPerRun(8, decrypt)
	// Allow for JSON encoder pools, map growth and race instrumentation.
	// Re-decoding the ciphertext requires multiple allocations per name.
	assert.Less(t, repeated, once+duplicates, "duplicate names should reuse the decrypted value")
}

func TestNilLicenseUnlock(t *testing.T) {
	var l *License
	uk := UserKey(testKey("reader"))
	assert.ErrorIs(t, l.CheckUserKey(uk), ErrInvalidLicense)
	_, err := l.ContentKey(uk)
	assert.ErrorIs(t, err, ErrInvalidLicense)
	_, err = l.ContentKeyFromPassphrase(testPassphrase)
	assert.ErrorIs(t, err, ErrInvalidLicense)
	_, err = l.DecryptUserFields(uk)
	assert.ErrorIs(t, err, ErrInvalidLicense)
}

func TestLicenseVerifyASN1Signature(t *testing.T) {
	l := loadLicense(t, "license-ecdsa.lcpl")
	sig := l.Signature.Value
	require.Len(t, sig, 64)
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	doc, err := decodeDocument(l.raw)
	require.NoError(t, err)
	for _, valid := range []bool{true, false} {
		if !valid {
			r.Add(r, big.NewInt(1))
		}
		der, err := asn1.Marshal(struct{ R, S *big.Int }{r, s})
		require.NoError(t, err)
		doc["signature"].(map[string]any)["value"] = der
		data, err := json.Marshal(doc)
		require.NoError(t, err)
		parsed, err := ParseLicense(data)
		require.NoError(t, err)
		err = parsed.VerifySignature(loadRoots(t, "test-root-ca.pem"))
		if valid {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, ErrInvalidSignature)
		}
	}
}
