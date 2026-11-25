package lcp

// Regression tests for the findings of the security review of the package:
// resources left in the clear, licenses whose fields aren't what was
// signed, memory exhaustion, and files leaking out of exploded publications.

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"maps"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	epubparser "github.com/readium/go-toolkit/pkg/parser/epub"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Resource lookups: a resource selected for encryption can't be found in the
// clear under another spelling of its path.

func TestEncryptRWPHrefSpellings(t *testing.T) {
	key := testKey("spellings")
	m := strings.Replace(testRWPManifest, `"href": "chapter.html"`, `"href": "chapter.html?v=2"`, 1)
	m = strings.Replace(m, `"href": "style.css"`, `"href": "./style.css#main"`, 1)
	require.NotEqual(t, testRWPManifest, m)

	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), key, &buf, Options{}))
	encrypted := writeTemp(t, "encrypted.webpub", buf.Bytes())
	output, _ := readZip(t, encrypted)
	assert.NotEqual(t, testRWPFiles["chapter.html"], output["chapter.html"].content)
	assert.NotEqual(t, testRWPFiles["style.css"], output["style.css"].content)
	// Every resource but the cover declares its encryption.
	assert.Len(t, encryptedHrefs(t, output["manifest.json"].content), 5)

	var restored bytes.Buffer
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, encrypted), key, &restored, Options{}))
	entries, _ := readZip(t, writeTemp(t, "restored.webpub", restored.Bytes()))
	assert.Equal(t, testRWPFiles["chapter.html"], entries["chapter.html"].content)
	assert.Equal(t, testRWPFiles["style.css"], entries["style.css"].content)
}

func TestEncryptRWPRefusesResourcesMissingFromThePackage(t *testing.T) {
	// The package holds chapter.html, but the manifest lists Chapter.html. On
	// a case-insensitive file system, an exploded package would serve one as
	// the other: chapter.html must not be copied in the clear as a file the
	// manifest doesn't list.
	m := strings.Replace(testRWPManifest, `"href": "chapter.html"`, `"href": "Chapter.html"`, 1)
	err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "Chapter.html is listed in the manifest, but not found in the package")
}

func TestEncryptPublicationFailsClosed(t *testing.T) {
	dir := t.TempDir()
	chapter := textBytes(3000)
	files := map[string][]byte{
		"chapter.html":       chapter,
		"cover.jpg":          randomBytes(500),
		"unlisted.txt":       textBytes(100),
		"META-INF/rights.xx": textBytes(100),
	}
	for name, data := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o644))
	}
	link := func(href string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}
	}
	cover := link("cover.jpg")
	cover.Rels = manifest.Strings{"cover"}
	cover.MediaType = &mediatype.JPEG
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{cover, link("chapter.html")}}
	key := testKey("closed")
	encrypted, err := EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), key, Options{})
	require.NoError(t, err)
	enc := encrypted.Manifest.ReadingOrder[1].Properties.Encryption()
	require.NotNil(t, enc)

	read := func(href string) ([]byte, *fetcher.ResourceError) {
		res := encrypted.Get(t.Context(), link(href))
		defer res.Close()
		return res.Read(t.Context(), 0, 0)
	}
	for _, href := range []string{"chapter.html", "chapter.html#intro", "chapter.html?v=2", "./chapter.html", "/chapter.html", "dir/../chapter.html"} {
		data, rerr := read(href)
		require.Nil(t, rerr, href)
		var plain bytes.Buffer
		_, err := DecryptContent(key, bytes.NewReader(data), &plain, *enc)
		require.NoError(t, err, "%s is served in the clear", href)
		assert.Equal(t, chapter, plain.Bytes(), href)
	}

	// The other resources of the manifest are served as they are.
	data, rerr := read("cover.jpg")
	require.Nil(t, rerr)
	assert.Equal(t, files["cover.jpg"], data)

	// Files the manifest doesn't link to could be a selected resource under
	// another name, e.g. differing in case on a case-insensitive file
	// system: they are not served, META-INF included.
	for _, href := range []string{"unlisted.txt", "CHAPTER.html", "META-INF/rights.xx"} {
		_, rerr := read(href)
		require.NotNil(t, rerr, href)
		assert.Equal(t, fetcher.CodeNotFound, rerr.Code, href)
	}
	links, err := encrypted.Fetcher.Links(t.Context())
	require.NoError(t, err)
	var hrefs []string
	for _, l := range links {
		hrefs = append(hrefs, l.Href.String())
	}
	assert.ElementsMatch(t, []string{"chapter.html", "cover.jpg"}, hrefs)
}

// Licenses: their fields hold exactly what was signed.

func TestParseLicenseRejectsAmbiguousMembers(t *testing.T) {
	read := func(name string) string {
		data, err := os.ReadFile("testdata/" + name)
		require.NoError(t, err)
		return string(data)
	}
	rsa, edrlab := read("license-rsa.lcpl"), read("edrlab-license.lcpl")
	for name, input := range map[string]string{
		// Go merged both objects into the struct, while the signature
		// covered the second one only: print = 7 and a valid signature.
		"duplicate member":                  strings.Replace(edrlab, `"rights": {`, `"rights": {"print": 7}, "rights": {`, 1),
		"duplicate nested member":           strings.Replace(rsa, `"rights": {`, `"rights": {"print": 1000, `, 1),
		"members differing in case":         strings.Replace(rsa, `"provider":`, `"Provider": "https://evil.example.com", "provider":`, 1),
		"members differing in Unicode case": strings.Replace(rsa, `"rights": {`, `"rights": {"Key": 1, "key": 2, `, 1),
		"trailing data":                     rsa + "{}",
	} {
		require.NotEqual(t, rsa, input, name)
		_, err := ParseLicense([]byte(input))
		assert.ErrorIs(t, err, ErrInvalidLicense, name)
	}
}

func TestLicenseFieldsAreDecodedFromTheSignedForm(t *testing.T) {
	l := loadLicense(t, "license-ecdsa.lcpl")
	canonical, err := l.canonicalJSON()
	require.NoError(t, err)
	var signed License
	require.NoError(t, json.Unmarshal(canonical, &signed))
	signed.Signature, signed.raw = l.Signature, l.raw
	assert.Equal(t, &signed, l)

	// Raw returns a copy.
	raw := l.Raw()
	raw[0] = 'x'
	assert.Equal(t, byte('{'), l.Raw()[0])
	assert.NoError(t, l.VerifyIntegrity())
}

func TestVerifySignatureRequiresRoots(t *testing.T) {
	l := loadLicense(t, "license-rsa.lcpl")
	assert.ErrorIs(t, l.VerifySignature(nil), ErrInvalidCertificate)
	assert.NoError(t, l.VerifyIntegrity())
}

func TestVerifier(t *testing.T) {
	roots := loadRoots(t, "test-root-ca.pem")
	l := loadLicense(t, "license-ecdsa.lcpl")
	assert.NoError(t, Verifier{Roots: roots, InsecureSkipRevocation: true}.Verify(l))
	assert.ErrorIs(t, Verifier{InsecureSkipRevocation: true}.Verify(l), ErrInvalidCertificate, "roots are required")
	assert.ErrorContains(t, Verifier{Roots: roots}.Verify(l), "IsRevoked is required", "revocation is checked, unless explicitly skipped")
	assert.ErrorIs(t, Verifier{Roots: loadRoots(t, "edrlab-test-root-ca-2026.pem"), InsecureSkipRevocation: true}.Verify(l), ErrInvalidCertificate)

	tampered, err := ParseLicense(bytes.Replace(l.Raw(), []byte(`"https://www.example.com"`), []byte(`"https://evil.example.com"`), 1))
	require.NoError(t, err)
	assert.ErrorIs(t, Verifier{Roots: roots, InsecureSkipRevocation: true}.Verify(tampered), ErrInvalidSignature)

	// Revocation is checked on the provider certificate, given its issuer
	// from the verified chain.
	var checked, checkedIssuer *x509.Certificate
	revoked := Verifier{Roots: roots, IsRevoked: func(provider, issuer *x509.Certificate) (bool, error) {
		checked, checkedIssuer = provider, issuer
		return true, nil
	}}
	assert.ErrorIs(t, revoked.Verify(l), ErrCertificateRevoked)
	require.NotNil(t, checked)
	assert.Equal(t, l.Signature.Certificate, checked.Raw)
	require.NotNil(t, checkedIssuer)
	assert.NoError(t, checked.CheckSignatureFrom(checkedIssuer), "the issuer is the root which signed the provider certificate")
	unavailable := Verifier{Roots: roots, IsRevoked: func(_, _ *x509.Certificate) (bool, error) {
		return false, errors.New("CRL unavailable")
	}}
	assert.ErrorContains(t, unavailable.Verify(l), "CRL unavailable")
	valid := Verifier{Roots: roots, IsRevoked: func(_, _ *x509.Certificate) (bool, error) { return false, nil }}
	assert.NoError(t, valid.Verify(l))

	// So are the rights.
	edrlab := loadLicense(t, "edrlab-license.lcpl")
	assert.ErrorIs(t, Verifier{Roots: loadRoots(t, "edrlab-test-root-ca-2016.pem"), InsecureSkipRevocation: true}.Verify(edrlab), ErrLicenseExpired)
}

// Memory: declared lengths are untrusted, and only what is needed is held in
// memory.

func TestDecryptorUntrustedOriginalLength(t *testing.T) {
	key := testKey("untrusted")
	plain := textBytes(10_000)
	for _, compress := range []bool{false, true} {
		encrypted, enc := encryptedResource(t, key, plain, compress)
		// Used to preallocate buffers, which panicked, or exhausted memory.
		enc.OriginalLength = 1 << 62
		res := Decryptor{key: key}.decrypt(encrypted, enc)

		data, rerr := res.Read(t.Context(), 0, 0)
		require.Nil(t, rerr, "compress=%v", compress)
		assert.Equal(t, plain, data)
		data, rerr = res.Read(t.Context(), 100, 199)
		require.Nil(t, rerr)
		assert.Equal(t, plain[100:200], data)

		if !compress {
			// The padding tells the actual length.
			length, rerr := res.Length(t.Context())
			require.Nil(t, rerr)
			assert.EqualValues(t, len(plain), length)
			data, rerr = res.Read(t.Context(), 9_990, 1<<40)
			require.Nil(t, rerr)
			assert.Equal(t, plain[9_990:], data)
		}
	}
}

func TestDecryptorCapsContentAtDeclaredLength(t *testing.T) {
	key := testKey("bomb")
	// 8 MiB of zeros deflate to a few kilobytes.
	bomb, enc := encryptedResource(t, key, make([]byte, 8<<20), true)
	ciphertextLength, rerr := bomb.Length(t.Context())
	require.Nil(t, rerr)
	require.Less(t, ciphertextLength, int64(64<<10))
	enc.OriginalLength = 1000
	res := Decryptor{key: key}.decrypt(bomb, enc)

	_, rerr = res.Read(t.Context(), 0, 0)
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr.Cause, ErrInvalidCiphertext)
	_, rerr = res.Stream(t.Context(), io.Discard, 0, 0)
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr.Cause, ErrInvalidCiphertext)

	data, rerr := res.Read(t.Context(), 0, 99)
	require.Nil(t, rerr)
	assert.Equal(t, make([]byte, 100), data)
}

// allocated returns the number of bytes fn allocates.
func allocated(fn func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestDecryptorCompressedRangesUseLittleMemory(t *testing.T) {
	key := testKey("ranges")
	plain := make([]byte, 32<<20)
	encrypted, enc := encryptedResource(t, key, plain, true)
	enc.OriginalLength = 0
	res := Decryptor{key: key}.decrypt(encrypted, enc)

	assert.Less(t, allocated(func() {
		data, rerr := res.Read(t.Context(), 1000, 1999)
		require.Nil(t, rerr)
		assert.Len(t, data, 1000)
	}), uint64(4<<20), "range read")
	assert.Less(t, allocated(func() {
		length, rerr := res.Length(t.Context())
		require.Nil(t, rerr)
		assert.EqualValues(t, len(plain), length)
	}), uint64(4<<20), "length")
}

func TestLicenseSizeLimit(t *testing.T) {
	data, err := os.ReadFile("testdata/license-rsa.lcpl")
	require.NoError(t, err)
	padded := bytes.Replace(data, []byte(`"provider":`), []byte(`"padding": "`+strings.Repeat("x", maxLicenseSize)+`", "provider":`), 1)
	_, err = ParseLicense(padded)
	assert.ErrorIs(t, err, ErrInvalidLicense)

	// Embedded licenses aren't read whole either.
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "META-INF"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "container.xml"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "license.lcpl"), padded, 0o644))
	_, err = ReadLicense(t.Context(), fileAsset(t, dir), Options{})
	assert.ErrorContains(t, err, "META-INF/license.lcpl is larger than")
}

// Files: transformed resources don't expose the file holding their source
// bytes, and exploded packages can't leak files from outside of them.

func TestTransformedResourcesHideTheirFile(t *testing.T) {
	key := testKey("file")
	dir := t.TempDir()
	var ciphertext bytes.Buffer
	enc, err := EncryptContent(key, bytes.NewReader(textBytes(100)), &ciphertext, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "resource"), ciphertext.Bytes(), 0o644))

	source := fetcher.NewFileFetcher("", dir).Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("resource", false)})
	require.NotEmpty(t, source.File())
	assert.Empty(t, Decryptor{key: key}.decrypt(source, enc).File())
	encrypting, err := newEncryptingResource(source, key, false, rand.Reader)
	require.NoError(t, err)
	assert.Empty(t, encrypting.File())
}

func TestEncryptRWPExplodedSkipsEscapingSymlinks(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("TOP SECRET"), 0o644))
	dir := explode(t, buildRWP(t))
	require.NoError(t, os.Symlink(secret, filepath.Join(dir, "innocent.txt")))

	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, dir), testKey("x"), &buf, Options{}))
	_, names := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	assert.NotContains(t, names, "innocent.txt")
	assert.NotContains(t, buf.String(), "TOP SECRET")
}

// Second review: links the manifest parser drops, remote links, aliases of
// files in exploded packages, licenses changed after parsing, and inflation
// beyond what any publication needs.

func TestEncryptRWPRefusesUnclassifiableLinks(t *testing.T) {
	// The manifest parser drops reading order links without a media type:
	// chapter.html would be copied in the clear as a file the manifest
	// doesn't list, while still listed in it.
	m := strings.Replace(testRWPManifest, `{"href": "chapter.html", "type": "text/html"}`, `{"href": "chapter.html"}`, 1)
	require.NotEqual(t, testRWPManifest, m)
	err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "chapter.html in manifest.json can't be classified")
}

func TestRemoteCoverDoesNotExemptLocalResources(t *testing.T) {
	m := strings.Replace(testRWPManifest, `"links": [`, `"links": [{"rel": "cover", "href": "https://cdn.example.com/chapter.html", "type": "text/html"}, `, 1)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), &buf, Options{}))
	output, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	assert.NotEqual(t, testRWPFiles["chapter.html"], output["chapter.html"].content)
	assert.Contains(t, encryptedHrefs(t, output["manifest.json"].content), "chapter.html")
}

// alias gives the file at target another name, with a symbolic or hard link.
func alias(t *testing.T, kind, target, name string) {
	t.Helper()
	var err error
	if kind == "symbolic" {
		rel, rerr := filepath.Rel(filepath.Dir(name), target)
		require.NoError(t, rerr)
		err = os.Symlink(rel, name)
	} else {
		err = os.Link(target, name)
	}
	if err != nil {
		t.Skipf("can't create a %s link: %v", kind, err)
	}
}

func TestEncryptRefusesFilesBothEncryptedAndInTheClear(t *testing.T) {
	for _, kind := range []string{"symbolic", "hard"} {
		// An unlisted name of the chapter would be copied in the clear.
		dir := explode(t, buildRWP(t))
		alias(t, kind, filepath.Join(dir, "chapter.html"), filepath.Join(dir, "alias.html"))
		err := EncryptRWP(t.Context(), fileAsset(t, dir), testKey("x"), io.Discard, Options{})
		assert.ErrorContains(t, err, "are the same file", kind)

		// Container files are copied in the clear. Unlisted names of
		// publication resources are encrypted, like the resources.
		dir = explode(t, "testdata/compat.epub")
		alias(t, kind, filepath.Join(dir, "OEBPS", "chapter1.xhtml"), filepath.Join(dir, "META-INF", "alias.xml"))
		err = EncryptEPUB(t.Context(), fileAsset(t, dir), testKey("x"), io.Discard, Options{})
		assert.ErrorContains(t, err, "are the same file", kind)
		dir = explode(t, "testdata/compat.epub")
		alias(t, kind, filepath.Join(dir, "OEBPS", "chapter1.xhtml"), filepath.Join(dir, "OEBPS", "alias.xhtml"))
		assert.NoError(t, EncryptEPUB(t.Context(), fileAsset(t, dir), testKey("x"), io.Discard, Options{}), kind)

		// The cover, served in the clear, is the chapter.
		dir = t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
		alias(t, kind, filepath.Join(dir, "chapter.html"), filepath.Join(dir, "cover.jpg"))
		cover := manifest.Link{Href: manifest.MustNewHREFFromString("cover.jpg", false), Rels: manifest.Strings{"cover"}, MediaType: &mediatype.JPEG}
		chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
		p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{cover, chapter}}, fetcher.NewFileFetcher("", dir), nil)
		_, err = EncryptPublication(t.Context(), p, testKey("x"), Options{})
		assert.ErrorContains(t, err, "are the same file", kind)
	}

	// Names which are all encrypted, or all in the clear, are fine.
	dir := explode(t, buildRWP(t))
	alias(t, "symbolic", filepath.Join(dir, "extra", "unlisted.json"), filepath.Join(dir, "extra", "alias.json"))
	assert.NoError(t, EncryptRWP(t.Context(), fileAsset(t, dir), testKey("x"), io.Discard, Options{}))
}

func TestModifiedLicenseFailsVerification(t *testing.T) {
	// An expired license whose end date is extended after parsing.
	l := loadLicense(t, "edrlab-license.lcpl")
	future := time.Now().AddDate(10, 0, 0)
	l.Rights.End = &future
	roots := loadRoots(t, "edrlab-test-root-ca-2016.pem")
	assert.ErrorIs(t, l.VerifySignature(roots), ErrInvalidSignature)
	assert.ErrorIs(t, l.VerifyIntegrity(), ErrInvalidSignature)
	assert.ErrorIs(t, Verifier{Roots: roots, InsecureSkipRevocation: true}.Verify(l), ErrInvalidSignature)

	// ContentProtection validates a copy parsed from the license's document,
	// whatever was done to the License given to it.
	cp := ContentProtection{License: l, Verify: Verifier{Roots: roots, InsecureSkipRevocation: true}.Verify}
	err := openProtected(t, "testdata/compat-old-lcp.epub", cp, testPassphrase)
	assert.ErrorIs(t, err, ErrLicenseExpired)
}

func TestInflationLimit(t *testing.T) {
	assert.EqualValues(t, DefaultMaxInflatedLength, inflationLimit(0))
	assert.EqualValues(t, -1, inflationLimit(-1))
	assert.EqualValues(t, 1000, inflationLimit(1000))

	key := testKey("limit")
	plain := make([]byte, 2<<20)
	encrypted, enc := encryptedResource(t, key, plain, true)
	enc.OriginalLength = 0

	// Reading a deflated resource whole is limited.
	limited := Decryptor{key: key, maxInflated: 1 << 20}.decrypt(encrypted, enc)
	_, rerr := limited.Read(t.Context(), 0, 0)
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr, fetcher.ErrResourceTooLarge)
	assert.ErrorIs(t, rerr, ErrResourceTooLarge)
	assert.ErrorContains(t, rerr, "inflates beyond 1 MiB")

	// Not streaming it, nor reading small ranges of it, wherever they are:
	// large audio or video files may be deflated.
	n, rerr := limited.Stream(t.Context(), io.Discard, 0, 0)
	require.Nil(t, rerr)
	assert.EqualValues(t, len(plain), n)
	length, rerr := limited.Length(t.Context())
	require.Nil(t, rerr)
	assert.EqualValues(t, len(plain), length)
	data, rerr := limited.Read(t.Context(), 1<<20, 1<<20+99)
	require.Nil(t, rerr)
	assert.Len(t, data, 100)

	unlimited := Decryptor{key: key, maxInflated: -1}.decrypt(encrypted, enc)
	data, rerr = unlimited.Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, plain, data)

	// Uncompressed resources are as large as their ciphertext.
	encrypted, enc = encryptedResource(t, key, plain, false)
	data, rerr = Decryptor{key: key, maxInflated: 1 << 20}.decrypt(encrypted, enc).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, plain, data)
}

func TestInflationLimitContentProtection(t *testing.T) {
	// The chapter of the fixture is deflated, and larger than 100 bytes.
	licensed := licensedEPUB(t, "license-rsa.lcpl")
	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{InsecureSkipVerify: true, MaxInflatedLength: 100}},
	}).Open(t.Context(), fileAsset(t, licensed), testPassphrase)
	require.NoError(t, err)
	defer p.Close()
	chapter := p.Manifest.ReadingOrder[0]
	_, rerr := p.Get(t.Context(), chapter).Read(t.Context(), 0, 0)
	require.NotNil(t, rerr, chapter.Href)
	assert.ErrorIs(t, rerr, ErrResourceTooLarge)
	_, rerr = p.Get(t.Context(), chapter).Stream(t.Context(), io.Discard, 0, 0)
	assert.Nil(t, rerr)
}

// Third review: legacy manifests, reserved service paths, exploded archives,
// range limits, ciphertext finalization, revocation and the canonical form
// of the specification.

func TestEncryptRWPLegacySpine(t *testing.T) {
	// The manifest parser takes the legacy "spine" for the reading order.
	spine := strings.Replace(testRWPManifest, `"readingOrder"`, `"spine"`, 1)
	key := testKey("spine")
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, spine)), key, &buf, Options{}))
	encrypted := writeTemp(t, "encrypted.webpub", buf.Bytes())
	output, _ := readZip(t, encrypted)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(output["manifest.json"].content, &raw))
	declared := 0
	walkRawLinks(raw, func(link map[string]any) {
		if props, _ := link["properties"].(map[string]any); props["encrypted"] != nil {
			declared++
		}
	})
	assert.Equal(t, 5, declared, "every encrypted resource of the spine declares its encryption")

	var restored bytes.Buffer
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, encrypted), key, &restored, Options{}))
	entries, _ := readZip(t, writeTemp(t, "restored.webpub", restored.Bytes()))
	assert.Equal(t, testRWPFiles["chapter.html"], entries["chapter.html"].content)

	untyped := strings.Replace(spine, `{"href": "chapter.html", "type": "text/html"}`, `{"href": "chapter.html"}`, 1)
	err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, untyped)), key, io.Discard, Options{})
	assert.ErrorContains(t, err, "chapter.html in manifest.json can't be classified")
}

// zipFiles writes a ZIP file holding the given files, in name order, and
// returns its path.
func zipFiles(t *testing.T, name string, files map[string][]byte) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, path := range slices.Sorted(maps.Keys(files)) {
		w, err := zw.Create(path)
		require.NoError(t, err)
		_, err = w.Write(files[path])
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return writeTemp(t, name, buf.Bytes())
}

func TestEncryptRWPRefusesReservedPaths(t *testing.T) {
	// Links under ~readium/ are taken for links to publication services.
	m := strings.Replace(testRWPManifest, `"href": "chapter.html"`, `"href": "~readium/chapter.html"`, 1)
	files := map[string][]byte{"manifest.json": []byte(m)}
	for name, data := range testRWPFiles {
		files[name] = data
	}
	files["~readium/chapter.html"] = files["chapter.html"]
	delete(files, "chapter.html")
	err := EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "reserved.webpub", files)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "reserved for publication services")
}

func TestEncryptPublicationExplodedArchiveAliases(t *testing.T) {
	// Exploded archives expose the files of their entries as well.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
	alias(t, "symbolic", filepath.Join(dir, "chapter.html"), filepath.Join(dir, "cover.jpg"))
	cover := manifest.Link{Href: manifest.MustNewHREFFromString("cover.jpg", false), Rels: manifest.Strings{"cover"}, MediaType: &mediatype.JPEG}
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	f := fetcher.NewArchiveFetcher(archive.NewExplodedArchive(dir))
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{cover, chapter}}, f, nil)
	_, err := EncryptPublication(t.Context(), p, testKey("x"), Options{})
	assert.ErrorContains(t, err, "are the same file")
}

func TestRangesEndingAtMaxInt64(t *testing.T) {
	key := testKey("max")
	plain := textBytes(1000)
	for _, compress := range []bool{false, true} {
		encrypted, enc := encryptedResource(t, key, plain, compress)
		// The exclusive end of the range used to overflow: a panic for
		// deflated resources, an empty range otherwise.
		data, rerr := Decryptor{key: key}.decrypt(encrypted, enc).Read(t.Context(), 10, math.MaxInt64)
		require.Nil(t, rerr, "compress=%v", compress)
		assert.Equal(t, plain[10:], data, "compress=%v", compress)

		source := fetcher.NewBytesResource(manifest.Link{}, func() []byte { return plain })
		er, err := newEncryptingResource(source, key, compress, rand.Reader)
		require.NoError(t, err)
		ciphertext, rerr := er.Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		data, rerr = er.Read(t.Context(), 10, math.MaxInt64)
		require.Nil(t, rerr)
		assert.Equal(t, ciphertext[10:], data, "compress=%v", compress)
	}
}

func TestDecryptContentFinalizesDeflatedContent(t *testing.T) {
	key := testKey("final")
	// Deflated content ending on a block boundary, followed by a block of
	// padding which the inflater doesn't need. The padding is written
	// directly: altering the ciphertext would garble the block, whose last
	// byte would still be a valid padding length once in sixteen.
	var deflated []byte
	for size := 1000; deflated == nil; size++ {
		var buf bytes.Buffer
		fw, err := flate.NewWriter(&buf, flate.BestCompression)
		require.NoError(t, err)
		_, err = fw.Write(textBytes(size))
		require.NoError(t, err)
		require.NoError(t, fw.Close())
		if buf.Len()%blockSize == 0 {
			deflated = buf.Bytes()
		}
	}
	enc := manifest.Encryption{Algorithm: AlgorithmAES256CBC, Compression: CompressionDeflate}
	for _, pad := range []byte{blockSize, 0, blockSize + 1, 0xff} {
		plaintext := append(bytes.Clone(deflated), bytes.Repeat([]byte{pad}, blockSize)...)
		_, err := DecryptContent(key, bytes.NewReader(encryptBlocks(t, key, plaintext)), io.Discard, enc)
		if pad == blockSize {
			assert.NoError(t, err, "valid padding")
		} else {
			assert.ErrorIs(t, err, ErrInvalidPadding, "padding %d", pad)
		}
	}

	// Data after the deflated content.
	var ciphertext bytes.Buffer
	_, err := Encrypt(key, io.MultiReader(bytes.NewReader(deflated), strings.NewReader("appended")), &ciphertext)
	require.NoError(t, err)
	_, err = DecryptContent(key, &ciphertext, io.Discard, enc)
	assert.ErrorContains(t, err, "data after the deflated content")
}

// encryptBlocks encrypts plaintext, whose length is a multiple of the block
// size, with AES-CBC and a zero IV prefixed to the ciphertext, without
// padding it.
func encryptBlocks(t *testing.T, key ContentKey, plaintext []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	out := make([]byte, blockSize+len(plaintext))
	cipher.NewCBCEncrypter(block, out[:blockSize]).CryptBlocks(out[blockSize:], plaintext)
	return out
}

func TestRevokedBy(t *testing.T) {
	p := newEd25519Provider(t)
	l := loadLicense(t, "license-rsa.lcpl")
	canonical, err := l.canonicalJSON()
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(l.Raw(), &doc))
	signed := p.sign(t, doc, canonical)
	roots := x509.NewCertPool()
	roots.AddCert(p.root)
	provider, err := x509.ParseCertificate(p.der)
	require.NoError(t, err)

	crl := func(serials ...*big.Int) *x509.RevocationList {
		var entries []x509.RevocationListEntry
		for _, serial := range serials {
			entries = append(entries, x509.RevocationListEntry{SerialNumber: serial, RevocationTime: time.Now()})
		}
		der, err := x509.CreateRevocationList(nil, &x509.RevocationList{
			Number:                    big.NewInt(1),
			ThisUpdate:                time.Now(),
			NextUpdate:                time.Now().AddDate(0, 0, 7),
			RevokedCertificateEntries: entries,
		}, p.root, p.rootKey)
		require.NoError(t, err)
		list, err := x509.ParseRevocationList(der)
		require.NoError(t, err)
		require.NoError(t, list.CheckSignatureFrom(p.root))
		return list
	}

	assert.NoError(t, Verifier{Roots: roots, IsRevoked: RevokedBy(crl(big.NewInt(99)))}.Verify(signed))
	err = Verifier{Roots: roots, IsRevoked: RevokedBy(crl(big.NewInt(99), provider.SerialNumber))}.Verify(signed)
	assert.ErrorIs(t, err, ErrCertificateRevoked)
	// Only the lists of the provider's issuer apply, even when another
	// authority has the same name.
	other := newEd25519Provider(t)
	require.Equal(t, p.root.Subject.String(), other.root.Subject.String())
	foreign, err := x509.CreateRevocationList(nil, &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now(),
		NextUpdate:                time.Now().AddDate(0, 0, 7),
		RevokedCertificateEntries: []x509.RevocationListEntry{{SerialNumber: provider.SerialNumber, RevocationTime: time.Now()}},
	}, other.root, other.rootKey)
	require.NoError(t, err)
	foreignList, err := x509.ParseRevocationList(foreign)
	require.NoError(t, err)
	// Without a list of the provider's issuer, whether its certificate was
	// revoked is unknown, which used to pass for not revoked.
	for name, isRevoked := range map[string]func(_, _ *x509.Certificate) (bool, error){
		"foreign list": RevokedBy(foreignList),
		"no list":      RevokedBy(),
	} {
		err = Verifier{Roots: roots, IsRevoked: isRevoked}.Verify(signed)
		assert.ErrorIs(t, err, ErrNoRevocationList, name)
		assert.NotErrorIs(t, err, ErrCertificateRevoked, name)
	}
	// Callers accept such certificates explicitly.
	isRevoked := RevokedBy(foreignList)
	assert.NoError(t, Verifier{Roots: roots, IsRevoked: func(provider, issuer *x509.Certificate) (bool, error) {
		revoked, err := isRevoked(provider, issuer)
		if errors.Is(err, ErrNoRevocationList) {
			return false, nil
		}
		return revoked, err
	}}.Verify(signed))
	// The issuer's list still applies alongside foreign ones.
	err = Verifier{Roots: roots, IsRevoked: RevokedBy(foreignList, crl(provider.SerialNumber))}.Verify(signed)
	assert.ErrorIs(t, err, ErrCertificateRevoked)
}

func TestCanonicalNumber(t *testing.T) {
	for literal, expected := range map[string]string{
		"0": "0", "-0": "0", "0.0": "0", "10": "10", "-10": "-10", "10.0": "10", "1E1": "10",
		"1e+2": "100", "100E-2": "1", "0.5": "5E-1", "1.25": "1.25E0", "12.5": "1.25E1",
		"-0.00125": "-1.25E-3", "1.5e10": "15000000000", "123.4500": "1.2345E2",
	} {
		actual, err := canonicalNumber(literal)
		require.NoError(t, err, literal)
		assert.Equal(t, expected, actual, literal)
	}

	// Exponents used to overflow at the bounds of int: 10e9223372036854775807
	// had the canonical form of 1e-9223372036854775808.
	for _, literal := range []string{"10e9223372036854775807", "1e-9223372036854775808", "1e2147483648", "1e-2147483649", "10e1000", "1e1001"} {
		_, err := canonicalNumber(literal)
		assert.ErrorContains(t, err, "out of range", literal)
	}
	for literal, expected := range map[string]string{"1e-2147483648": "1E-2147483648", "1.5e-2147483647": "1.5E-2147483647"} {
		actual, err := canonicalNumber(literal)
		require.NoError(t, err, literal)
		assert.Equal(t, expected, actual, literal)
	}
}

func TestCanonicalForm(t *testing.T) {
	doc, err := decodeDocument([]byte(`{"b": "<a href=\"/x\"> \u0001\n</a> é", "a": [3, 1.50, {"d": true, "c": null}]}`))
	require.NoError(t, err)
	canonical, err := canonicalize(doc)
	require.NoError(t, err)
	// Only quotation marks, backslashes and control characters are escaped.
	assert.Equal(t, "{\"a\":[3,1.5E0,{\"c\":null,\"d\":true}],\"b\":\"<a href=\\\"/x\\\"> \\u0001\\u000A</a> é\"}", string(canonical))

	// A conforming provider's license verifies, as well as one from
	// readium-lcp-server, whose canonical form differs from the
	// specification's for such content. Other serializations don't.
	p := newEd25519Provider(t)
	l := loadLicense(t, "license-rsa.lcpl")
	license, err := decodeDocument(l.Raw())
	require.NoError(t, err)
	license["encryption"].(map[string]any)["user_key"].(map[string]any)["text_hint"] = "<b>Line separated</b>\u0001"
	license["x-extension"] = json.Number("1.50")
	forms := map[string]func(map[string]any) ([]byte, error){
		"specification":      canonicalize,
		"readium-lcp-server": canonicalizeLegacy,
		"HTML-escaped": func(doc map[string]any) ([]byte, error) {
			return json.Marshal(doc)
		},
	}
	for name, form := range forms {
		unsigned := maps.Clone(license)
		delete(unsigned, "signature")
		message, err := form(unsigned)
		require.NoError(t, err, name)
		signed := p.sign(t, unsigned, message)
		if name == "HTML-escaped" {
			assert.ErrorIs(t, signed.VerifyIntegrity(), ErrInvalidSignature, name)
		} else {
			assert.NoError(t, signed.VerifyIntegrity(), name)
		}
	}
}

// Fourth review: URL templates, font obfuscation declared for other
// resources, ranges held in memory, the size of the canonical form of
// licenses, and issuers without revocation list (see TestRevokedBy).

func TestEncryptRWPRefusesLocalTemplates(t *testing.T) {
	// A template expanding to a path of the package wasn't classified, and
	// its resource was copied in the clear.
	for _, href := range []string{"chapter.html", "chapter.html{?page}", "{+path}"} {
		m := strings.Replace(testRWPManifest, `{"href": "chapter.html", "type": "text/html"}`, `{"href": "`+href+`", "templated": true, "type": "text/html"}`, 1)
		err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{})
		assert.ErrorContains(t, err, "is a URL template expanding to paths of the package", href)
	}
	// Remote templates point outside of the package.
	m := strings.Replace(testRWPManifest, `"resources": [`, `"resources": [{"href": "https://example.com/search{?q}", "templated": true, "type": "text/html"},`, 1)
	assert.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{}))

	// EncryptPublication doesn't serve them.
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", true), MediaType: &mediatype.HTML}
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}}, archiveFetcher(t, buildRWP(t)), nil)
	encrypted, err := EncryptPublication(t.Context(), p, testKey("x"), Options{})
	require.NoError(t, err)
	_, rerr := encrypted.Get(t.Context(), chapter).Read(t.Context(), 0, 0)
	require.NotNil(t, rerr)
	assert.Equal(t, fetcher.CodeNotFound, rerr.Code)
}

const obfuscation = `"properties": {"encrypted": {"algorithm": "http://www.idpf.org/2008/embedding"}}`

func TestEncryptRWPObfuscationOnlyExemptsFonts(t *testing.T) {
	// Declaring font obfuscation for a chapter left it in the clear.
	m := strings.Replace(testRWPManifest, `{"href": "chapter.html", "type": "text/html"}`, `{"href": "chapter.html", "type": "text/html", `+obfuscation+`}`, 1)
	err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "chapter.html is declared as an obfuscated font, but it is not a font (text/html)")

	// Every link to an obfuscated resource must be a font.
	m = strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"}`, `{"href": "chapter.html", "type": "font/otf", `+obfuscation+`}, {"href": "style.css", "type": "text/css"}`, 1)
	err = EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "chapter.html is declared as an obfuscated font, but it is not a font (text/html)")

	// Fonts keep their obfuscation, and are copied as they are.
	files := maps.Clone(testRWPFiles)
	files["font.otf"] = randomBytes(2000)
	m = strings.Replace(testRWPManifest, `"resources": [`, `"resources": [{"href": "font.otf", "type": "font/otf", `+obfuscation+`},`, 1)
	files["manifest.json"] = []byte(m)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "font.webpub", files)), testKey("x"), &buf, Options{}))
	output, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	assert.Equal(t, files["font.otf"], output["font.otf"].content)
	assert.Equal(t, protection.AlgorithmIDPFObfuscation, encryptedHrefs(t, output["manifest.json"].content)["font.otf"].Algorithm)
}

func TestEncryptPublicationObfuscationOnlyExemptsFonts(t *testing.T) {
	chapter := manifest.Link{
		Href:       manifest.MustNewHREFFromString("chapter.html", false),
		MediaType:  &mediatype.HTML,
		Properties: withEncryption(nil, &manifest.Encryption{Algorithm: protection.AlgorithmIDPFObfuscation}),
	}
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}}, archiveFetcher(t, buildRWP(t)), nil)
	_, err := EncryptPublication(t.Context(), p, testKey("x"), Options{})
	assert.ErrorContains(t, err, "chapter.html is declared as an obfuscated font, but it is not a font (text/html)")
}

// obfuscatedEPUB writes an EPUB whose package document lists the given
// items, and whose encryption.xml declares font obfuscation for the given
// paths, and returns its path.
func obfuscatedEPUB(t *testing.T, items string, obfuscated []string, files map[string][]byte) string {
	t.Helper()
	var refs strings.Builder
	for _, p := range obfuscated {
		fmt.Fprintf(&refs, `<enc:EncryptedData><enc:EncryptionMethod Algorithm="%s"/><enc:CipherData><enc:CipherReference URI="%s"/></enc:CipherData></enc:EncryptedData>`, protection.AlgorithmIDPFObfuscation, p)
	}
	all := map[string][]byte{
		"mimetype":                []byte("application/epub+zip"),
		"META-INF/container.xml":  []byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OPS/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"META-INF/encryption.xml": []byte(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#">` + refs.String() + `</encryption>`),
		"OPS/package.opf":         []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">urn:x</dc:identifier><dc:title>T</dc:title><dc:language>en</dc:language></metadata><manifest><item id="c" href="chapter.xhtml" media-type="application/xhtml+xml"/>` + items + `</manifest><spine><itemref idref="c"/></spine></package>`),
		"OPS/chapter.xhtml":       textBytes(3000),
	}
	maps.Copy(all, files)
	return zipFiles(t, "obfuscated.epub", all)
}

func TestEncryptEPUBObfuscationOnlyExemptsFonts(t *testing.T) {
	encrypt := func(path string) error {
		return EncryptEPUB(t.Context(), fileAsset(t, path), testKey("x"), io.Discard, Options{})
	}
	// Declaring font obfuscation for a chapter left it in the clear, listed
	// in the package document or not.
	err := encrypt(obfuscatedEPUB(t, "", []string{"OPS/chapter.xhtml"}, nil))
	assert.ErrorContains(t, err, "OPS/chapter.xhtml is declared as an obfuscated font, but it is not a font (application/xhtml+xml)")
	err = encrypt(obfuscatedEPUB(t, "", []string{"OPS/notes.html"}, map[string][]byte{"OPS/notes.html": textBytes(100)}))
	assert.ErrorContains(t, err, "OPS/notes.html is declared as an obfuscated font, but it is not a font (text/html)")

	// Fonts keep their obfuscation, whatever font media type they are listed
	// with, and are copied as they are.
	fonts := map[string][]byte{"OPS/listed.otf": randomBytes(2000), "OPS/unlisted.ttf": randomBytes(2000)}
	source := obfuscatedEPUB(t, `<item id="f" href="listed.otf" media-type="application/vnd.ms-opentype"/>`, []string{"OPS/listed.otf", "OPS/unlisted.ttf"}, fonts)
	encrypted := encryptEPUBToFile(t, source, testKey("x"), Options{})
	output, _ := readZip(t, encrypted)
	entries := encryptionEntries(t, encrypted)
	for path, data := range fonts {
		assert.Equal(t, data, output[path].content, path)
		assert.Equal(t, protection.AlgorithmIDPFObfuscation, entries[path].Algorithm, path)
	}
	chapter := entries["OPS/chapter.xhtml"]
	assert.True(t, IsLCPEncrypted(&chapter))
}

func TestDecryptorRangesHeldInMemoryAreLimited(t *testing.T) {
	key := testKey("range-limit")
	plain := make([]byte, 4<<20)
	encrypted, enc := encryptedResource(t, key, plain, true)
	enc.OriginalLength = 0
	limited := Decryptor{key: key, maxInflated: 1 << 20}.decrypt(encrypted, enc)

	// A range spanning the resource used to bypass the limit.
	var rerr *fetcher.ResourceError
	assert.Less(t, allocated(func() {
		_, rerr = limited.Read(t.Context(), 1, math.MaxInt64-1)
	}), uint64(8<<20))
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr, ErrResourceTooLarge)

	// Ranges within the limit are read, wherever they are.
	data, rerr := limited.Read(t.Context(), 3<<20, 3<<20+999)
	require.Nil(t, rerr)
	assert.Len(t, data, 1000)
}

func TestRangedStreamsUseLittleMemory(t *testing.T) {
	// Ranged streams used to be read into memory before being written, e.g.
	// an HTTP request for bytes=1000- of a large video.
	key := testKey("range-stream")
	plain := make([]byte, 32<<20)
	for i := range plain {
		plain[i] = byte(i >> 12)
	}
	expected := sha256.Sum256(plain[1000:])
	check := func(name string, res fetcher.Resource, contentLength int) {
		h := sha256.New()
		var n int64
		var rerr *fetcher.ResourceError
		assert.Less(t, allocated(func() {
			n, rerr = res.Stream(t.Context(), h, 1000, math.MaxInt64)
		}), uint64(4<<20), name)
		require.Nil(t, rerr, name)
		assert.EqualValues(t, contentLength-1000, n, name)
		if contentLength == len(plain) {
			assert.Equal(t, expected[:], h.Sum(nil), name)
		}
	}
	for _, compress := range []bool{false, true} {
		encrypted, enc := encryptedResource(t, key, plain, compress)
		check(fmt.Sprintf("decrypting, compress=%v", compress), Decryptor{key: key}.decrypt(encrypted, enc), len(plain))
	}
	source := fetcher.NewBytesResource(manifest.Link{}, func() []byte { return plain })
	er, err := newEncryptingResource(source, key, false, rand.Reader)
	require.NoError(t, err)
	check("encrypting", er, int(EncryptedLength(int64(len(plain)))))
}

func TestCanonicalFormSizeLimit(t *testing.T) {
	// Exponents expand into zeroes: 1e1000 takes 6 bytes, its canonical form
	// 1001. A license of 700 kB used to take hundreds of megabytes.
	data, err := os.ReadFile("testdata/license-rsa.lcpl")
	require.NoError(t, err)
	numbers := "[" + strings.Repeat("1e1000,", 100_000) + "1]"
	data = bytes.Replace(data, []byte(`"provider":`), []byte(`"x-numbers": `+numbers+`, "provider":`), 1)
	require.Less(t, len(data), maxLicenseSize)
	assert.Less(t, allocated(func() {
		_, err = ParseLicense(data)
	}), uint64(64<<20))
	assert.ErrorIs(t, err, ErrInvalidLicense)
	assert.ErrorContains(t, err, "canonical form larger than 1 MiB")
}

// Compressions

func TestDecryptRefusesUnknownCompressions(t *testing.T) {
	key := testKey("compression")
	plain := textBytes(5000)

	// "none", which the EPUB parser declares for Method "0", is no
	// compression.
	res, enc := encryptedResource(t, key, plain, false)
	enc.Compression = protection.CompressionNone
	data, rerr := Decryptor{key: key}.decrypt(res, enc).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, plain, data)

	// Compressions this package can't undo used to be taken for none,
	// serving the content still compressed.
	const unsupported = `lcp: unsupported compression "93"`
	for _, compress := range []bool{false, true} {
		res, enc := encryptedResource(t, key, plain, compress)
		ciphertext, rerr := res.Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		enc.Compression = "93"

		decrypted := Decryptor{key: key}.decrypt(res, enc)
		for _, r := range [][2]int64{{0, 0}, {0, 99}} {
			_, rerr := decrypted.Read(t.Context(), r[0], r[1])
			require.NotNil(t, rerr, r)
			assert.ErrorContains(t, rerr, unsupported, r)
		}
		_, rerr = decrypted.Stream(t.Context(), io.Discard, 0, 99)
		require.NotNil(t, rerr)
		assert.ErrorContains(t, rerr, unsupported)

		_, err := DecryptContent(key, bytes.NewReader(ciphertext), io.Discard, enc)
		assert.ErrorContains(t, err, unsupported)
	}
}

// Fifth review

func TestIsRemote(t *testing.T) {
	for href, remote := range map[string]bool{
		"https://cdn.example.com/chapter.html": true,
		// Network-path references have a host, without a scheme.
		"//cdn.example.com/chapter.html": true,
		"//user@/chapter.html":           true,
		"///chapter.html":                true,
		"/chapter.html":                  false,
		"chapter.html":                   false,
		"images/../chapter.html":         false,
	} {
		assert.Equal(t, remote, isRemote(manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}), href)
	}
}

func TestEncryptRWPNetworkPathReferences(t *testing.T) {
	// A remote cover sharing the path of a chapter left it in the clear, and
	// remote resources sharing the path of local ones were declared
	// encrypted.
	m := strings.Replace(testRWPManifest, `"links": [`, `"links": [{"rel": "cover", "href": "//cdn.example.com/chapter.html", "type": "image/jpeg"},`, 1)
	m = strings.Replace(m, `"resources": [`, `"resources": [{"href": "https://cdn.example.com/style.css", "type": "text/css"}, {"href": "//cdn.example.com/images/figure.png", "type": "image/png"},`, 1)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), &buf, Options{}))
	output, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	assert.NotEqual(t, testRWPFiles["chapter.html"], output["chapter.html"].content)
	assert.Equal(t, testRWPFiles["images/cover.jpg"], output["images/cover.jpg"].content, "the local cover stays in the clear")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(output["manifest.json"].content, &doc))
	encryptedHrefs := make(map[string]bool)
	for _, r := range doc["resources"].([]any) {
		l := r.(map[string]any)
		props, _ := l["properties"].(map[string]any)
		encryptedHrefs[l["href"].(string)] = props["encrypted"] != nil
	}
	assert.Equal(t, map[string]bool{
		"https://cdn.example.com/style.css":   false,
		"//cdn.example.com/images/figure.png": false,
		"style.css":                           true,
		"images/figure.png":                   true,
	}, encryptedHrefs)
}

func TestEncryptPublicationNetworkPathReferences(t *testing.T) {
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	remote := manifest.Link{Href: manifest.MustNewHREFFromString("//cdn.example.com/chapter.html", false), MediaType: &mediatype.JPEG, Rels: manifest.Strings{"cover"}}
	// A fetcher may serve a remote link from the resource of its path.
	f := pathFetcher{archiveFetcher(t, buildRWP(t))}
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Links: manifest.LinkList{remote}}, f, nil)
	encrypted, err := EncryptPublication(t.Context(), p, testKey("x"), Options{})
	require.NoError(t, err)
	data, rerr := encrypted.Get(t.Context(), chapter).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.NotEqual(t, testRWPFiles["chapter.html"], data)
	// Remote links aren't resources of the publication.
	_, rerr = encrypted.Get(t.Context(), remote).Read(t.Context(), 0, 0)
	require.NotNil(t, rerr)
	assert.Equal(t, fetcher.CodeNotFound, rerr.Code)
}

// pathFetcher serves links from the resource of their path, whatever their
// host.
type pathFetcher struct{ fetcher.Fetcher }

func (f pathFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	link.Href = manifest.MustNewHREFFromString(resourceKey(link.Href), false)
	return f.Fetcher.Get(ctx, link)
}

func TestRevokedByIdentifiesIssuersByKey(t *testing.T) {
	newCA := func() (*x509.Certificate, ed25519.PrivateKey) {
		pub, key, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Renewed CA"},
			NotBefore: mustTime("2020-01-01T00:00:00Z"), NotAfter: mustTime("2060-01-01T00:00:00Z"),
			IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		}
		der, err := x509.CreateCertificate(nil, tmpl, tmpl, pub, key)
		require.NoError(t, err)
		ca, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		return ca, key
	}
	crl := func(ca *x509.Certificate, key ed25519.PrivateKey, serials ...*big.Int) *x509.RevocationList {
		var entries []x509.RevocationListEntry
		for _, serial := range serials {
			entries = append(entries, x509.RevocationListEntry{SerialNumber: serial, RevocationTime: time.Now()})
		}
		der, err := x509.CreateRevocationList(nil, &x509.RevocationList{
			Number: big.NewInt(1), ThisUpdate: time.Now(), NextUpdate: time.Now().AddDate(0, 0, 7),
			RevokedCertificateEntries: entries,
		}, ca, key)
		require.NoError(t, err)
		list, err := x509.ParseRevocationList(der)
		require.NoError(t, err)
		return list
	}
	// Two authorities with the same name, like EDRLab's renewed test root,
	// and a provider certificate without authority key identifier, like
	// EDRLab's.
	old, oldKey := newCA()
	renewed, renewedKey := newCA()
	parent := *old
	parent.SubjectKeyId = nil
	providerPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	der, err := x509.CreateCertificate(nil, &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "Provider"},
		NotBefore: mustTime("2021-01-01T00:00:00Z"), NotAfter: mustTime("2050-01-01T00:00:00Z"),
	}, &parent, providerPub, oldKey)
	require.NoError(t, err)
	provider, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	require.Empty(t, provider.AuthorityKeyId)

	// The list of the renewed authority used to cover the provider, which
	// passed for not revoked.
	_, err = RevokedBy(crl(renewed, renewedKey))(provider, old)
	assert.ErrorIs(t, err, ErrNoRevocationList)
	revoked, err := RevokedBy(crl(renewed, renewedKey), crl(old, oldKey, provider.SerialNumber))(provider, old)
	require.NoError(t, err)
	assert.True(t, revoked)
	revoked, err = RevokedBy(crl(old, oldKey))(provider, old)
	require.NoError(t, err)
	assert.False(t, revoked)
	_, err = RevokedBy(crl(old, oldKey))(provider, nil)
	assert.ErrorIs(t, err, ErrNoRevocationList, "without a verified issuer")
}

func TestVerifierStrictProfile(t *testing.T) {
	roots := loadRoots(t, "test-root-ca.pem")
	strict := Verifier{Roots: roots, InsecureSkipRevocation: true, StrictProfile: true}

	// The Basic profile requires RSA with SHA-256.
	assert.NoError(t, strict.Verify(loadLicense(t, "license-rsa.lcpl")))
	ecdsa := loadLicense(t, "license-ecdsa.lcpl")
	assert.NoError(t, Verifier{Roots: roots, InsecureSkipRevocation: true}.Verify(ecdsa), "like EDRLab's License Server, unless strict")
	err := strict.Verify(ecdsa)
	assert.ErrorIs(t, err, ErrInvalidSignature)
	assert.ErrorContains(t, err, "isn't the one of the Basic profile")
	ed, edRoots := ed25519License(t)
	assert.ErrorIs(t, Verifier{Roots: edRoots, InsecureSkipRevocation: true, StrictProfile: true}.Verify(ed), ErrInvalidSignature)

	// The algorithms of other profiles aren't known.
	p := newEd25519Provider(t)
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(loadLicense(t, "license-rsa.lcpl").Raw()))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&doc))
	doc["encryption"].(map[string]any)["profile"] = "http://readium.org/lcp/profile-1.0"
	delete(doc, "signature")
	canonical, err := canonicalize(doc)
	require.NoError(t, err)
	production := p.sign(t, doc, canonical)
	pool := x509.NewCertPool()
	pool.AddCert(p.root)
	assert.NoError(t, Verifier{Roots: pool, InsecureSkipRevocation: true, StrictProfile: true}.Verify(production))
}

// isLCPProtectedPackage is isLCPProtected for the format the files of the
// package tell.
func isLCPProtectedPackage(t *testing.T, f fetcher.Fetcher) (bool, error) {
	format, err := detectFormat(t.Context(), f)
	if err != nil {
		return false, err
	}
	return isLCPProtected(t.Context(), f, format)
}

func TestIsLCPProtectedKeepsTheManifestSizeLimit(t *testing.T) {
	// An oversized manifest.json passed for a publication which isn't a
	// Readium Web Publication package, and the parser then read it whole.
	oversized := strings.Replace(testRWPManifest, `"title": "Test",`, `"title": "`+strings.Repeat("x", maxManifestSize)+`",`, 1)
	_, err := isLCPProtectedPackage(t, archiveFetcher(t, buildRWPWith(t, oversized)))
	assert.ErrorContains(t, err, "manifest.json is larger than 32 MiB")

	for name, files := range map[string]map[string][]byte{
		"no manifest":      {"image.jpg": randomBytes(10)},
		"invalid manifest": {"manifest.json": []byte("{")},
	} {
		protected, err := isLCPProtectedPackage(t, archiveFetcher(t, zipFiles(t, "package.zip", files)))
		require.NoError(t, err, name)
		assert.False(t, protected, name)
	}
}

// Sixth review

func TestInjectLicenseRequiresEncryptedResources(t *testing.T) {
	license := loadLicense(t, "license-rsa.lcpl")
	inject := func(path string) error {
		return InjectLicense(t.Context(), fileAsset(t, path), license, io.Discard, Options{})
	}

	// A publication in the clear holding a license used to be taken for a
	// protected one, and delivered as such.
	clear, _ := readZip(t, "testdata/compat.epub")
	epub := map[string][]byte{protection.PathEPUBLCPLicense: license.Raw()}
	for name, e := range clear {
		epub[name] = e.content
	}
	err := inject(zipFiles(t, "clear.epub", epub))
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "no resource of the package is declared LCP-encrypted")

	rwp := map[string][]byte{"manifest.json": []byte(testRWPManifest), protection.PathRWPLCPLicense: license.Raw()}
	for name, data := range testRWPFiles {
		rwp[name] = data
	}
	assert.ErrorIs(t, inject(zipFiles(t, "clear.webpub", rwp)), ErrNotProtected)

	// So were leftover declarations of resources which aren't encrypted.
	lcpEncrypted := `"properties": {"encrypted": {"scheme": "http://readium.org/2014/01/lcp", "algorithm": "http://www.w3.org/2001/04/xmlenc#aes256-cbc"}}`
	rwp["manifest.json"] = []byte(strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"}`, `{"href": "style.css", "type": "text/css", `+lcpEncrypted+`}`, 1))
	err = inject(zipFiles(t, "leftover.webpub", rwp))
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "style.css is declared LCP-encrypted, but its 300 bytes can't be an AES-CBC ciphertext")
	rwp["manifest.json"] = []byte(strings.Replace(testRWPManifest, `"resources": [`, `"resources": [{"href": "missing.css", "type": "text/css", `+lcpEncrypted+`},`, 1))
	assert.ErrorIs(t, inject(zipFiles(t, "missing.webpub", rwp)), ErrNotProtected, "declared resources must exist")

	// Protected publications are accepted, with or without a license.
	assert.NoError(t, inject("testdata/compat-old-lcp.epub"))
	assert.NoError(t, inject(licensedEPUB(t, "license-rsa.lcpl")))
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &buf, Options{}))
	assert.NoError(t, inject(writeTemp(t, "encrypted.webpub", buf.Bytes())))
}

// revokedEntries returns revocation list entries of the given serial numbers.
func revokedEntries(serials ...*big.Int) []x509.RevocationListEntry {
	var entries []x509.RevocationListEntry
	for _, serial := range serials {
		entries = append(entries, x509.RevocationListEntry{SerialNumber: serial, RevocationTime: time.Now()})
	}
	return entries
}

// revocationTestCA returns a certificate authority, with a function issuing
// revocation lists of the given entries, with extra extensions.
func revocationTestCA(t *testing.T) (*x509.Certificate, func(extensions []pkix.Extension, entries []x509.RevocationListEntry) *x509.RevocationList) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"},
		NotBefore: mustTime("2020-01-01T00:00:00Z"), NotAfter: mustTime("2060-01-01T00:00:00Z"),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(nil, tmpl, tmpl, pub, key)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return ca, func(extensions []pkix.Extension, entries []x509.RevocationListEntry) *x509.RevocationList {
		der, err := x509.CreateRevocationList(nil, &x509.RevocationList{
			Number: big.NewInt(2), ThisUpdate: time.Now(), NextUpdate: time.Now().AddDate(0, 0, 7),
			RevokedCertificateEntries: entries, ExtraExtensions: extensions,
		}, ca, key)
		require.NoError(t, err)
		list, err := x509.ParseRevocationList(der)
		require.NoError(t, err)
		return list
	}
}

func TestRevokedByIgnoresUnsupportedLists(t *testing.T) {
	ca, crl := revocationTestCA(t)
	provider := &x509.Certificate{SerialNumber: big.NewInt(7), RawIssuer: ca.RawSubject}
	baseNumber, err := asn1.Marshal(big.NewInt(1))
	require.NoError(t, err)
	onlyUserCerts, err := asn1.Marshal(struct {
		OnlyContainsUserCerts bool `asn1:"optional,tag:1"`
	}{true})
	require.NoError(t, err)
	unknown := asn1.ObjectIdentifier{1, 2, 3, 4}
	removed := revokedEntries(provider.SerialNumber)
	removed[0].ReasonCode = reasonRemoveFromCRL
	criticalEntry := revokedEntries(provider.SerialNumber)
	criticalEntry[0].ExtraExtensions = []pkix.Extension{{Id: unknown, Critical: true, Value: []byte{5, 0}}}

	// Delta and scoped lists are recognized even when they don't mark their
	// extension critical, as RFC 5280 requires.
	unsupported := map[string]func(...*big.Int) *x509.RevocationList{
		"delta": func(serials ...*big.Int) *x509.RevocationList {
			return crl([]pkix.Extension{{Id: oidDeltaCRLIndicator, Value: baseNumber}}, revokedEntries(serials...))
		},
		"scoped": func(serials ...*big.Int) *x509.RevocationList {
			return crl([]pkix.Extension{{Id: oidIssuingDistributionPoint, Value: onlyUserCerts}}, revokedEntries(serials...))
		},
		"critical extension": func(serials ...*big.Int) *x509.RevocationList {
			return crl([]pkix.Extension{{Id: unknown, Critical: true, Value: []byte{5, 0}}}, revokedEntries(serials...))
		},
	}
	for name, list := range unsupported {
		// Omitting the provider, they used to pass for telling it isn't
		// revoked.
		_, err := RevokedBy(list())(provider, ca)
		assert.ErrorIs(t, err, ErrNoRevocationList, name)
		// They don't revoke it either.
		_, err = RevokedBy(list(provider.SerialNumber))(provider, ca)
		assert.ErrorIs(t, err, ErrNoRevocationList, name)
		// Along with a complete list, the latter decides.
		isRevoked, err := RevokedBy(crl(nil, nil), list(provider.SerialNumber))(provider, ca)
		require.NoError(t, err, name)
		assert.False(t, isRevoked, name)
	}
	// Entries unrevoking a certificate, used to revoke it, or with critical
	// extensions, don't make a list apply either.
	for name, entries := range map[string][]x509.RevocationListEntry{"removeFromCRL": removed, "critical entry extension": criticalEntry} {
		_, err := RevokedBy(crl(nil, entries))(provider, ca)
		assert.ErrorIs(t, err, ErrNoRevocationList, name)
	}

	// EDRLab's list applies.
	der, err := os.ReadFile("testdata/edrlab-ca.crl")
	require.NoError(t, err)
	_, err = newRevocationList(der)
	assert.NoError(t, err)
}

func TestRevokedBySnapshotsLists(t *testing.T) {
	ca, crl := revocationTestCA(t)
	_, otherCRL := revocationTestCA(t)
	provider := &x509.Certificate{SerialNumber: big.NewInt(7), RawIssuer: ca.RawSubject}

	// Replacing or changing the lists afterwards, e.g. when refreshing them,
	// used to check the signature of one list against the serial numbers
	// indexed from another: here, the provider passed for not revoked.
	replaced := []*x509.RevocationList{crl(nil, revokedEntries(provider.SerialNumber))}
	isRevoked := RevokedBy(replaced...)
	replaced[0] = otherCRL(nil, nil)
	revoked, err := isRevoked(provider, ca)
	require.NoError(t, err)
	assert.True(t, revoked, "replaced list")

	changed := crl(nil, revokedEntries(provider.SerialNumber))
	isRevoked = RevokedBy(changed)
	*changed = *otherCRL(nil, nil)
	revoked, err = isRevoked(provider, ca)
	require.NoError(t, err)
	assert.True(t, revoked, "changed list")

	// Lists which weren't parsed can't be authenticated.
	_, err = RevokedBy(&x509.RevocationList{Issuer: ca.Subject})(provider, ca)
	assert.ErrorIs(t, err, ErrNoRevocationList)
}

func TestPatchManifestIsCompact(t *testing.T) {
	keep := func(_ string, props map[string]any) map[string]any { return props }

	// Nested extensions were indented, by the depth of each line: a few
	// kilobytes of nested arrays took tens of megabytes.
	nested := strings.Repeat("[", 3000) + strings.Repeat("]", 3000)
	m := strings.Replace(testRWPManifest, `"x-custom": {`, `"x-nested": `+nested+`, "x-custom": {`, 1)
	doc, err := decodeDocument([]byte(m))
	require.NoError(t, err)
	out, err := patchManifest(doc, keep)
	require.NoError(t, err)
	assert.Less(t, len(out), len(m))
	assert.JSONEq(t, m, string(out))

	// Manifests are written without insignificant whitespace.
	var encrypted bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &encrypted, Options{}))
	entries, _ := readZip(t, writeTemp(t, "encrypted.webpub", encrypted.Bytes()))
	for _, out := range [][]byte{out, entries["manifest.json"].content} {
		var compact bytes.Buffer
		require.NoError(t, json.Compact(&compact, out))
		assert.Equal(t, compact.String(), string(out))
	}
}

// remoteFetcher serves remote links from elsewhere than the package.
type remoteFetcher struct{ fetcher.Fetcher }

func (f remoteFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	if outsidePackage(link) {
		return fetcher.NewBytesResource(link, func() []byte { return []byte("remote resource in the clear") })
	}
	return f.Fetcher.Get(ctx, link)
}

func TestDecryptPublicationRemoteLinks(t *testing.T) {
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	key := testKey("remote")
	encrypted, err := EncryptPublication(t.Context(), pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}}, archiveFetcher(t, buildRWP(t)), nil), key, Options{})
	require.NoError(t, err)

	// A remote resource sharing the path of an encrypted one was decrypted
	// as if it were the latter.
	remote := manifest.Link{Href: manifest.MustNewHREFFromString("https://cdn.example.com/chapter.html", false), MediaType: &mediatype.HTML}
	m := encrypted.Manifest
	m.Resources = append(m.Resources, remote)
	decrypted, err := DecryptPublication(pub.New(m, remoteFetcher{encrypted.Fetcher}, nil), key)
	require.NoError(t, err)
	data, rerr := decrypted.Get(t.Context(), remote).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, "remote resource in the clear", string(data))
	data, rerr = decrypted.Get(t.Context(), chapter).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, testRWPFiles["chapter.html"], data)
}

// Seventh review

func TestEncryptRWPManifestSizeLimit(t *testing.T) {
	// A manifest just below the size limit grows beyond it once encryption
	// properties are added, which readers, including DecryptRWP, refused.
	padding := maxManifestSize - len(testRWPManifest) - len(` "padding": "",`)
	m := strings.Replace(testRWPManifest, `"title": "Test",`, `"title": "Test", "padding": "`+strings.Repeat("x", padding)+`",`, 1)
	require.Equal(t, maxManifestSize, len(m))
	err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "manifest.json would be larger than 32 MiB, the size limit of readers")
}

func TestEncryptPublicationLimitsMediaOverlays(t *testing.T) {
	// The XML decoder held whole attributes in memory, however much they
	// inflated, before the first text element.
	smil := `<smil xmlns="http://www.w3.org/ns/SMIL"><body a="` + strings.Repeat("a", epubparser.MaxDocumentSize) + `"><par><text src="chapter.html"/></par></body></smil>`
	files := map[string][]byte{"chapter.html": textBytes(100), "overlay.smil": []byte(smil)}
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	overlay := manifest.Link{Href: manifest.MustNewHREFFromString("overlay.smil", false), MediaType: &mediatype.SMIL}
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Resources: manifest.LinkList{overlay}}, archiveFetcher(t, zipFiles(t, "overlay.epub", files)), nil)
	_, err := EncryptPublication(t.Context(), p, testKey("x"), Options{})
	assert.ErrorIs(t, err, fetcher.ErrResourceTooLarge)

	// Below the limit, media overlays are paired with their content.
	files["overlay.smil"] = []byte(`<smil xmlns="http://www.w3.org/ns/SMIL"><body><par><text src="chapter.html"/></par></body></smil>`)
	p = pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Resources: manifest.LinkList{overlay}}, archiveFetcher(t, zipFiles(t, "overlay.epub", files)), nil)
	_, err = EncryptPublication(t.Context(), p, testKey("x"), Options{})
	assert.NoError(t, err)
}

func TestInjectLicenseReplacesEveryLicense(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &buf, Options{}))
	encrypted, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	files := make(map[string][]byte, len(encrypted))
	for name, e := range encrypted {
		files[name] = e.content
	}

	// A previous recipient's license at the path of the other format was
	// kept, and read instead of the new one.
	files[protection.PathEPUBLCPLicense] = loadLicense(t, "license-ecdsa.lcpl").Raw()
	stale := zipFiles(t, "stale.webpub", files)
	_, err := ReadLicense(t.Context(), fileAsset(t, stale), Options{})
	assert.ErrorIs(t, err, ErrNoLicense, "only the path of the format is read")
	buf.Reset()
	license := loadLicense(t, "license-rsa.lcpl")
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, stale), license, &buf, Options{}))
	delivered := writeTemp(t, "delivered.webpub", buf.Bytes())
	output, _ := readZip(t, delivered)
	assert.NotContains(t, output, protection.PathEPUBLCPLicense)
	read, err := ReadLicense(t.Context(), fileAsset(t, delivered), Options{})
	require.NoError(t, err)
	assert.Equal(t, license.Raw(), read.Raw())
}

func TestInjectLicenseChecksEveryEncryptedResource(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &buf, Options{}))
	encrypted, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	inject := func(change func(files map[string][]byte)) error {
		files := make(map[string][]byte, len(encrypted))
		for name, e := range encrypted {
			files[name] = e.content
		}
		change(files)
		return InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "package.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	}
	require.NoError(t, inject(func(map[string][]byte) {}))

	// One encrypted resource was enough to accept the others, missing or
	// decrypted in place: the PNG, stored as it is and 2000 bytes long, has
	// the size of an AES-CBC ciphertext, but not of its own.
	err := inject(func(files map[string][]byte) { delete(files, "images/figure.png") })
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "images/figure.png is declared LCP-encrypted, but is not in the package")
	err = inject(func(files map[string][]byte) { files["images/figure.png"] = testRWPFiles["images/figure.png"] })
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "images/figure.png is declared LCP-encrypted, but its 2000 bytes aren't the 2032 of the ciphertext of its 2000 bytes")
}

// Eighth review

func TestVerifierRequiresSigningKeyUsage(t *testing.T) {
	// A certificate whose key usage only allows e.g. key encipherment signed
	// licenses which verified: Go's chain verification ignores it.
	sign := func(usage x509.KeyUsage) (*License, *x509.CertPool) {
		p := newEd25519Provider(t)
		pub, key, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		der, err := x509.CreateCertificate(nil, &x509.Certificate{
			SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "Restricted provider"},
			NotBefore: mustTime("2021-01-01T00:00:00Z"), NotAfter: mustTime("2050-01-01T00:00:00Z"),
			KeyUsage: usage,
		}, p.root, pub, p.rootKey)
		require.NoError(t, err)
		p.key, p.der = key, der
		var doc map[string]any
		dec := json.NewDecoder(bytes.NewReader(loadLicense(t, "license-rsa.lcpl").Raw()))
		dec.UseNumber()
		require.NoError(t, dec.Decode(&doc))
		delete(doc, "signature")
		canonical, err := canonicalize(doc)
		require.NoError(t, err)
		pool := x509.NewCertPool()
		pool.AddCert(p.root)
		return p.sign(t, doc, canonical), pool
	}
	l, roots := sign(x509.KeyUsageKeyEncipherment)
	err := l.VerifySignature(roots)
	assert.ErrorIs(t, err, ErrInvalidCertificate)
	assert.ErrorContains(t, err, "doesn't allow signatures")
	assert.NoError(t, l.VerifyIntegrity(), "the certificate isn't checked")
	for name, usage := range map[string]x509.KeyUsage{
		"digital signature":  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		"content commitment": x509.KeyUsageContentCommitment,
		"no key usage":       0,
	} {
		l, roots := sign(usage)
		assert.NoError(t, l.VerifySignature(roots), name)
	}
}

func TestEncryptPublicationPinsFilesInTheClear(t *testing.T) {
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	cover := manifest.Link{Href: manifest.MustNewHREFFromString("cover.jpg", false), MediaType: &mediatype.JPEG, Rels: manifest.Strings{"cover"}}
	for name, newFetcher := range map[string]func(dir string) fetcher.Fetcher{
		"file":     func(dir string) fetcher.Fetcher { return fetcher.NewFileFetcher("", dir) },
		"exploded": func(dir string) fetcher.Fetcher { return archiveFetcher(t, dir) },
	} {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), randomBytes(100), 0o644))
		p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Resources: manifest.LinkList{cover}}, newFetcher(dir), nil)
		encrypted, err := EncryptPublication(t.Context(), p, testKey("x"), Options{})
		require.NoError(t, err)
		res := encrypted.Get(t.Context(), cover)
		_, rerr := res.Read(t.Context(), 0, 0)
		require.Nil(t, rerr, name)
		// Its path may name another file: consumers opening it would bypass
		// the pin.
		assert.Empty(t, res.File(), name)

		// A selected file renamed to the path of a resource in the clear,
		// once the publication was checked, was served in the clear.
		require.NoError(t, os.Rename(filepath.Join(dir, "chapter.html"), filepath.Join(dir, "cover.jpg")))
		data, rerr := encrypted.Get(t.Context(), cover).Read(t.Context(), 0, 0)
		require.NotNil(t, rerr, "%s: served %q", name, data)
		assert.ErrorIs(t, rerr, fetcher.ErrFileChanged, name)

		// A resource in the clear missing when the publication was checked
		// can't be pinned, so it isn't served once a file takes its path.
		require.NoError(t, os.Rename(filepath.Join(dir, "cover.jpg"), filepath.Join(dir, "chapter.html")))
		p = pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Resources: manifest.LinkList{cover}}, newFetcher(dir), nil)
		encrypted, err = EncryptPublication(t.Context(), p, testKey("x"), Options{})
		require.NoError(t, err)
		require.NoError(t, os.Rename(filepath.Join(dir, "chapter.html"), filepath.Join(dir, "cover.jpg")))
		data, rerr = encrypted.Get(t.Context(), cover).Read(t.Context(), 0, 0)
		require.NotNil(t, rerr, "%s: served %q", name, data)
		assert.ErrorContains(t, rerr, "couldn't be identified", name)
	}
}

func TestRemuxPinsEntriesCopiedInTheClear(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cover.jpg"), randomBytes(100), 0o644))
	f := archiveFetcher(t, dir)
	entries, err := containerEntries(t.Context(), f)
	require.NoError(t, err)
	pins, err := checkSameFiles(t.Context(), f, entries, func(e containerEntry) (bool, bool) {
		return e.path == "chapter.html", true
	})
	require.NoError(t, err)

	// The entries copied in the clear were opened again once checked,
	// whatever file their path named by then.
	require.NoError(t, os.Rename(filepath.Join(dir, "chapter.html"), filepath.Join(dir, "cover.jpg")))
	var buf bytes.Buffer
	r := remuxer{ctx: t.Context(), f: f, zw: zip.NewWriter(&buf), pins: pins}
	for _, e := range entries {
		if e.path == "cover.jpg" {
			assert.ErrorIs(t, r.copy(e, e.link), fetcher.ErrFileChanged)
		}
	}
}

func TestRemuxRemovesEveryLicense(t *testing.T) {
	key := testKey("licenses")
	stale := loadLicense(t, "license-ecdsa.lcpl").Raw()
	withLicenses := func(path string) string {
		files := map[string][]byte{}
		entries, _ := readZip(t, path)
		for name, e := range entries {
			files[name] = e.content
		}
		files[protection.PathEPUBLCPLicense] = stale
		files[protection.PathRWPLCPLicense] = stale
		return zipFiles(t, "licenses"+filepath.Ext(path), files)
	}
	names := func(data []byte) []string {
		_, names := readZip(t, writeTemp(t, "output.zip", data))
		return names
	}

	// Decrypting kept the license at the path of the other format, which
	// holds a previous recipient's identifier and passphrase hint.
	var buf bytes.Buffer
	require.NoError(t, EncryptEPUB(t.Context(), fileAsset(t, "testdata/compat.epub"), key, &buf, Options{}))
	epub := withLicenses(writeTemp(t, "encrypted.epub", buf.Bytes()))
	buf.Reset()
	require.NoError(t, DecryptEPUB(t.Context(), fileAsset(t, epub), key, &buf, Options{}))
	assert.NotContains(t, names(buf.Bytes()), protection.PathEPUBLCPLicense)
	assert.NotContains(t, names(buf.Bytes()), protection.PathRWPLCPLicense)

	buf.Reset()
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), key, &buf, Options{}))
	rwp := withLicenses(writeTemp(t, "encrypted.webpub", buf.Bytes()))
	buf.Reset()
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, rwp), key, &buf, Options{}))
	assert.NotContains(t, names(buf.Bytes()), protection.PathEPUBLCPLicense)
	assert.NotContains(t, names(buf.Bytes()), protection.PathRWPLCPLicense)

	// So did encrypting, at the path its own format doesn't refuse.
	files := map[string][]byte{"manifest.json": []byte(testRWPManifest), protection.PathEPUBLCPLicense: stale}
	maps.Copy(files, testRWPFiles)
	buf.Reset()
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "stale.webpub", files)), key, &buf, Options{}))
	assert.NotContains(t, names(buf.Bytes()), protection.PathEPUBLCPLicense)
}

func TestDecryptorRangesRespectOriginalLength(t *testing.T) {
	key := testKey("original-length")
	plain := textBytes(5000)
	res, enc := encryptedResource(t, key, plain, false)

	// Ranges were served beyond the declared original length, which whole
	// reads refuse.
	enc.OriginalLength = int64(len(plain) - 10)
	shortened := Decryptor{key: key}.decrypt(res, enc)
	_, rerr := shortened.Read(t.Context(), 4990, 4999)
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr, ErrInvalidCiphertext)
	_, rerr = shortened.Stream(t.Context(), io.Discard, 0, 10)
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr, ErrInvalidCiphertext)
	_, rerr = shortened.Read(t.Context(), 0, 0)
	assert.ErrorIs(t, rerr, ErrInvalidCiphertext, "as whole reads")

	enc.OriginalLength = int64(len(plain))
	data, rerr := Decryptor{key: key}.decrypt(res, enc).Read(t.Context(), 4990, 4999)
	require.Nil(t, rerr)
	assert.Equal(t, plain[4990:], data)
}

func TestRWPDeclarationsInLinks(t *testing.T) {
	key := testKey("links")
	m := strings.Replace(testRWPManifest, `"links": [`, `"links": [{"rel": "alternate", "href": "chapter.html", "type": "text/html"},`, 1)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, m)), key, &buf, Options{}))
	encrypted := writeTemp(t, "encrypted.webpub", buf.Bytes())
	linkProperties := func(path string) map[string]any {
		entries, _ := readZip(t, path)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(entries["manifest.json"].content, &doc))
		for _, l := range doc["links"].([]any) {
			if link := l.(map[string]any); link["href"] == "chapter.html" {
				props, _ := link["properties"].(map[string]any)
				return props
			}
		}
		return nil
	}
	// The links declare the encryption of the resources they point to, as
	// the reading order and resources do, and lose it once decrypted: the
	// decrypted manifest used to declare decrypted resources encrypted.
	assert.NotNil(t, linkProperties(encrypted)["encrypted"])
	buf.Reset()
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, encrypted), key, &buf, Options{}))
	assert.Nil(t, linkProperties(writeTemp(t, "decrypted.webpub", buf.Bytes()))["encrypted"])

	// Injecting a license checks them as well: a resource declared
	// encrypted in the links used to be accepted missing.
	entries, _ := readZip(t, encrypted)
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	var doc map[string]any
	require.NoError(t, json.Unmarshal(files["manifest.json"], &doc))
	doc["links"] = append(doc["links"].([]any), map[string]any{
		"href": "missing.bin", "type": "application/octet-stream",
		"properties": map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": AlgorithmAES256CBC}},
	})
	patched, err := json.Marshal(doc)
	require.NoError(t, err)
	files["manifest.json"] = patched
	err = InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "missing.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "missing.bin is declared LCP-encrypted, but is not in the package")
}

func TestInjectLicenseKeepsRWPMimetypeResource(t *testing.T) {
	// A resource named mimetype was dropped from Readium Web Publication
	// packages, as if it were the mimetype file of an EPUB.
	m := strings.Replace(testRWPManifest, `"resources": [`, `"resources": [{"href": "mimetype", "type": "text/plain"},`, 1)
	files := map[string][]byte{"manifest.json": []byte(m), "mimetype": textBytes(100)}
	maps.Copy(files, testRWPFiles)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "package.webpub", files)), testKey("x"), &buf, Options{}))
	encryptedPath := writeTemp(t, "encrypted.webpub", buf.Bytes())
	encrypted, _ := readZip(t, encryptedPath)
	require.Contains(t, encrypted, "mimetype")

	var delivered bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, encryptedPath), loadLicense(t, "license-rsa.lcpl"), &delivered, Options{}))
	output, _ := readZip(t, writeTemp(t, "delivered.webpub", delivered.Bytes()))
	require.Contains(t, output, "mimetype")
	assert.Equal(t, encrypted["mimetype"].content, output["mimetype"].content)
}

// Ninth review

// fetcherAsset is a Readium Web Publication package read with the given
// fetcher.
type fetcherAsset struct{ f fetcher.Fetcher }

func (a fetcherAsset) Name() string { return "package.webpub" }
func (a fetcherAsset) MediaType(context.Context) mediatype.MediaType {
	return mediatype.ReadiumWebpub
}
func (a fetcherAsset) CreateFetcher(context.Context, asset.Dependencies, string) (fetcher.Fetcher, error) {
	return a.f, nil
}

// hookFetcher calls hook before each Get, with the number of Gets of the
// path so far, this one included.
type hookFetcher struct {
	fetcher.Fetcher
	gets map[string]int
	hook func(path string, gets int)
}

// Exhaustive lists what the wrapped fetcher lists.
func (f hookFetcher) Exhaustive() bool {
	e, ok := f.Fetcher.(fetcher.Exhaustive)
	return ok && e.Exhaustive()
}

func (f *hookFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	path := link.Href.String()
	f.gets[path]++
	f.hook(path, f.gets[path])
	return f.Fetcher.Get(ctx, link)
}

func TestInjectLicenseCopiesWhatItChecked(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &buf, Options{}))
	encrypted, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	explode := func() string {
		dir := t.TempDir()
		for name, e := range encrypted {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), e.content, 0o644))
		}
		return dir
	}
	// The files are got once to identify them (licenses must have no other
	// name), once to check them, then once to copy them: the hooks change
	// them in between the last two.
	inject := func(dir string, hook func(dir, path string, gets int)) ([]byte, error) {
		f := &hookFetcher{Fetcher: archiveFetcher(t, dir), gets: make(map[string]int), hook: func(path string, gets int) { hook(dir, path, gets) }}
		var out bytes.Buffer
		err := InjectLicense(t.Context(), fetcherAsset{f}, loadLicense(t, "license-rsa.lcpl"), &out, Options{})
		return out.Bytes(), err
	}
	plain := testRWPFiles["images/figure.png"]

	// A resource changed between its check and its copy used to be copied
	// as it was then: here, in the clear.
	_, err := inject(explode(), func(dir, path string, gets int) {
		if path == "images/figure.png" && gets == 3 {
			replacement := filepath.Join(dir, "replacement")
			require.NoError(t, os.WriteFile(replacement, plain, 0o644))
			require.NoError(t, os.Rename(replacement, filepath.Join(dir, path)))
		}
	})
	assert.ErrorIs(t, err, fetcher.ErrFileChanged, "replaced")
	out, err := inject(explode(), func(dir, path string, gets int) {
		if path == "images/figure.png" && gets == 3 {
			require.NoError(t, os.WriteFile(filepath.Join(dir, path), plain, 0o644))
		}
	})
	assert.ErrorContains(t, err, "images/figure.png changed since it was checked", "rewritten")
	// It was written before being checked: a client receiving the package
	// as it is written got the plaintext before the error.
	assert.False(t, bytes.Contains(out, plain), "the plaintext was written")

	// So was the manifest declaring the encrypted resources.
	dir := explode()
	source := archiveFetcher(t, dir)
	changed := false
	afterRead := fetcher.NewTransformingFetcher(source, func(r fetcher.Resource) fetcher.Resource {
		if r.Link().Href.String() != "manifest.json" {
			return r
		}
		return fetcher.NewTransformingResource(r, false, func(captured []byte) []byte {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(testRWPManifest), 0o644))
			changed = true
			return captured
		})
	})
	type inventory interface{ fetcher.Exhaustive }
	f := struct {
		fetcher.Fetcher
		inventory
	}{afterRead, source.(fetcher.Exhaustive)}
	var delivered bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fetcherAsset{f}, loadLicense(t, "license-rsa.lcpl"), &delivered, Options{}))
	require.True(t, changed, "source manifest was replaced")
	output, _ := readZip(t, writeTemp(t, "delivered.webpub", delivered.Bytes()))
	assert.Equal(t, encrypted["manifest.json"].content, output["manifest.json"].content)
}

func TestInjectLicenseReadsTheRawManifest(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &buf, Options{}))
	entries, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	// The parsed manifest drops links without a media type, whatever they
	// declare: a resource declared encrypted, but in the clear, was accepted.
	var doc map[string]any
	require.NoError(t, json.Unmarshal(files["manifest.json"], &doc))
	doc["resources"] = append(doc["resources"].([]any), map[string]any{
		"href":       "extra/unlisted.json",
		"properties": map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": AlgorithmAES256CBC}},
	})
	patched, err := json.Marshal(doc)
	require.NoError(t, err)
	files["manifest.json"] = patched
	err = InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "untyped.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "extra/unlisted.json is declared LCP-encrypted, but its 100 bytes can't be an AES-CBC ciphertext")
}

func TestLicenseNestingLimit(t *testing.T) {
	// The check of member names took a call per level, however deep.
	nested := func(depth int) []byte {
		return []byte(`{"x": ` + strings.Repeat("[", depth-1) + strings.Repeat("]", depth-1) + `}`)
	}
	assert.NoError(t, checkMemberNames(nested(maxJSONDepth)))
	assert.ErrorContains(t, checkMemberNames(nested(maxJSONDepth+1)), "nested more than 1000 levels deep")
	_, err := ParseLicense(nested(maxJSONDepth + 1))
	assert.ErrorIs(t, err, ErrInvalidLicense)
}

// fakeFileInfo describes a file of the given size, modified at the given
// time, with the given system information.
type fakeFileInfo struct {
	size  int64
	mtime time.Time
	sys   any
}

func (f fakeFileInfo) Name() string       { return "file" }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return f.mtime }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return f.sys }

func TestSameFilesBoundsComparisons(t *testing.T) {
	// Files which can't be identified are compared in pairs, which took
	// quadratic time for files of the same size and modification time, as
	// in publications extracted with normalized times.
	ids := make([]fs.FileInfo, 3000)
	for i := range ids {
		ids[i] = fakeFileInfo{size: 100, mtime: mustTime("2020-01-01T00:00:00Z")}
	}
	limit := maxFileComparisons
	t.Cleanup(func() { maxFileComparisons = limit })
	maxFileComparisons = 1000
	_, err := sameFiles(t.Context(), ids)
	assert.ErrorContains(t, err, "too many files of the same size and modification time")

	// The comparisons stop once the context is canceled.
	maxFileComparisons = limit
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = sameFiles(ctx, ids)
	assert.ErrorIs(t, err, context.Canceled)
}

// Tenth review

func TestNullRWPManifest(t *testing.T) {
	// ManifestFromJSON accepts null, which was dereferenced.
	files := map[string][]byte{"manifest.json": []byte("null")}
	maps.Copy(files, testRWPFiles)
	path := zipFiles(t, "null.webpub", files)
	protected, err := isLCPProtectedPackage(t, archiveFetcher(t, path))
	require.NoError(t, err)
	assert.False(t, protected)
	err = EncryptRWP(t.Context(), fileAsset(t, path), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "not a JSON object")
	err = InjectLicense(t.Context(), fileAsset(t, path), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorContains(t, err, "not a JSON object")
}

// encryptedRWPFiles returns the entries of testRWPManifest encrypted, with
// the manifest changed by change.
func encryptedRWPFiles(t *testing.T, change func(doc map[string]any)) map[string][]byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("x"), &buf, Options{}))
	entries, _ := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	var doc map[string]any
	require.NoError(t, json.Unmarshal(files["manifest.json"], &doc))
	change(doc)
	patched, err := json.Marshal(doc)
	require.NoError(t, err)
	files["manifest.json"] = patched
	return files
}

func TestInjectLicenseValidatesTheManifest(t *testing.T) {
	// A manifest readers refuse was licensed, as long as it declared
	// encrypted resources.
	files := encryptedRWPFiles(t, func(doc map[string]any) { delete(doc, "metadata") })
	err := InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "invalid.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorContains(t, err, "invalid manifest.json")
}

func TestInjectLicenseMergesDeclarations(t *testing.T) {
	inject := func(link map[string]any, plaintext bool) error {
		files := encryptedRWPFiles(t, func(doc map[string]any) {
			doc["links"] = append(doc["links"].([]any), link)
		})
		if plaintext {
			files["images/figure.png"] = testRWPFiles["images/figure.png"]
		}
		return InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "package.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	}
	declaration := func(properties map[string]any) map[string]any {
		properties["scheme"], properties["algorithm"] = Scheme, AlgorithmAES256CBC
		return map[string]any{"href": "images/figure.png", "type": "image/png", "properties": map[string]any{"encrypted": properties}}
	}

	// A later declaration without original length used to replace the
	// earlier one, checking the size of the PNG, decrypted in place, as
	// block-aligned rather than exactly: its 2000 bytes passed.
	err := inject(declaration(map[string]any{}), true)
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "aren't the 2032 of the ciphertext of its 2000 bytes")
	assert.NoError(t, inject(declaration(map[string]any{}), false))

	// Declarations which can't hold together are an error.
	err = inject(declaration(map[string]any{"originalLength": 1999}), false)
	assert.ErrorContains(t, err, "images/figure.png is declared encrypted with different original lengths, 2000 and 1999")
	err = inject(declaration(map[string]any{"compression": "deflate"}), false)
	assert.ErrorContains(t, err, "images/figure.png is declared encrypted with different compressions")
}

// Eleventh review

// changingFetcher serves a resource at path from the fetcher wrapped the
// first time it is requested, through a proxy, then from later.
type changingFetcher struct {
	fetcher.Fetcher
	path  string
	later func(link manifest.Link) fetcher.Resource
	gets  int
}

// Exhaustive lists what the wrapped fetcher lists.
func (f changingFetcher) Exhaustive() bool {
	e, ok := f.Fetcher.(fetcher.Exhaustive)
	return ok && e.Exhaustive()
}

func (f *changingFetcher) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	if link.Href.String() != f.path {
		return f.Fetcher.Get(ctx, link)
	}
	// The first two gets identify the file (licenses must have no other
	// name), and check it: the later ones copy it.
	if f.gets++; f.gets <= 2 {
		return fetcher.ProxyResource{Res: f.Fetcher.Get(ctx, link)}
	}
	return f.later(link)
}

// growingResource streams up to 64 MiB, counting the bytes its writer
// accepts.
type growingResource struct {
	fetcher.Resource
	accepted int64
}

func (r *growingResource) Stream(ctx context.Context, w io.Writer, start, end int64) (int64, *fetcher.ResourceError) {
	chunk := make([]byte, 1<<20)
	for range 64 {
		n, err := w.Write(chunk)
		r.accepted += int64(n)
		if err != nil {
			return r.accepted, fetcher.Other(err)
		}
	}
	return r.accepted, nil
}

func TestInjectLicenseStagesUnstableSources(t *testing.T) {
	files := encryptedRWPFiles(t, func(map[string]any) {})
	plain := testRWPFiles["images/figure.png"]
	inject := func(later func(link manifest.Link) fetcher.Resource) ([]byte, error) {
		f := &changingFetcher{Fetcher: archiveFetcher(t, zipFiles(t, "encrypted.webpub", files)), path: "images/figure.png", later: later}
		var out bytes.Buffer
		err := InjectLicense(t.Context(), fetcherAsset{f}, loadLicense(t, "license-rsa.lcpl"), &out, Options{})
		return out.Bytes(), err
	}

	// Any proxy passed for an archive entry, read from a single version of
	// its archive: a remote resource changed since it was checked was copied
	// without being checked again.
	out, err := inject(func(link manifest.Link) fetcher.Resource {
		return fetcher.ProxyResource{Res: fetcher.NewBytesResource(link, func() []byte { return plain })}
	})
	assert.ErrorContains(t, err, "images/figure.png changed since it was checked: 2000 bytes instead of 2032")
	assert.False(t, bytes.Contains(out, plain), "the plaintext was written")

	// A resource growing since it was checked filled the temporary file
	// before its length was compared.
	growing := &growingResource{}
	_, err = inject(func(link manifest.Link) fetcher.Resource {
		growing.Resource = fetcher.NewBytesResource(link, func() []byte { return nil })
		return growing
	})
	assert.ErrorContains(t, err, "more than 2032 bytes")
	assert.LessOrEqual(t, growing.accepted, int64(2032))
}

func TestInjectLicenseChecksInflatedLengths(t *testing.T) {
	files := encryptedRWPFiles(t, func(map[string]any) {})
	ciphertext := files["images/figure.png"]
	plain := testRWPFiles["images/figure.png"]
	// inject injects a license into the package, its PNG replaced by a
	// deflated entry of the given content, checksum and declared length.
	inject := func(content []byte, crc uint32, declared uint64) error {
		var compressed bytes.Buffer
		fw, err := flate.NewWriter(&compressed, flate.BestCompression)
		require.NoError(t, err)
		_, err = fw.Write(content)
		require.NoError(t, err)
		require.NoError(t, fw.Close())
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, name := range slices.Sorted(maps.Keys(files)) {
			if name == "images/figure.png" {
				w, err := zw.CreateRaw(&zip.FileHeader{
					Name: name, Method: zip.Deflate, CRC32: crc,
					CompressedSize64: uint64(compressed.Len()), UncompressedSize64: declared,
				})
				require.NoError(t, err)
				_, err = w.Write(compressed.Bytes())
				require.NoError(t, err)
				continue
			}
			w, err := zw.Create(name)
			require.NoError(t, err)
			_, err = w.Write(files[name])
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())
		return InjectLicense(t.Context(), fileAsset(t, writeTemp(t, "deflated.webpub", buf.Bytes())), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	}
	declared := uint64(len(ciphertext))
	require.NoError(t, inject(ciphertext, crc32.ChecksumIEEE(ciphertext), declared))

	// A deflated entry declaring the length of its ciphertext, but inflating
	// to plaintext, was copied compressed as it was, unchecked. So were
	// entries with a bad checksum, or inflating longer than declared, whose
	// end wasn't read.
	for name, err := range map[string]error{
		"plaintext":            inject(plain, crc32.ChecksumIEEE(plain), declared),
		"bad checksum":         inject(ciphertext, crc32.ChecksumIEEE(ciphertext)+1, declared),
		"longer than declared": inject(append(bytes.Clone(ciphertext), randomBytes(16)...), crc32.ChecksumIEEE(ciphertext), declared),
	} {
		assert.ErrorContains(t, err, "images/figure.png isn't as it was checked", name)
	}
}

func TestDecryptionMergesDeclarations(t *testing.T) {
	key := testKey("x")
	// The chapter, deflated, declares an original length shorter than its
	// content, and another link declares it again without one, which used
	// to remove the bound.
	files := encryptedRWPFiles(t, func(doc map[string]any) {
		chapter := doc["readingOrder"].([]any)[2].(map[string]any)
		chapter["properties"].(map[string]any)["encrypted"].(map[string]any)["originalLength"] = 100
		doc["links"] = append(doc["links"].([]any), map[string]any{
			"href": "chapter.html", "type": "text/html",
			"properties": map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": AlgorithmAES256CBC, "compression": "deflate"}},
		})
	})
	path := zipFiles(t, "duplicates.webpub", files)
	err := DecryptRWP(t.Context(), fileAsset(t, path), key, io.Discard, Options{})
	assert.ErrorIs(t, err, ErrInvalidCiphertext, "DecryptRWP")

	enc := manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC, Compression: CompressionDeflate}
	bounded := enc
	bounded.OriginalLength = 100
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML, Properties: withEncryption(nil, &bounded)}
	again := chapter
	again.Properties = withEncryption(nil, &enc)
	p := pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Links: manifest.LinkList{again}}, archiveFetcher(t, path), nil)
	decrypted, err := DecryptPublication(p, key)
	require.NoError(t, err)
	_, rerr := decrypted.Get(t.Context(), chapter).Read(t.Context(), 0, 0)
	require.NotNil(t, rerr)
	assert.ErrorIs(t, rerr, ErrInvalidCiphertext, "DecryptPublication")

	// Declarations which can't hold together are an error.
	stored := enc
	stored.Compression = ""
	again.Properties = withEncryption(nil, &stored)
	_, err = DecryptPublication(pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}, Links: manifest.LinkList{again}}, archiveFetcher(t, path), nil), key)
	assert.ErrorContains(t, err, "chapter.html is declared encrypted with different compressions")
}

func TestInjectLicenseParsesHREFsAsTheManifestParser(t *testing.T) {
	lcpEncrypted := map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": AlgorithmAES256CBC}}
	inject := func(link map[string]any) error {
		files := encryptedRWPFiles(t, func(doc map[string]any) {
			doc["resources"] = append(doc["resources"].([]any), link)
		})
		return InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "package.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	}
	// The manifest parser takes an href holding a literal "%" for a decoded
	// path, while the declarations were read only from valid URLs: the
	// declaration of this missing resource was skipped.
	err := inject(map[string]any{"href": "images/100%.png", "type": "image/png", "properties": lcpEncrypted})
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "images/100%.png is declared LCP-encrypted, but is not in the package")
	// Declarations without a valid link are an error, from the validation of
	// the manifest as well.
	err = inject(map[string]any{"type": "image/png", "properties": lcpEncrypted})
	assert.ErrorContains(t, err, "'href' is required in link")
	_, err = rwpEncryptedResources([]byte(`{"resources": [{"type": "image/png", "properties": {"encrypted": {"scheme": "http://readium.org/2014/01/lcp", "algorithm": "http://www.w3.org/2001/04/xmlenc#aes256-cbc"}}}]}`))
	assert.ErrorContains(t, err, "is declared encrypted, but isn't a valid link")
}

func TestInjectLicenseRefusesDeclarationsOfReplacedEntries(t *testing.T) {
	// A declaration of the license, the only one, passed for the protected
	// resource of a package delivered without any.
	m := strings.Replace(testRWPManifest, `"resources": [`, `"resources": [{"href": "license.lcpl", "type": "application/vnd.readium.lcp.license.v1.0+json", "properties": {"encrypted": {"scheme": "http://readium.org/2014/01/lcp", "algorithm": "http://www.w3.org/2001/04/xmlenc#aes256-cbc"}}},`, 1)
	files := map[string][]byte{"manifest.json": []byte(m), protection.PathRWPLCPLicense: randomBytes(32)}
	maps.Copy(files, testRWPFiles)
	err := InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "license.webpub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorIs(t, err, ErrNotProtected)
	assert.ErrorContains(t, err, "license.lcpl is declared LCP-encrypted, but is replaced by the injection")
}

func TestDecryptEPUBMergesDeclarations(t *testing.T) {
	key := testKey("x")
	var buf bytes.Buffer
	require.NoError(t, EncryptEPUB(t.Context(), fileAsset(t, "testdata/compat.epub"), key, &buf, Options{}))
	entries, _ := readZip(t, writeTemp(t, "encrypted.epub", buf.Bytes()))
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	// encryption.xml declaring a resource twice, differently: the last
	// declaration was taken.
	doc := string(files[protection.PathEPUBEncryption])
	var entry string
	for _, block := range strings.SplitAfter(doc, "</enc:EncryptedData>") {
		if i := strings.Index(block, "<enc:EncryptedData>"); i >= 0 && strings.Contains(block, `OriginalLength="`) {
			entry = block[i:]
			break
		}
	}
	require.NotEmpty(t, entry)
	changed := regexp.MustCompile(`OriginalLength="\d+"`).ReplaceAllString(entry, `OriginalLength="1"`)
	files[protection.PathEPUBEncryption] = []byte(strings.Replace(doc, entry, entry+changed, 1))
	err := DecryptEPUB(t.Context(), fileAsset(t, zipFiles(t, "duplicates.epub", files)), key, io.Discard, Options{})
	assert.ErrorContains(t, err, "is declared encrypted with different original lengths")
}

// Twelfth review

// epubWithEncryption returns the path of licensedEPUB with its encryption.xml
// changed by change, given the entry of an uncompressed resource, and the
// href of that resource.
func epubWithEncryption(t *testing.T, change func(doc, entry string) string) (path, href string) {
	t.Helper()
	entries, _ := readZip(t, licensedEPUB(t, "license-rsa.lcpl"))
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	doc := string(files[protection.PathEPUBEncryption])
	var entry string
	for _, block := range strings.SplitAfter(doc, "</EncryptedData>") {
		if i := strings.Index(block, "<EncryptedData"); i >= 0 && strings.Contains(block, `Method="0"`) {
			entry = block[i:]
			break
		}
	}
	require.NotEmpty(t, entry, "an uncompressed resource")
	href = regexp.MustCompile(`CipherReference[^>]*URI="([^"]+)"`).FindStringSubmatch(entry)[1]
	files[protection.PathEPUBEncryption] = []byte(change(doc, entry))
	return zipFiles(t, "duplicates.epub", files), href
}

// openWithLCP opens a publication in the streamer with the LCP content
// protection, unlocking its license with testPassphrase.
func openWithLCP(t *testing.T, path string) (*pub.Publication, error) {
	t.Helper()
	return streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{InsecureSkipVerify: true}},
	}).Open(t.Context(), fileAsset(t, path), testPassphrase)
}

func TestStreamerMergesEncryptionDeclarations(t *testing.T) {
	// The parser took the last declaration of a resource: here, one dropping
	// the original length the first declares, too short for its content.
	path, href := epubWithEncryption(t, func(doc, entry string) string {
		bounded := regexp.MustCompile(`OriginalLength="\d+"`).ReplaceAllString(entry, `OriginalLength="16"`)
		unbounded := regexp.MustCompile(`(?s)<EncryptionProperties.*</EncryptionProperties>`).ReplaceAllString(entry, "")
		return strings.Replace(doc, entry, bounded+unbounded, 1)
	})
	p, err := openWithLCP(t, path)
	require.NoError(t, err)
	defer p.Close()
	_, rerr := p.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
	require.NotNil(t, rerr, "the resource keeps the original length of its first declaration")
	assert.ErrorIs(t, rerr, ErrInvalidCiphertext)

	// Declarations which can't hold together are an error.
	path, _ = epubWithEncryption(t, func(doc, entry string) string {
		other := regexp.MustCompile(`OriginalLength="\d+"`).ReplaceAllString(entry, `OriginalLength="16"`)
		return strings.Replace(doc, entry, entry+other, 1)
	})
	_, err = openWithLCP(t, path)
	assert.ErrorContains(t, err, "declared encrypted with different original lengths")
}

func TestDecryptRWPReadsTheRawManifest(t *testing.T) {
	// The parsed manifest drops the links without a media type: a resource
	// only declared encrypted by such a link was copied encrypted, without
	// its license.
	files := encryptedRWPFiles(t, func(doc map[string]any) {
		for _, r := range doc["resources"].([]any) {
			if link := r.(map[string]any); link["href"] == "images/figure.png" {
				delete(link, "type")
			}
		}
	})
	var buf bytes.Buffer
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, zipFiles(t, "untyped.webpub", files)), testKey("x"), &buf, Options{}))
	output, _ := readZip(t, writeTemp(t, "decrypted.webpub", buf.Bytes()))
	assert.Equal(t, testRWPFiles["images/figure.png"], output["images/figure.png"].content)
	assert.NotContains(t, string(output["manifest.json"].content), AlgorithmAES256CBC)
}

// Thirteenth review

// respellHref percent-encodes the first character of the CipherReference URI
// of an encryption.xml entry: the same resource, spelled differently.
func respellHref(entry string) string {
	m := regexp.MustCompile(`(CipherReference[^>]*URI=")([^"]+)"`).FindStringSubmatchIndex(entry)
	start := m[4]
	return entry[:start] + fmt.Sprintf("%%%02X", entry[start]) + entry[start+1:]
}

func TestStreamerMergesEquivalentHrefs(t *testing.T) {
	// The EPUB parser merged declarations by href spelling, while archives
	// resolve the decoded path: a declaration spelling the href of its
	// resource differently was another resource to the parser, and the
	// original length it declares was lost.
	path, href := epubWithEncryption(t, func(doc, entry string) string {
		bounded := respellHref(regexp.MustCompile(`OriginalLength="\d+"`).ReplaceAllString(entry, `OriginalLength="16"`))
		unbounded := regexp.MustCompile(`(?s)<EncryptionProperties.*</EncryptionProperties>`).ReplaceAllString(entry, "")
		return strings.Replace(doc, entry, bounded+unbounded, 1)
	})
	builder, err := epubparser.NewParser(nil).Parse(t.Context(), fileAsset(t, path), archiveFetcher(t, path))
	require.NoError(t, err)
	link := builder.Manifest.LinkWithHref(manifest.MustNewHREFFromString(href, false).Resolve(nil, nil))
	require.NotNil(t, link)
	enc := link.Properties.Encryption()
	require.NotNil(t, enc)
	assert.EqualValues(t, 16, enc.OriginalLength, "the parsed manifest declares the original length")

	p, err := openWithLCP(t, path)
	require.NoError(t, err)
	defer p.Close()
	_, rerr := p.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
	require.NotNil(t, rerr, "the resource keeps the original length of its other spelling")
	assert.ErrorIs(t, rerr, ErrInvalidCiphertext)

	path, _ = epubWithEncryption(t, func(doc, entry string) string {
		other := respellHref(regexp.MustCompile(`OriginalLength="\d+"`).ReplaceAllString(entry, `OriginalLength="16"`))
		return strings.Replace(doc, entry, entry+other, 1)
	})
	_, err = openWithLCP(t, path)
	assert.ErrorContains(t, err, "declared encrypted with different original lengths")
}

func TestEncryptPublicationDecidesOncePerResource(t *testing.T) {
	// Each link to a resource was asked whether to compress it, and declared
	// its own answer, while the resource was encrypted as the last link
	// said: links answering differently made a publication its decryption
	// refuses. Links not selecting the resource didn't declare it at all.
	dir := t.TempDir()
	chapter := textBytes(3000)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), chapter, 0o644))
	link := func(title string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML, Title: title}
	}
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{link("")},
		Resources:    manifest.LinkList{link("again")},
		Links:        manifest.LinkList{link("related")},
	}
	key := testKey("once")
	encrypted, err := EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), key, Options{
		Encrypt:  func(l manifest.Link) bool { return l.Title != "related" },
		Compress: func(l manifest.Link) bool { return l.Title == "" },
	})
	require.NoError(t, err)
	declared := encrypted.Manifest.ReadingOrder[0].Properties.Encryption()
	require.NotNil(t, declared)
	assert.Equal(t, CompressionDeflate, declared.Compression, "the first link decides")
	for _, l := range []manifest.Link{encrypted.Manifest.Resources[0], encrypted.Manifest.Links[0]} {
		assert.Equal(t, declared, l.Properties.Encryption(), "%q declares the same encryption", l.Title)
	}

	decrypted, err := DecryptPublication(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, chapter, readResource(t, decrypted, link("")))
}

// zipContents returns the content of the entries of a ZIP archive, by name.
func zipContents(t *testing.T, path string) map[string][]byte {
	t.Helper()
	entries, _ := readZip(t, path)
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	return files
}

// rwpWithUntypedDeclarations returns the path of the test package encrypted
// with key, where every declaration moved to an untyped link to the same
// resource, which the manifest parser drops, and the path of its source.
func rwpWithUntypedDeclarations(t *testing.T, key ContentKey) (encrypted, source string) {
	t.Helper()
	source = buildRWP(t)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, source), key, &buf, Options{}))
	files := zipContents(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	var doc map[string]any
	require.NoError(t, json.Unmarshal(files["manifest.json"], &doc))
	var untyped []any
	var move func(links []any)
	move = func(links []any) {
		for _, l := range links {
			link := l.(map[string]any)
			if props, ok := link["properties"].(map[string]any); ok && props["encrypted"] != nil {
				untyped = append(untyped, map[string]any{"href": link["href"], "properties": map[string]any{"encrypted": props["encrypted"]}})
				delete(props, "encrypted")
			}
			if alternates, ok := link["alternate"].([]any); ok {
				move(alternates)
			}
		}
	}
	move(doc["readingOrder"].([]any))
	move(doc["resources"].([]any))
	require.NotEmpty(t, untyped)
	doc["resources"] = append(doc["resources"].([]any), untyped...)
	patched, err := json.Marshal(doc)
	require.NoError(t, err)
	files["manifest.json"] = patched
	return zipFiles(t, "untyped.webpub", files), source
}

func TestStreamerReadsThePackageDeclarations(t *testing.T) {
	// The streamer only knew the declarations of the parsed manifest. It
	// drops the links without a media type, so resources only declared
	// encrypted by such links were served encrypted, and a package only
	// declaring encrypted resources this way wasn't taken as protected
	// when its license was given separately.
	encrypted, source := rwpWithUntypedDeclarations(t, testKey("lcp-compat"))
	expected := openPublication(t, source, streamer.Config{})

	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{License: loadLicense(t, "license-rsa.lcpl"), InsecureSkipVerify: true}},
	}).Open(t.Context(), fileAsset(t, encrypted), testPassphrase)
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, expected, p)

	var licensed bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, encrypted), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}))
	p, err = openWithLCP(t, writeTemp(t, "licensed.webpub", licensed.Bytes()))
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, expected, p)

	// The EPUB parser only declares the resources of the package document:
	// a resource missing from it, but declared by encryption.xml, was
	// served encrypted.
	figure := manifest.Link{Href: manifest.MustNewHREFFromString("OEBPS/images/figure.png", false)}
	reference, err := openWithLCP(t, licensedEPUB(t, "license-rsa.lcpl"))
	require.NoError(t, err)
	defer reference.Close()
	files := zipContents(t, licensedEPUB(t, "license-rsa.lcpl"))
	opf := string(files["OEBPS/content.opf"])
	unlisted := regexp.MustCompile(`\s*<item id="fig"[^>]*/>`).ReplaceAllString(opf, "")
	require.NotEqual(t, opf, unlisted)
	files["OEBPS/content.opf"] = []byte(unlisted)
	p, err = openWithLCP(t, zipFiles(t, "unlisted.epub", files))
	require.NoError(t, err)
	defer p.Close()
	assert.Equal(t, readResource(t, reference, figure), readResource(t, p, figure))
}

func TestInjectLicenseMergesEPUBDeclarationsFirst(t *testing.T) {
	// Only the LCP declarations were merged: a resource also declared
	// obfuscated was licensed, though readers refuse it.
	entries, _ := readZip(t, "testdata/compat-old-lcp.epub")
	files := make(map[string][]byte, len(entries))
	for name, e := range entries {
		files[name] = e.content
	}
	obfuscated := fmt.Sprintf(`<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="%s"/><CipherData><CipherReference URI="OEBPS/style.css"/></CipherData></EncryptedData></encryption>`, protection.AlgorithmIDPFObfuscation)
	files[protection.PathEPUBEncryption] = []byte(strings.Replace(string(files[protection.PathEPUBEncryption]), "</encryption>", obfuscated, 1))
	err := InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "conflicting.epub", files)), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorContains(t, err, "declared encrypted differently")
}

// Fourteenth review

func TestEncryptRWPReadsTheRawDeclarations(t *testing.T) {
	// The existing declarations were read from the parsed manifest, which
	// drops the links without a media type: a resource only declared
	// encrypted by such a link was encrypted again, its declaration
	// overwritten.
	encrypted, _ := rwpWithUntypedDeclarations(t, testKey("x"))
	err := EncryptRWP(t.Context(), fileAsset(t, encrypted), testKey("y"), io.Discard, Options{})
	assert.ErrorIs(t, err, ErrAlreadyProtected)

	// So was font obfuscation declared for a resource which isn't a font.
	obfuscated := strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "style.css", "properties": {"encrypted": {"algorithm": "`+protection.AlgorithmIDPFObfuscation+`"}}},`, 1)
	require.NotEqual(t, testRWPManifest, obfuscated)
	err = EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, obfuscated)), testKey("y"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is not a font")
}

func TestRemoteEPUBDeclarationsStayRemote(t *testing.T) {
	// Declarations were keyed by the path of their URI, whatever its host:
	// one for a remote resource applied to the clear file of the package at
	// the same path.
	for _, uri := range []string{"https://cdn.example.org/OEBPS/images/cover.jpg", "//cdn.example.org/OEBPS/images/cover.jpg"} {
		files := zipContents(t, "testdata/compat-old-lcp.epub")
		remote := `<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + AlgorithmAES256CBC + `"/>` +
			`<KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><RetrievalMethod URI="` + protection.LCPContentKeyRetrievalURI + `" Type="` + protection.LCPContentKeyRetrievalType + `"/></KeyInfo>` +
			`<CipherData><CipherReference URI="` + uri + `"/></CipherData></EncryptedData></encryption>`
		files[protection.PathEPUBEncryption] = []byte(strings.Replace(string(files[protection.PathEPUBEncryption]), "</encryption>", remote, 1))
		var licensed bytes.Buffer
		require.NoError(t, InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "remote.epub", files)), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}), uri)
		path := writeTemp(t, "licensed.epub", licensed.Bytes())

		cover := manifest.MustNewHREFFromString("OEBPS/images/cover.jpg", false)
		builder, err := epubparser.NewParser(nil).Parse(t.Context(), fileAsset(t, path), archiveFetcher(t, path))
		require.NoError(t, err)
		link := builder.Manifest.LinkWithHref(cover.Resolve(nil, nil))
		require.NotNil(t, link)
		assert.Nil(t, link.Properties.Encryption(), uri)

		p, err := openWithLCP(t, path)
		require.NoError(t, err, uri)
		defer p.Close()
		assert.Equal(t, files["OEBPS/images/cover.jpg"], readResource(t, p, manifest.Link{Href: cover}), uri)
	}
}

func TestRWPDeclarationsAreMergedBeforeSelection(t *testing.T) {
	// Only the LCP declarations were merged: a resource also declared
	// otherwise was licensed, and decrypting it dropped both declarations.
	files := encryptedRWPFiles(t, func(doc map[string]any) {
		doc["resources"] = append(doc["resources"].([]any), map[string]any{
			"href": "chapter.html", "properties": map[string]any{"encrypted": map[string]any{"algorithm": protection.AlgorithmIDPFObfuscation}},
		})
	})
	path := zipFiles(t, "conflicting.webpub", files)
	err := InjectLicense(t.Context(), fileAsset(t, path), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorContains(t, err, "declared encrypted differently")
	err = DecryptRWP(t.Context(), fileAsset(t, path), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "declared encrypted differently")

	// So does the decryption of a parsed publication.
	link := func(enc manifest.Encryption) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML, Properties: manifest.Properties{"encrypted": enc.ToMap()}}
	}
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{link(manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC})},
		Resources:    manifest.LinkList{link(manifest.Encryption{Algorithm: protection.AlgorithmIDPFObfuscation})},
	}
	_, err = DecryptPublication(pub.New(m, fetcher.EmptyFetcher{}, nil), testKey("x"))
	assert.ErrorContains(t, err, "declared encrypted differently")
}

func TestRemuxesDecideOncePerResource(t *testing.T) {
	// EncryptRWP asked about every link to a resource when checking that
	// the selected ones are in the package: a later link selecting the
	// resource the first left in the clear made it reported missing.
	calls := make(map[string]int)
	again := strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "chapter.html", "type": "text/html", "title": "again"},`, 1)
	require.NotEqual(t, testRWPManifest, again)
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, again)), testKey("x"), &buf, Options{
		Encrypt: func(l manifest.Link) bool {
			calls[l.Href.String()]++
			return l.Title == "again"
		},
	}))
	assert.Equal(t, testRWPFiles["chapter.html"], zipContents(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))["chapter.html"], "as the first link says")
	assert.Equal(t, 1, calls["chapter.html"])

	// EncryptEPUB asked about the names of a file when checking that they
	// are all encrypted or all in the clear, then again when writing them.
	clear(calls)
	dir := explode(t, "testdata/compat.epub")
	alias(t, "symbolic", filepath.Join(dir, "OEBPS", "chapter1.xhtml"), filepath.Join(dir, "OEBPS", "alias.xhtml"))
	buf.Reset()
	require.NoError(t, EncryptEPUB(t.Context(), fileAsset(t, dir), testKey("x"), &buf, Options{
		// Encrypts what it is first asked about, and nothing afterwards.
		Encrypt: func(l manifest.Link) bool {
			calls[l.Href.String()]++
			return calls[l.Href.String()] == 1
		},
	}))
	for href, n := range calls {
		assert.Equal(t, 1, n, href)
	}
	declarations := string(zipContents(t, writeTemp(t, "encrypted.epub", buf.Bytes()))[protection.PathEPUBEncryption])
	assert.Contains(t, declarations, `URI="OEBPS/chapter1.xhtml"`)
	assert.Contains(t, declarations, `URI="OEBPS/alias.xhtml"`)
}

func TestStreamerTellsPackageFormatsApart(t *testing.T) {
	// A clear Readium Web Publication package holding a file at the path of
	// EPUB licenses was taken for an LCP-protected EPUB, without license.
	source := buildRWP(t)
	files := zipContents(t, source)
	files[protection.PathEPUBLCPLicense] = loadLicense(t, "license-rsa.lcpl").Raw()
	p, err := openWithLCP(t, zipFiles(t, "stray.webpub", files))
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, openPublication(t, source, streamer.Config{}), p)
}

// Fifteenth review

func TestVerificationCoversTheSignatureDelivered(t *testing.T) {
	// The signature of a parsed license wasn't compared with the one of its
	// document, which InjectLicense writes: a license verified with a
	// signature attached after parsing was delivered unsigned.
	signed := loadLicense(t, "license-rsa.lcpl")
	require.NoError(t, signed.VerifyIntegrity())
	canonical, err := signed.canonicalJSON()
	require.NoError(t, err)
	unsigned, err := ParseLicense(canonical)
	require.NoError(t, err)
	require.Nil(t, unsigned.Signature)
	unsigned.Signature = signed.Signature
	assert.ErrorIs(t, unsigned.VerifyIntegrity(), ErrInvalidSignature)
}

func TestRemuxesKeepEachEntry(t *testing.T) {
	// Entry names were taken for encoded hrefs: "page%20one.txt" was
	// another name of "page one.txt", which was written twice, and an
	// encryption.xml entry for "x#y.css" declared "x".
	files := zipContents(t, buildRWP(t))
	names := []string{"extra/page one.txt", "extra/page%20one.txt", "extra/a#b.txt", "extra/100%.txt"}
	for _, name := range names {
		files[name] = []byte(name)
	}
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "names.webpub", files)), testKey("x"), &buf, Options{}))
	output, order := readZip(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	for _, name := range names {
		assert.Equal(t, name, string(output[name].content), name)
	}
	assert.Len(t, order, len(files))

	// Unlisted resources of an EPUB are encrypted, and declared.
	files = zipContents(t, "testdata/compat.epub")
	files["OEBPS/x#y.css"] = textBytes(500)
	path := encryptEPUBToFile(t, zipFiles(t, "names.epub", files), testKey("x"), Options{})
	assert.Contains(t, string(zipContents(t, path)[protection.PathEPUBEncryption]), `URI="OEBPS/x%23y.css"`)
	restored := zipContents(t, decryptEPUBToFile(t, path, testKey("x")))
	assert.Equal(t, files["OEBPS/x#y.css"], restored["OEBPS/x#y.css"])
}

func TestDecryptionRefusesRemoteEncryptedResources(t *testing.T) {
	// Remote resources can't be decrypted: their declarations were dropped,
	// or kept without the license, delivering their ciphertext as if it
	// were in the clear.
	files := zipContents(t, "testdata/compat-old-lcp.epub")
	remote := `<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + AlgorithmAES256CBC + `"/>` +
		`<KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><RetrievalMethod URI="` + protection.LCPContentKeyRetrievalURI + `" Type="` + protection.LCPContentKeyRetrievalType + `"/></KeyInfo>` +
		`<CipherData><CipherReference URI="https://cdn.example.org/video.mp4"/></CipherData></EncryptedData></encryption>`
	files[protection.PathEPUBEncryption] = []byte(strings.Replace(string(files[protection.PathEPUBEncryption]), "</encryption>", remote, 1))
	err := DecryptEPUB(t.Context(), fileAsset(t, zipFiles(t, "remote.epub", files)), testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorContains(t, err, "https://cdn.example.org/video.mp4 is declared LCP-encrypted, but is a remote resource")

	rwp := encryptedRWPFiles(t, func(doc map[string]any) {
		doc["resources"] = append(doc["resources"].([]any), map[string]any{
			"href": "https://cdn.example.org/video.mp4", "type": "video/mp4",
			"properties": map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": AlgorithmAES256CBC}},
		})
	})
	err = DecryptRWP(t.Context(), fileAsset(t, zipFiles(t, "remote.webpub", rwp)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "https://cdn.example.org/video.mp4 is declared LCP-encrypted, but is a remote resource")
}

// zipAsset is a publication asset whose media type is plain ZIP, so that
// the format of the package is told by its files.
type zipAsset struct{ asset.PublicationAsset }

func (zipAsset) MediaType(context.Context) mediatype.MediaType { return mediatype.ZIP }

// probeCounter counts the reads of the files identifying package formats.
type probeCounter struct {
	fetcher.Fetcher
	probes int
}

func (c *probeCounter) Get(ctx context.Context, link manifest.Link) fetcher.Resource {
	if s := link.Href.String(); s == "META-INF/container.xml" || s == rwpManifestPath {
		c.probes++
	}
	return c.Fetcher.Get(ctx, link)
}

func TestPackageFormatsAreToldOnce(t *testing.T) {
	// A Readium Web Publication package converted from an EPUB may keep its
	// META-INF/container.xml: EncryptRWP produced packages which injection
	// then took for EPUBs.
	files := zipContents(t, buildRWP(t))
	files["META-INF/container.xml"] = []byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`)
	err := EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "converted.webpub", files)), testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorContains(t, err, "META-INF/container.xml")

	// Such packages from other producers are what their media type says,
	// as the streamer parses them.
	var buf bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("lcp-compat"), &buf, Options{}))
	files = zipContents(t, writeTemp(t, "encrypted.webpub", buf.Bytes()))
	files["META-INF/container.xml"] = []byte(`<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`)
	var licensed bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, zipFiles(t, "converted.webpub", files)), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}))
	path := writeTemp(t, "licensed.webpub", licensed.Bytes())
	p, err := openWithLCP(t, path)
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, openPublication(t, buildRWP(t), streamer.Config{}), p)
	// ReadLicense tells it as well.
	license, err := ReadLicense(t.Context(), fileAsset(t, path), Options{})
	require.NoError(t, err)
	assert.Equal(t, loadLicense(t, "license-rsa.lcpl").Raw(), license.Raw())

	// The format is told once per opening, when the media type doesn't.
	epub := licensedEPUB(t, "license-rsa.lcpl")
	counter := &probeCounter{Fetcher: archiveFetcher(t, epub)}
	_, err = ContentProtection{InsecureSkipVerify: true}.Open(t.Context(), zipAsset{fileAsset(t, epub)}, counter, testPassphrase)
	require.NoError(t, err)
	assert.Equal(t, 1, counter.probes)
}

// exclusiveReader reads random bytes, counting the reads made while another
// one is in progress.
type exclusiveReader struct {
	busy     atomic.Bool
	overlaps atomic.Int32
}

func (r *exclusiveReader) Read(p []byte) (int, error) {
	if !r.busy.CompareAndSwap(false, true) {
		r.overlaps.Add(1)
	} else {
		defer r.busy.Store(false)
	}
	time.Sleep(2 * time.Millisecond)
	return rand.Read(p)
}

func TestEncryptPublicationSerializesRandomReads(t *testing.T) {
	// The resources of the publication drew their random bytes when
	// fetched, which may be concurrently, from a reader promised to be read
	// from one goroutine at a time.
	dir := t.TempDir()
	m := manifest.Manifest{}
	for i := range 8 {
		name := fmt.Sprintf("chapter%d.html", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), textBytes(100), 0o644))
		m.ReadingOrder = append(m.ReadingOrder, manifest.Link{Href: manifest.MustNewHREFFromString(name, false), MediaType: &mediatype.HTML})
	}
	random := &exclusiveReader{}
	encrypted, err := EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), testKey("x"), Options{Random: random})
	require.NoError(t, err)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, link := range encrypted.Manifest.ReadingOrder {
		wg.Go(func() {
			<-start
			encrypted.Get(t.Context(), link).Close()
		})
	}
	close(start)
	wg.Wait()
	assert.Zero(t, random.overlaps.Load())
}

// Sixteenth review

func TestLongLivedObjectsCopyTheContentKey(t *testing.T) {
	// The publication EncryptPublication returns, and decryptors, kept the
	// caller's key buffer: clearing it afterwards changed the key resources
	// were encrypted, or decrypted, with.
	dir := t.TempDir()
	chapter := textBytes(1000)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), chapter, 0o644))
	link := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{link}}
	key := testKey("copied")
	original := slices.Clone(key)

	encrypted, err := EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), key, Options{})
	require.NoError(t, err)
	clear(key)
	check, err := DecryptPublication(encrypted, slices.Clone(original))
	require.NoError(t, err)
	assert.Equal(t, chapter, readResource(t, check, link), "encrypted with the key given")

	decryptionKey := slices.Clone(original)
	decrypted, err := DecryptPublication(encrypted, decryptionKey)
	require.NoError(t, err)
	clear(decryptionKey)
	assert.Equal(t, chapter, readResource(t, decrypted, link), "decrypted with the key given")
}

func TestDecryptionRequiresTheDeclaredEntries(t *testing.T) {
	// A declared resource missing from the package, e.g. named differently
	// on a case-insensitive file system, lost its declaration with the
	// license, and the decryption succeeded.
	files := encryptedRWPFiles(t, func(map[string]any) {})
	delete(files, "chapter.html")
	err := DecryptRWP(t.Context(), fileAsset(t, zipFiles(t, "missing.webpub", files)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "chapter.html is declared LCP-encrypted, but is not in the package")

	epub := zipContents(t, "testdata/compat-old-lcp.epub")
	delete(epub, "OEBPS/style.css")
	err = DecryptEPUB(t.Context(), fileAsset(t, zipFiles(t, "missing.epub", epub)), testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorContains(t, err, "OEBPS/style.css is declared LCP-encrypted, but is not in the package")

	// Entries the decryption replaces aren't decrypted either.
	files = encryptedRWPFiles(t, func(doc map[string]any) {
		doc["links"] = append(doc["links"].([]any), map[string]any{
			"href": "manifest.json", "type": "application/webpub+json",
			"properties": map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": AlgorithmAES256CBC}},
		})
	})
	err = DecryptRWP(t.Context(), fileAsset(t, zipFiles(t, "replaced.webpub", files)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "manifest.json is declared LCP-encrypted, but is replaced by the decryption")
}

func TestEncryptRWPRefusesRemoteProtection(t *testing.T) {
	// Remote resources already declared encrypted were carried over, in a
	// package DecryptRWP then refused.
	declared := strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "https://cdn.example.org/video.mp4", "type": "video/mp4", "properties": {"encrypted": {"scheme": "`+Scheme+`", "algorithm": "`+AlgorithmAES256CBC+`"}}},`, 1)
	require.NotEqual(t, testRWPManifest, declared)
	err := EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, declared)), testKey("x"), io.Discard, Options{})
	assert.ErrorIs(t, err, ErrAlreadyProtected)

	// Remote resources in the clear are carried over.
	remote := strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "https://cdn.example.org/video.mp4", "type": "video/mp4"},`, 1)
	assert.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWPWith(t, remote)), testKey("x"), io.Discard, Options{}))
}

func TestRemoteEncryptionKeysAreDistinct(t *testing.T) {
	// The absolute URI "file:font.otf" and the local file "./file:font.otf"
	// had the same key: declarations of distinct resources were merged, and
	// one for the remote resource applied to the local file.
	doc := `<?xml version="1.0" encoding="UTF-8"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container">` +
		`<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + AlgorithmAES256CBC + `"/>` +
		`<KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#"><RetrievalMethod URI="` + protection.LCPContentKeyRetrievalURI + `" Type="` + protection.LCPContentKeyRetrievalType + `"/></KeyInfo>` +
		`<CipherData><CipherReference URI="./file:font.otf"/></CipherData></EncryptedData>` +
		`<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + protection.AlgorithmIDPFObfuscation + `"/>` +
		`<CipherData><CipherReference URI="file:font.otf"/></CipherData></EncryptedData></encryption>`
	declared, err := epubEncryptedResources([]byte(doc))
	require.NoError(t, err)
	assert.Equal(t, []string{"file:font.otf"}, slices.Collect(maps.Keys(declared)))

	// The parser doesn't apply the declaration of the remote resource to
	// the local file.
	files := zipContents(t, "testdata/compat.epub")
	files["file:font.otf"] = randomBytes(2000)
	files["OEBPS/content.opf"] = []byte(strings.Replace(string(files["OEBPS/content.opf"]), `<item id="font"`, `<item id="local-font" href="../file:font.otf" media-type="font/otf"/><item id="font"`, 1))
	remote := `<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + protection.AlgorithmIDPFObfuscation + `"/><CipherData><CipherReference URI="file:font.otf"/></CipherData></EncryptedData></encryption>`
	files[protection.PathEPUBEncryption] = []byte(strings.Replace(string(files[protection.PathEPUBEncryption]), "</encryption>", remote, 1))
	path := zipFiles(t, "local-font.epub", files)
	builder, err := epubparser.NewParser(nil).Parse(t.Context(), fileAsset(t, path), archiveFetcher(t, path))
	require.NoError(t, err)
	local := builder.Manifest.LinkWithHref(url.MustURLFromString("./file:font.otf"))
	require.NotNil(t, local)
	assert.Nil(t, local.Properties.Encryption())
}

func TestTransformedResourcesCacheTheirLengthSafely(t *testing.T) {
	// Concurrent reads of one resource raced on its cached length: run with
	// -race.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(5000), 0o644))
	link := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	key := testKey("lengths")
	encrypted, err := EncryptPublication(t.Context(), pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{link}}, fetcher.NewFileFetcher("", dir), nil), key, Options{
		Compress: func(manifest.Link) bool { return false },
	})
	require.NoError(t, err)
	decrypted, err := DecryptPublication(encrypted, key)
	require.NoError(t, err)
	for _, res := range []fetcher.Resource{encrypted.Get(t.Context(), link), decrypted.Get(t.Context(), link)} {
		var wg sync.WaitGroup
		lengths := make([]int64, 8)
		for i := range lengths {
			wg.Go(func() {
				n, rerr := res.Length(t.Context())
				assert.Nil(t, rerr)
				lengths[i] = n
			})
		}
		wg.Wait()
		for _, n := range lengths {
			assert.Equal(t, lengths[0], n)
		}
		res.Close()
	}
}

// Seventeenth review

func TestUnmarshalledLicensesAreParsed(t *testing.T) {
	// Licenses decoded with encoding/json kept neither their document nor
	// its checks: verified from their fields, a member added to a signed
	// license wasn't covered by the signature.
	data := loadLicense(t, "license-rsa.lcpl").Raw()
	var decoded License
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, data, decoded.Raw())
	require.NoError(t, decoded.VerifyIntegrity())

	added := bytes.Replace(data, []byte(`"provider":`), []byte(`"x-added": "unsigned", "provider":`), 1)
	require.NotEqual(t, data, added)
	var holder struct{ License *License }
	require.NoError(t, json.Unmarshal(append(append([]byte(`{"License": `), added...), '}'), &holder))
	assert.ErrorIs(t, holder.License.VerifyIntegrity(), ErrInvalidSignature)

	duplicated := bytes.Replace(data, []byte(`"provider":`), []byte(`"id": "other", "provider":`), 1)
	assert.ErrorIs(t, json.Unmarshal(duplicated, &decoded), ErrInvalidLicense)
}

func assertPublicationHidesLicense(t *testing.T, p *pub.Publication) {
	t.Helper()
	walkDeclarations(p.Manifest, func(l manifest.Link) {
		assert.False(t, isLicenseLink(l), l.Href.String())
	})
	links, err := p.Fetcher.Links(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, links)
	for _, l := range links {
		assert.False(t, isLicenseLink(l), l.Href.String())
	}
	for _, path := range []string{protection.PathEPUBLCPLicense, protection.PathRWPLCPLicense} {
		res := p.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(path, false)})
		data, rerr := res.Read(t.Context(), 0, 0)
		res.Close()
		require.NotNil(t, rerr, path)
		assert.Equal(t, fetcher.CodeNotFound, rerr.Code, path)
		assert.Empty(t, data, path)
	}
}

func TestDecryptedPublicationsHoldNoLicense(t *testing.T) {
	// The license of the recipient was served by the decrypted publication,
	// and so carried over by packagers, disclosing its identifier and
	// passphrase hint, and making the publication look protected.
	license := manifest.Link{Href: manifest.MustNewHREFFromString(protection.PathEPUBLCPLicense, false)}
	source := openPublication(t, licensedEPUB(t, "license-rsa.lcpl"), streamer.Config{})
	_, rerr := source.Get(t.Context(), license).Read(t.Context(), 0, 0)
	require.Nil(t, rerr, "the source serves its license")
	decrypted, err := DecryptPublication(source, testKey("lcp-compat"))
	require.NoError(t, err)
	assertPublicationHidesLicense(t, decrypted)

	// Links of the manifest to the license are dropped as well: packagers
	// would fail to copy it.
	m := source.Manifest
	m.Links = append(slices.Clone(m.Links), manifest.Link{Href: license.Href, MediaType: &mediatype.LCPLicenseDocument})
	linked, err := DecryptPublication(pub.New(m, source.Fetcher, nil), testKey("lcp-compat"))
	require.NoError(t, err)
	assertPublicationHidesLicense(t, linked)
}

func TestEncryptEPUBExemptsThePackageDocumentAsSpelled(t *testing.T) {
	// The package document was exempt by the spelling of its rootfile path,
	// which "./" made differ from its entry: it was encrypted.
	files := zipContents(t, "testdata/compat.epub")
	container := strings.Replace(string(files["META-INF/container.xml"]), `full-path="OEBPS/content.opf"`, `full-path="./OEBPS/content.opf"`, 1)
	require.NotEqual(t, string(files["META-INF/container.xml"]), container)
	files["META-INF/container.xml"] = []byte(container)
	path := encryptEPUBToFile(t, zipFiles(t, "dot.epub", files), testKey("x"), Options{})
	declarations := string(zipContents(t, path)[protection.PathEPUBEncryption])
	assert.NotContains(t, declarations, "content.opf")
	assert.Contains(t, declarations, "chapter1.xhtml")
}

func TestUnsupportedLCPAlgorithmsAreErrors(t *testing.T) {
	// A resource declared with the LCP scheme but another algorithm wasn't
	// taken for LCP-encrypted: decryption copied it, still encrypted, and
	// dropped the license it needs.
	const aes128 = "http://www.w3.org/2001/04/xmlenc#aes128-cbc"
	files := zipContents(t, licensedEPUB(t, "license-rsa.lcpl"))
	doc := string(files[protection.PathEPUBEncryption])
	changed := strings.Replace(doc, `Algorithm="`+AlgorithmAES256CBC+`"`, `Algorithm="`+aes128+`"`, 1)
	require.NotEqual(t, doc, changed)
	files[protection.PathEPUBEncryption] = []byte(changed)
	path := zipFiles(t, "aes128.epub", files)
	err := DecryptEPUB(t.Context(), fileAsset(t, path), testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorContains(t, err, "which isn't supported")
	_, err = openWithLCP(t, path)
	assert.ErrorContains(t, err, "which isn't supported")

	rwp := encryptedRWPFiles(t, func(doc map[string]any) {
		for _, r := range doc["resources"].([]any) {
			if link := r.(map[string]any); link["href"] == "style.css" {
				link["properties"].(map[string]any)["encrypted"].(map[string]any)["algorithm"] = aes128
			}
		}
	})
	err = DecryptRWP(t.Context(), fileAsset(t, zipFiles(t, "aes128.webpub", rwp)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "which isn't supported")

	// Remote resources declared with the LCP scheme can't be decrypted
	// either, whatever their algorithm.
	rwp = encryptedRWPFiles(t, func(doc map[string]any) {
		doc["resources"] = append(doc["resources"].([]any), map[string]any{
			"href": "https://cdn.example.org/video.mp4", "type": "video/mp4",
			"properties": map[string]any{"encrypted": map[string]any{"scheme": Scheme, "algorithm": aes128}},
		})
	})
	err = DecryptRWP(t.Context(), fileAsset(t, zipFiles(t, "remote.webpub", rwp)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "https://cdn.example.org/video.mp4 is declared LCP-encrypted, but is a remote resource")
}

func TestEncryptionRefusesResourcesAtLicensePaths(t *testing.T) {
	// A resource at the path of a license was declared encrypted, but
	// dropped, as licenses are: the package then failed injection.
	listed := strings.Replace(testRWPManifest, `"resources": [`, `"resources": [{"href": "META-INF/license.lcpl", "type": "application/vnd.readium.lcp.license.v1.0+json"},`, 1)
	files := zipContents(t, buildRWPWith(t, listed))
	files[protection.PathEPUBLCPLicense] = loadLicense(t, "license-rsa.lcpl").Raw()
	err := EncryptRWP(t.Context(), fileAsset(t, zipFiles(t, "listed.webpub", files)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "META-INF/license.lcpl is selected for encryption, but is at the path of an LCP license")

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "META-INF"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "license.lcpl"), loadLicense(t, "license-rsa.lcpl").Raw(), 0o644))
	m := manifest.Manifest{Resources: manifest.LinkList{{Href: manifest.MustNewHREFFromString(protection.PathEPUBLCPLicense, false), MediaType: &mediatype.LCPLicenseDocument}}}
	_, err = EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), testKey("x"), Options{
		Encrypt: func(manifest.Link) bool { return true },
	})
	assert.ErrorContains(t, err, "META-INF/license.lcpl is selected for encryption, but is at the path of an LCP license")
}

// Eighteenth review

func TestEncryptedPublicationsHoldNoLicense(t *testing.T) {
	// Selection only considers the reading order and resources: a license
	// the links of the manifest point to was served in the clear, and
	// written by packagers, disclosing a previous recipient's license.
	files := zipContents(t, "testdata/compat.epub")
	files[protection.PathEPUBLCPLicense] = loadLicense(t, "license-rsa.lcpl").Raw()
	source := openPublication(t, zipFiles(t, "licensed.epub", files), streamer.Config{})
	license := manifest.Link{Href: manifest.MustNewHREFFromString(protection.PathEPUBLCPLicense, false), MediaType: &mediatype.LCPLicenseDocument}
	m := source.Manifest
	m.Links = append(slices.Clone(m.Links), license)
	encrypted, err := EncryptPublication(t.Context(), pub.New(m, source.Fetcher, nil), testKey("x"), Options{})
	require.NoError(t, err)
	assertPublicationHidesLicense(t, encrypted)
}

func TestLicenseHiddenFromTemplatedLinks(t *testing.T) {
	// Templates were taken for links outside of the package, while the
	// fetcher expands them: one resolving to the license served it.
	decrypted, err := DecryptPublication(openPublication(t, licensedEPUB(t, "license-rsa.lcpl"), streamer.Config{}), testKey("lcp-compat"))
	require.NoError(t, err)
	templated := manifest.Link{Href: manifest.MustNewHREFFromString(protection.PathEPUBLCPLicense+"{?v}", true)}
	data, rerr := decrypted.Get(t.Context(), templated).Read(t.Context(), 0, 0)
	assert.NotNil(t, rerr, "served %d bytes", len(data))
}

func TestNullLicenses(t *testing.T) {
	// JSON null left a previously decoded license in place.
	var license License
	require.NoError(t, json.Unmarshal(loadLicense(t, "license-rsa.lcpl").Raw(), &license))
	assert.ErrorIs(t, json.Unmarshal([]byte("null"), &license), ErrInvalidLicense)

	// A nil license, e.g. decoded from null into a *License, is an error
	// rather than dereferenced.
	var nilLicense *License
	assert.ErrorIs(t, Verifier{InsecureSkipRevocation: true}.Verify(nilLicense), ErrInvalidLicense)
	assert.ErrorIs(t, nilLicense.VerifySignature(x509.NewCertPool()), ErrInvalidLicense)
	assert.ErrorIs(t, nilLicense.VerifyIntegrity(), ErrInvalidLicense)
}

func TestEncryptEPUBExemptsEveryPackageDocument(t *testing.T) {
	// Only the first rootfile was exempt: the package documents of the
	// other renditions were encrypted.
	files := zipContents(t, "testdata/compat.epub")
	files["OEBPS/content2.opf"] = files["OEBPS/content.opf"]
	files["OEBPS/alt.pdf"] = randomBytes(500)
	container := strings.Replace(string(files["META-INF/container.xml"]), `</rootfiles>`,
		`<rootfile full-path="./OEBPS/content2.opf" media-type="application/oebps-package+xml"/><rootfile full-path="OEBPS/alt.pdf" media-type="application/pdf"/></rootfiles>`, 1)
	require.NotEqual(t, string(files["META-INF/container.xml"]), container)
	files["META-INF/container.xml"] = []byte(container)
	path := encryptEPUBToFile(t, zipFiles(t, "renditions.epub", files), testKey("x"), Options{})
	declarations := string(zipContents(t, path)[protection.PathEPUBEncryption])
	assert.NotContains(t, declarations, "content.opf")
	assert.NotContains(t, declarations, "content2.opf")
	// Rootfiles which aren't package documents are content.
	assert.Contains(t, declarations, "alt.pdf")
}

func TestDecryptPublicationRefusesRemoteEncryptedResources(t *testing.T) {
	// A remote resource kept its declaration, but not its license, once the
	// publication was written.
	source := openPublication(t, licensedEPUB(t, "license-rsa.lcpl"), streamer.Config{})
	m := source.Manifest
	m.Resources = append(slices.Clone(m.Resources), manifest.Link{
		Href: manifest.MustNewHREFFromString("https://cdn.example.org/video.mp4", false), MediaType: &mediatype.MP4,
		Properties: manifest.Properties{"encrypted": manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC}.ToMap()},
	})
	_, err := DecryptPublication(pub.New(m, source.Fetcher, nil), testKey("lcp-compat"))
	assert.ErrorContains(t, err, "https://cdn.example.org/video.mp4 is declared LCP-encrypted, but is a remote resource")
}

// cachingFetcher returns the same slice of links each time, as cloud
// fetchers cache them.
type cachingFetcher struct {
	fetcher.Fetcher
	links manifest.LinkList
}

func (f cachingFetcher) Links(context.Context) (manifest.LinkList, error) { return f.links, nil }

func TestDecryptedListingsLeaveTheSourceIntact(t *testing.T) {
	// The license was filtered out of the source's slice in place.
	link := func(href string) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}
	}
	cached := manifest.LinkList{link(protection.PathEPUBLCPLicense), link("OEBPS/chapter1.xhtml"), link("OEBPS/style.css")}
	original := slices.Clone(cached)
	f := &decryptingFetcher{Fetcher: cachingFetcher{Fetcher: fetcher.EmptyFetcher{}, links: cached}}
	links, err := f.Links(t.Context())
	require.NoError(t, err)
	assert.Len(t, links, 2)
	assert.Equal(t, original, cached)
}

// unlistedAsset is an asset whose fetcher can't list all of its files.
type unlistedAsset struct{ asset.PublicationAsset }

type unlistedFetcher struct{ fetcher.Fetcher }

func (a unlistedAsset) CreateFetcher(ctx context.Context, deps asset.Dependencies, credentials string) (fetcher.Fetcher, error) {
	f, err := a.PublicationAsset.CreateFetcher(ctx, deps, credentials)
	return unlistedFetcher{f}, err
}

func TestRemuxesRequireCompleteListings(t *testing.T) {
	// The remuxes took the links of any fetcher for the files of the
	// package: those a fetcher can't list, e.g. on HTTP or past a page of a
	// cloud listing, were left out of the output.
	err := EncryptRWP(t.Context(), unlistedAsset{fileAsset(t, buildRWP(t))}, testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "can't all be listed")
}

// Nineteenth review

func TestRemoteLookingLinksDontReachTheLicense(t *testing.T) {
	// File fetchers looked up the path of any URL, whatever its scheme or
	// authority: a remote-looking link served the local license.
	dir := explode(t, licensedEPUB(t, "license-rsa.lcpl"))
	decrypted, err := DecryptPublication(pub.New(manifest.Manifest{}, fetcher.NewFileFetcher("", dir), nil), testKey("lcp-compat"))
	require.NoError(t, err)
	for _, href := range []string{"https://cdn.example.org/META-INF/license.lcpl", "//cdn.example.org/META-INF/license.lcpl"} {
		data, rerr := decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
		assert.NotNil(t, rerr, "%s served %d bytes", href, len(data))
	}
}

func TestLicenseAliasesAreErrors(t *testing.T) {
	// Licenses were left out, but other names of their files, e.g. hard
	// links, were checked against the encrypted resources only, and carried
	// over in the clear.
	license := loadLicense(t, "license-rsa.lcpl").Raw()

	// EncryptPublication
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "META-INF"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "license.lcpl"), license, 0o644))
	alias(t, "hard", filepath.Join(dir, "META-INF", "license.lcpl"), filepath.Join(dir, "notes.txt"))
	link := func(href string, mt *mediatype.MediaType) manifest.Link {
		return manifest.Link{Href: manifest.MustNewHREFFromString(href, false), MediaType: mt}
	}
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{link("chapter.html", &mediatype.HTML)}, Resources: manifest.LinkList{link("notes.txt", &mediatype.Text)}}
	_, err := EncryptPublication(t.Context(), pub.New(m, fetcher.NewFileFetcher("", dir), nil), testKey("x"), Options{
		Encrypt: func(l manifest.Link) bool { return l.MediaType.IsHTML() },
	})
	assert.ErrorContains(t, err, "notes.txt is the same file as META-INF/license.lcpl")

	// EncryptRWP, with a stray EPUB license
	dir = explode(t, buildRWP(t))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "META-INF"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "license.lcpl"), license, 0o644))
	alias(t, "hard", filepath.Join(dir, "META-INF", "license.lcpl"), filepath.Join(dir, "extra", "notes.txt"))
	err = EncryptRWP(t.Context(), fileAsset(t, dir), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is the same file as META-INF/license.lcpl")

	// DecryptEPUB
	dir = explode(t, licensedEPUB(t, "license-rsa.lcpl"))
	alias(t, "hard", filepath.Join(dir, "META-INF", "license.lcpl"), filepath.Join(dir, "OEBPS", "notes.txt"))
	err = DecryptEPUB(t.Context(), fileAsset(t, dir), testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is the same file as META-INF/license.lcpl")

	// InjectLicense
	err = InjectLicense(t.Context(), fileAsset(t, dir), loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is the same file as META-INF/license.lcpl")

	// A license spelled differently, on a case-insensitive file system, is
	// read as the license, though its listed name isn't its path.
	dir = explode(t, buildRWP(t))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "META-INF"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "LICENSE.LCPL"), license, 0o644))
	if _, err := os.Stat(filepath.Join(dir, "META-INF", "license.lcpl")); err == nil {
		err = EncryptRWP(t.Context(), fileAsset(t, dir), testKey("x"), io.Discard, Options{})
		assert.ErrorContains(t, err, "META-INF/LICENSE.LCPL is the same file as META-INF/license.lcpl")
	}

	// DecryptRWP
	var encrypted, licensed bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("lcp-compat"), &encrypted, Options{}))
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, writeTemp(t, "encrypted.webpub", encrypted.Bytes())), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}))
	dir = explode(t, writeTemp(t, "licensed.webpub", licensed.Bytes()))
	alias(t, "hard", filepath.Join(dir, "license.lcpl"), filepath.Join(dir, "extra", "notes.txt"))
	err = DecryptRWP(t.Context(), fileAsset(t, dir), testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is the same file as license.lcpl")
}

func TestStreamerDetectsRemoteProtection(t *testing.T) {
	// Only the resources of the package declared LCP-encrypted made it
	// protected: with only remote ones, a license given separately wasn't
	// verified, and the publication was opened as unprotected.
	remote := strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "https://cdn.example.org/video.mp4", "type": "video/mp4", "properties": {"encrypted": {"scheme": "`+Scheme+`", "algorithm": "`+AlgorithmAES256CBC+`"}}},`, 1)
	require.NotEqual(t, testRWPManifest, remote)
	verified := false
	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{
			License: loadLicense(t, "license-rsa.lcpl"),
			Verify:  func(*License) error { verified = true; return nil },
		}},
	}).Open(t.Context(), fileAsset(t, buildRWPWith(t, remote)), testPassphrase)
	require.NoError(t, err)
	p.Close()
	assert.True(t, verified, "the license is verified")
}

func TestEncryptEPUBKeepsTheRolesOfEveryRendition(t *testing.T) {
	// Only the first rendition told which resources are the cover and the
	// navigation document: those of other renditions were encrypted.
	files := zipContents(t, "testdata/compat.epub")
	opf := string(files["OEBPS/content.opf"])
	second := strings.NewReplacer(`href="nav.xhtml"`, `href="nav2.xhtml"`, `href="images/cover.jpg"`, `href="images/cover2.jpg"`).Replace(opf)
	require.NotEqual(t, opf, second)
	files["OEBPS/content2.opf"] = []byte(second)
	files["OEBPS/nav2.xhtml"] = files["OEBPS/nav.xhtml"]
	files["OEBPS/images/cover2.jpg"] = files["OEBPS/images/cover.jpg"]
	files["META-INF/container.xml"] = []byte(strings.Replace(string(files["META-INF/container.xml"]), `</rootfiles>`,
		`<rootfile full-path="OEBPS/content2.opf" media-type="application/oebps-package+xml"/></rootfiles>`, 1))
	path := encryptEPUBToFile(t, zipFiles(t, "renditions.epub", files), testKey("x"), Options{})
	declarations := string(zipContents(t, path)[protection.PathEPUBEncryption])
	assert.NotContains(t, declarations, "nav2.xhtml")
	assert.NotContains(t, declarations, "cover2.jpg")
	assert.Contains(t, declarations, "chapter1.xhtml")

	// A resource of both renditions has the roles it has in any of them.
	figure := strings.Replace(second, `href="images/figure.png" media-type="image/png"`, `href="images/figure.png" media-type="image/png" properties="cover-image"`, 1)
	require.NotEqual(t, second, figure)
	files["OEBPS/content2.opf"] = []byte(figure)
	path = encryptEPUBToFile(t, zipFiles(t, "renditions.epub", files), testKey("x"), Options{})
	assert.NotContains(t, string(zipContents(t, path)[protection.PathEPUBEncryption]), "figure.png")

	// Font obfuscation declared for a resource another rendition lists, as
	// something else than a font, is an error, as for the first rendition.
	files["OEBPS/content2.opf"] = []byte(strings.Replace(second, `</manifest>`, `<item id="notes" href="notes.otf" media-type="text/plain"/></manifest>`, 1))
	files["OEBPS/notes.otf"] = textBytes(2000)
	obfuscated := `<EncryptedData xmlns="http://www.w3.org/2001/04/xmlenc#"><EncryptionMethod Algorithm="` + protection.AlgorithmIDPFObfuscation + `"/><CipherData><CipherReference URI="OEBPS/notes.otf"/></CipherData></EncryptedData></encryption>`
	files[protection.PathEPUBEncryption] = []byte(strings.Replace(string(files[protection.PathEPUBEncryption]), "</encryption>", obfuscated, 1))
	err := EncryptEPUB(t.Context(), fileAsset(t, zipFiles(t, "renditions.epub", files)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is not a font")
}

func TestInjectLicenseRefusesNilLicenses(t *testing.T) {
	// A nil license, e.g. decoded from JSON null into a *License, was
	// dereferenced.
	var buf bytes.Buffer
	err := InjectLicense(t.Context(), fileAsset(t, "testdata/compat-old-lcp.epub"), nil, &buf, Options{})
	assert.ErrorIs(t, err, ErrInvalidLicense)
	assert.Zero(t, buf.Len())
}

// Twentieth review

func TestDecryptedPublicationsHideLicenseAliases(t *testing.T) {
	// The decrypted publication hid the license by path only: another name
	// of its file, e.g. a hard link, served it.
	dir := explode(t, licensedEPUB(t, "license-rsa.lcpl"))
	alias(t, "hard", filepath.Join(dir, "META-INF", "license.lcpl"), filepath.Join(dir, "OEBPS", "notes.txt"))
	decrypted, err := DecryptPublication(pub.New(manifest.Manifest{}, fetcher.NewFileFetcher("", dir), nil), testKey("lcp-compat"))
	require.NoError(t, err)
	names := []string{"OEBPS/notes.txt"}
	if _, err := os.Stat(filepath.Join(dir, "META-INF", "LICENSE.LCPL")); err == nil {
		names = append(names, "META-INF/LICENSE.LCPL") // a case-insensitive file system
	}
	for _, name := range names {
		data, rerr := decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(name, false)}).Read(t.Context(), 0, 0)
		assert.NotNil(t, rerr, "%s served %d bytes", name, len(data))
	}
	links, err := decrypted.Fetcher.Links(t.Context())
	require.NoError(t, err)
	for _, l := range links {
		assert.NotEqual(t, "OEBPS/notes.txt", l.Href.String())
	}
	// The other resources are served, from the files checked not to be the
	// license: one replaced by it afterwards isn't read.
	_, rerr := decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("OEBPS/nav.xhtml", false)}).Read(t.Context(), 0, 0)
	assert.Nil(t, rerr)
	cover := decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("OEBPS/images/cover.jpg", false)})
	defer cover.Close()
	require.NoError(t, os.Remove(filepath.Join(dir, "OEBPS", "images", "cover.jpg")))
	require.NoError(t, os.Link(filepath.Join(dir, "META-INF", "license.lcpl"), filepath.Join(dir, "OEBPS", "images", "cover.jpg")))
	_, rerr = cover.Read(t.Context(), 0, 0)
	assert.ErrorIs(t, rerr, fetcher.ErrFileChanged)
}

// epubFetcherAsset is an EPUB asset of the given fetcher.
type epubFetcherAsset struct{ fetcherAsset }

func (epubFetcherAsset) Name() string                                  { return "package.epub" }
func (epubFetcherAsset) MediaType(context.Context) mediatype.MediaType { return mediatype.EPUB }

func TestDeliveriesCopyTheFilesTheyChecked(t *testing.T) {
	// The files checked not to be other names of the license were opened
	// again to be copied: one replaced by the license meanwhile was
	// delivered in its place.
	replaceWithLicense := func(dir, path, license string) func(string, int) {
		return func(p string, gets int) {
			// The first get identifies the file, the second copies it.
			if p == path && gets == 2 {
				target := filepath.Join(dir, filepath.FromSlash(path))
				require.NoError(t, os.Remove(target))
				require.NoError(t, os.Link(filepath.Join(dir, filepath.FromSlash(license)), target))
			}
		}
	}
	epub := func(t *testing.T, path string) (string, *hookFetcher) {
		dir := explode(t, path)
		return dir, &hookFetcher{Fetcher: archiveFetcher(t, dir), gets: make(map[string]int)}
	}

	dir, f := epub(t, licensedEPUB(t, "license-rsa.lcpl"))
	f.hook = replaceWithLicense(dir, "OEBPS/images/cover.jpg", protection.PathEPUBLCPLicense)
	err := DecryptEPUB(t.Context(), epubFetcherAsset{fetcherAsset{f}}, testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorIs(t, err, fetcher.ErrFileChanged, "DecryptEPUB")

	dir, f = epub(t, licensedEPUB(t, "license-rsa.lcpl"))
	f.hook = replaceWithLicense(dir, "OEBPS/images/cover.jpg", protection.PathEPUBLCPLicense)
	err = InjectLicense(t.Context(), epubFetcherAsset{fetcherAsset{f}}, loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorIs(t, err, fetcher.ErrFileChanged, "InjectLicense")

	var encrypted, licensed bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, buildRWP(t)), testKey("lcp-compat"), &encrypted, Options{}))
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, writeTemp(t, "encrypted.webpub", encrypted.Bytes())), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}))
	dir = explode(t, writeTemp(t, "licensed.webpub", licensed.Bytes()))
	f = &hookFetcher{Fetcher: archiveFetcher(t, dir), gets: make(map[string]int)}
	f.hook = replaceWithLicense(dir, "extra/unlisted.json", protection.PathRWPLCPLicense)
	err = DecryptRWP(t.Context(), fetcherAsset{f}, testKey("lcp-compat"), io.Discard, Options{})
	assert.ErrorIs(t, err, fetcher.ErrFileChanged, "DecryptRWP")
}

func TestEncryptEPUBChecksTheFontsOfEveryRendition(t *testing.T) {
	// Renditions were reduced to one link per resource before checking that
	// obfuscated resources are fonts: one the first rendition lists as a
	// font, and another as a document, stayed obfuscated rather than
	// encrypted.
	files := zipContents(t, "testdata/compat.epub")
	opf := string(files["OEBPS/content.opf"])
	second := strings.Replace(opf, `href="fonts/font.otf" media-type="font/otf"`, `href="fonts/font.otf" media-type="application/xhtml+xml"`, 1)
	require.NotEqual(t, opf, second)
	files["OEBPS/content2.opf"] = []byte(second)
	files["META-INF/container.xml"] = []byte(strings.Replace(string(files["META-INF/container.xml"]), `</rootfiles>`,
		`<rootfile full-path="OEBPS/content2.opf" media-type="application/oebps-package+xml"/></rootfiles>`, 1))
	err := EncryptEPUB(t.Context(), fileAsset(t, zipFiles(t, "renditions.epub", files)), testKey("x"), io.Discard, Options{})
	assert.ErrorContains(t, err, "is not a font")
}

func TestStreamerDecryptsRemoteResources(t *testing.T) {
	// The remote resources of a publication were only decrypted when read
	// with their link, declaring their encryption, rather than their href.
	key := testKey("remote")
	plain := textBytes(3000)
	ciphertext, enc := encryptedResource(t, key, plain, false)
	data, rerr := ciphertext.Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	href := manifest.MustNewHREFFromString("https://cdn.example.org/chapter.html", false)
	m := manifest.Manifest{ReadingOrder: manifest.LinkList{{Href: href, MediaType: &mediatype.HTML, Properties: manifest.Properties{"encrypted": enc.ToMap()}}}}
	b := pub.NewBuilder(m, servingFetcher{data: data}, nil)
	require.NoError(t, DecryptBuilder(b, key))
	read, rerr := b.Fetcher.Get(t.Context(), manifest.Link{Href: href}).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, plain, read)

	// An algorithm the package doesn't support is an error, rather than
	// served still encrypted.
	unsupported := enc
	unsupported.Algorithm = "http://www.w3.org/2001/04/xmlenc#aes128-cbc"
	m.ReadingOrder[0].Properties = manifest.Properties{"encrypted": unsupported.ToMap()}
	err := DecryptBuilder(pub.NewBuilder(m, servingFetcher{data: data}, nil), key)
	assert.ErrorContains(t, err, "which isn't supported")
	d, err := NewDecryptor(key)
	require.NoError(t, err)
	_, rerr = d.Transform(fetcher.NewBytesResource(m.ReadingOrder[0], func() []byte { return data })).Read(t.Context(), 0, 0)
	assert.NotNil(t, rerr)
}

// servingFetcher serves the same bytes for every link, remote or not.
type servingFetcher struct {
	fetcher.EmptyFetcher
	data []byte
}

func (f servingFetcher) Get(_ context.Context, link manifest.Link) fetcher.Resource {
	return fetcher.NewBytesResource(link, func() []byte { return f.data })
}

// Twenty-first review

func TestDecryptedPublicationsCheckEveryFile(t *testing.T) {
	dir := explode(t, licensedEPUB(t, "license-rsa.lcpl"))
	decrypted, err := DecryptPublication(pub.New(manifest.Manifest{}, fetcher.NewFileFetcher("", dir), nil), testKey("lcp-compat"))
	require.NoError(t, err)
	get := func(path string) fetcher.Resource {
		return decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(path, false)})
	}
	license := filepath.Join(dir, "META-INF", "license.lcpl")

	// A resource whose file couldn't be identified was served unpinned:
	// the license linked at its path afterwards was read.
	late := get("OEBPS/late.txt")
	defer late.Close()
	require.NoError(t, os.Link(license, filepath.Join(dir, "OEBPS", "late.txt")))
	data, rerr := late.Read(t.Context(), 0, 0)
	assert.NotNil(t, rerr, "served %d bytes", len(data))

	// The files of the licenses were identified once: a license replaced
	// afterwards, by renaming a new file over it, had other names.
	_, rerr = get("OEBPS/nav.xhtml").Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	replacement := filepath.Join(dir, "META-INF", "license.new")
	require.NoError(t, os.WriteFile(replacement, loadLicense(t, "license-rsa.lcpl").Raw(), 0o644))
	require.NoError(t, os.Rename(replacement, license))
	require.NoError(t, os.Link(license, filepath.Join(dir, "OEBPS", "notes.txt")))
	data, rerr = get("OEBPS/notes.txt").Read(t.Context(), 0, 0)
	assert.NotNil(t, rerr, "served %d bytes", len(data))
}

func TestEncryptPublicationPinsSelectedResources(t *testing.T) {
	// Only the resources served in the clear were pinned to the files
	// checked: a selected one replaced by the license afterwards was
	// encrypted for the new recipient.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chapter.html"), textBytes(1000), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "META-INF"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "META-INF", "license.lcpl"), loadLicense(t, "license-rsa.lcpl").Raw(), 0o644))
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("chapter.html", false), MediaType: &mediatype.HTML}
	encrypted, err := EncryptPublication(t.Context(), pub.New(manifest.Manifest{ReadingOrder: manifest.LinkList{chapter}}, fetcher.NewFileFetcher("", dir), nil), testKey("x"), Options{})
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(dir, "chapter.html")))
	require.NoError(t, os.Link(filepath.Join(dir, "META-INF", "license.lcpl"), filepath.Join(dir, "chapter.html")))
	_, rerr := encrypted.Get(t.Context(), chapter).Read(t.Context(), 0, 0)
	assert.ErrorIs(t, rerr, fetcher.ErrFileChanged)
}

func TestInjectLicenseChecksTheFilesItIdentified(t *testing.T) {
	// The declared resources were identified again to be checked: a file
	// replaced after the first identification, by one of the expected size,
	// was delivered.
	dir := explode(t, "testdata/compat-old-lcp.epub")
	path := "OEBPS/chapter1.xhtml"
	f := &hookFetcher{Fetcher: archiveFetcher(t, dir), gets: make(map[string]int), hook: func(p string, gets int) {
		// The first get identifies the file, the second checks it.
		if p == path && gets == 2 {
			target := filepath.Join(dir, filepath.FromSlash(path))
			data, err := os.ReadFile(target)
			require.NoError(t, err)
			require.NoError(t, os.Remove(target))
			require.NoError(t, os.WriteFile(target, data, 0o644))
		}
	}}
	err := InjectLicense(t.Context(), epubFetcherAsset{fetcherAsset{f}}, loadLicense(t, "license-rsa.lcpl"), io.Discard, Options{})
	assert.ErrorContains(t, err, path+" changed since it was checked")
}

func TestHTTPPublicationsHideTheirLicense(t *testing.T) {
	// The license was hidden by the path of the links, while HTTP fetchers
	// resolve them against their URL: a network-path or root-absolute
	// reference to it was served.
	dir := explode(t, licensedEPUB(t, "license-rsa.lcpl"))
	server := httptest.NewServer(http.StripPrefix("/pub/", http.FileServer(http.Dir(dir))))
	t.Cleanup(server.Close)
	base, err := url.AbsoluteURLFromString(server.URL + "/pub/")
	require.NoError(t, err)
	f := fetcher.NewHTTPFetcher("", server.Client(), base)
	host := strings.TrimPrefix(server.URL, "http:")
	spellings := []string{host + "/pub/META-INF/license.lcpl", "/pub/META-INF/license.lcpl", "/pub/META-INF/%6cicense.lcpl"}

	decrypted, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{InsecureSkipVerify: true}},
	}).Open(t.Context(), epubFetcherAsset{fetcherAsset{f}}, testPassphrase)
	require.NoError(t, err)
	_, rerr := decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("OEBPS/nav.xhtml", false)}).Read(t.Context(), 0, 0)
	require.Nil(t, rerr, "the other resources are served")
	for _, href := range spellings {
		data, rerr := decrypted.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
		assert.NotNil(t, rerr, "%s served %d bytes", href, len(data))
	}

	// Nor is it served in the clear by an encrypted publication.
	m := manifest.Manifest{
		ReadingOrder: manifest.LinkList{{Href: manifest.MustNewHREFFromString("OEBPS/chapter1.xhtml", false), MediaType: &mediatype.XHTML}},
		Resources:    manifest.LinkList{{Href: manifest.MustNewHREFFromString("/pub/META-INF/license.lcpl", false), MediaType: &mediatype.Text}},
	}
	_, err = EncryptPublication(t.Context(), pub.New(m, f, nil), testKey("x"), Options{
		Encrypt: func(l manifest.Link) bool { return l.MediaType.IsHTML() },
	})
	assert.ErrorContains(t, err, "is the same file as META-INF/license.lcpl")
}

func TestRemoteDeclarationsAreMergedBeforeSelection(t *testing.T) {
	key := testKey("remote")
	href := manifest.MustNewHREFFromString("https://cdn.example.org/font.otf", false)
	font := []byte("an obfuscated font")
	link := func(enc manifest.Encryption) manifest.Link {
		return manifest.Link{Href: href, MediaType: &mediatype.OTF, Properties: manifest.Properties{"encrypted": enc.ToMap()}}
	}

	// A remote font obfuscated as older producers declared it, with the LCP
	// scheme, was decrypted with AES.
	legacy := manifest.Encryption{Scheme: Scheme, Algorithm: protection.AlgorithmIDPFObfuscation}
	b := pub.NewBuilder(manifest.Manifest{Resources: manifest.LinkList{link(legacy)}}, servingFetcher{data: font}, nil)
	require.NoError(t, DecryptBuilder(b, key))
	data, rerr := b.Fetcher.Get(t.Context(), manifest.Link{Href: href}).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.Equal(t, font, data)

	export, err := DecryptPublication(pub.New(manifest.Manifest{Resources: manifest.LinkList{link(legacy)}}, servingFetcher{data: font}, nil), key)
	require.NoError(t, err)
	assert.Equal(t, legacy.Algorithm, export.Manifest.Resources[0].Properties.Encryption().Algorithm)
	export.Close()

	// Declarations of a remote resource which can't hold together are an
	// error, as for the resources of the package.
	other := manifest.Encryption{Scheme: protection.SchemeAdept, Algorithm: AlgorithmAES256CBC}
	m := manifest.Manifest{Resources: manifest.LinkList{link(manifest.Encryption{Scheme: Scheme, Algorithm: AlgorithmAES256CBC}), link(other)}}
	err = DecryptBuilder(pub.NewBuilder(m, servingFetcher{data: font}, nil), key)
	assert.ErrorContains(t, err, "declared encrypted differently")
}

func TestStreamerKeepsRawRemoteDeclarations(t *testing.T) {
	// The declarations of remote resources were taken from the parsed
	// manifest only, which drops the links without a media type: once the
	// license verified, such a resource was served encrypted.
	key := testKey("lcp-compat")
	plain := textBytes(2000)
	ciphertext, enc := encryptedResource(t, key, plain, false)
	data, rerr := ciphertext.Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	encJSON, err := json.Marshal(enc.ToMap())
	require.NoError(t, err)
	untyped := strings.Replace(testRWPManifest, `{"href": "style.css", "type": "text/css"},`,
		`{"href": "style.css", "type": "text/css"}, {"href": "https://cdn.example.org/video.mp4", "properties": {"encrypted": `+string(encJSON)+`}},`, 1)
	require.NotEqual(t, testRWPManifest, untyped)
	declared, err := packageDeclarations(t.Context(), archiveFetcher(t, buildRWPWith(t, untyped)), rwpFormat)
	require.NoError(t, err)
	b := pub.NewBuilder(manifest.Manifest{}, servingFetcher{data: data}, nil)
	require.NoError(t, decryptBuilder(b, key, 0, declared))
	for _, href := range []string{"https://cdn.example.org/video.mp4", "https://cdn.example.org/video.mp4#t=10"} {
		read, rerr := b.Fetcher.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
		require.Nil(t, rerr, href)
		assert.Equal(t, plain, read, href)
	}
}

func TestEncryptPublicationReadsApprovedHTTPLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.RequestURI())
	}))
	defer srv.Close()
	base, err := url.AbsoluteURLFromString(srv.URL + "/book/")
	require.NoError(t, err)
	chapter := manifest.Link{Href: manifest.MustNewHREFFromString("book/chapter?edition=1", false)}
	f := fetcher.NewHTTPFetcher("", srv.Client(), base)
	p, err := EncryptPublication(t.Context(), pub.New(manifest.Manifest{Resources: manifest.LinkList{chapter}}, f, nil), testKey("x"), Options{Encrypt: func(manifest.Link) bool { return false }})
	require.NoError(t, err)
	defer p.Close()
	for _, href := range []string{"book/chapter?edition=1", "/book/chapter", "book/chapter?edition=2"} {
		data, rerr := p.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)}).Read(t.Context(), 0, 0)
		require.Nil(t, rerr)
		assert.Equal(t, "/book/book/chapter?edition=1", string(data))
	}
	other := manifest.Link{Href: manifest.MustNewHREFFromString("book/chapter?edition=2", false)}
	_, err = EncryptPublication(t.Context(), pub.New(manifest.Manifest{Resources: manifest.LinkList{chapter, other}}, f, nil), testKey("x"), Options{Encrypt: func(manifest.Link) bool { return false }})
	require.ErrorContains(t, err, "different sources for the same package path")

}

type unavailableLicenseFetcher struct{ fetcher.Fetcher }

func (f unavailableLicenseFetcher) Get(ctx context.Context, l manifest.Link) fetcher.Resource {
	if isLicenseLink(l) {
		return fetcher.NewFailureResource(l, fetcher.Forbidden(fs.ErrPermission))
	}
	return f.Fetcher.Get(ctx, l)
}

func TestUnknownLicenseFileIdentity(t *testing.T) {
	link := manifest.Link{Href: manifest.MustNewHREFFromString("chapter", false)}
	f := unavailableLicenseFetcher{servingFetcher{data: []byte("chapter")}}
	p := pub.New(manifest.Manifest{Resources: manifest.LinkList{link}}, f, nil)
	_, err := EncryptPublication(t.Context(), p, testKey("x"), Options{Encrypt: func(manifest.Link) bool { return false }})
	require.ErrorIs(t, err, fs.ErrPermission)
	decrypted, err := DecryptPublication(p, testKey("x"))
	require.NoError(t, err)
	_, rerr := decrypted.Get(t.Context(), link).Read(t.Context(), 0, 0)
	require.ErrorIs(t, rerr, fs.ErrPermission)
}

func TestCheckResourceRequiresKnownFile(t *testing.T) {
	id, err := os.Stat("testdata/compat.epub")
	require.NoError(t, err)
	_, err = checkResource(t.Context(), servingFetcher{data: []byte("test")}, containerEntry{path: "chapter"}, id)
	require.ErrorContains(t, err, "changed since it was checked")
}

func TestRemoteLegacyFontInRWP(t *testing.T) {
	font := `{"href":"https://example.org/font.otf","type":"font/otf","properties":{"encrypted":{"scheme":"` + Scheme + `","algorithm":"` + protection.AlgorithmIDPFObfuscation + `"}}}`
	raw := strings.Replace(testRWPManifest, `"resources": [`, `"resources": [`+font+",", 1)
	require.NotEqual(t, testRWPManifest, raw)
	source := buildRWPWith(t, raw)
	protected, err := isLCPProtected(t.Context(), archiveFetcher(t, source), rwpFormat)
	require.NoError(t, err)
	assert.False(t, protected)
	var encrypted, clear bytes.Buffer
	key := testKey("legacy")
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, source), key, &encrypted, Options{}))
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, writeTemp(t, "encrypted.webpub", encrypted.Bytes())), key, &clear, Options{}))
	entries, _ := readZip(t, writeTemp(t, "clear.webpub", clear.Bytes()))
	assert.Contains(t, string(entries["manifest.json"].content), protection.AlgorithmIDPFObfuscation)
}
