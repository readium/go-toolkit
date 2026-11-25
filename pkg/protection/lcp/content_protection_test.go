package lcp

import (
	"bytes"
	"io"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// licensedEPUB returns the path of compat-old-lcp.epub with the given license
// embedded.
func licensedEPUB(t *testing.T, licenseName string) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, "testdata/compat-old-lcp.epub"), loadLicense(t, licenseName), &buf, Options{}))
	return writeTemp(t, "licensed.epub", buf.Bytes())
}

func archiveFetcher(t *testing.T, path string) fetcher.Fetcher {
	t.Helper()
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(f.Close)
	return f
}

func openProtected(t *testing.T, path string, cp ContentProtection, credentials string) error {
	t.Helper()
	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{cp},
	}).Open(t.Context(), fileAsset(t, path), credentials)
	if err == nil {
		p.Close()
	}
	return err
}

func TestInjectLicenseEPUB(t *testing.T) {
	licensed := licensedEPUB(t, "license-rsa.lcpl")
	assert.Equal(t, protection.LCP, identify(t, licensed))

	source, sourceNames := readZip(t, "testdata/compat-old-lcp.epub")
	output, outputNames := readZip(t, licensed)
	assert.Equal(t, "mimetype", outputNames[0])
	assert.ElementsMatch(t, append(sourceNames, protection.PathEPUBLCPLicense), outputNames)
	for name, src := range source {
		switch name {
		case "mimetype":
			continue
		case protection.PathEPUBEncryption:
			// Written as it was read and checked, compressed again.
			assert.Equal(t, src.content, output[name].content, name)
			continue
		}
		assert.True(t, bytes.Equal(src.raw, output[name].raw), "%s not copied verbatim", name)
	}

	// The license is embedded byte for byte, and still verifies.
	license := loadLicense(t, "license-rsa.lcpl")
	assert.Equal(t, license.Raw(), output[protection.PathEPUBLCPLicense].content)
	read, err := ReadLicense(t.Context(), fileAsset(t, licensed), Options{})
	require.NoError(t, err)
	assert.NoError(t, read.VerifySignature(loadRoots(t, "test-root-ca.pem")))

	// Injecting again replaces the license.
	var buf bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, licensed), loadLicense(t, "license-ecdsa.lcpl"), &buf, Options{}))
	replaced, names := readZip(t, writeTemp(t, "replaced.epub", buf.Bytes()))
	assert.Len(t, names, len(outputNames))
	assert.Equal(t, loadLicense(t, "license-ecdsa.lcpl").Raw(), replaced[protection.PathEPUBLCPLicense].content)
}

func TestInjectLicenseRefusesUnprotected(t *testing.T) {
	license := loadLicense(t, "license-rsa.lcpl")
	for _, path := range []string{"testdata/compat.epub", buildRWP(t)} {
		err := InjectLicense(t.Context(), fileAsset(t, path), license, io.Discard, Options{})
		assert.ErrorIs(t, err, ErrNotProtected, path)
	}
}

func TestReadLicenseMissing(t *testing.T) {
	_, err := ReadLicense(t.Context(), fileAsset(t, "testdata/compat-old-lcp.epub"), Options{})
	assert.ErrorIs(t, err, ErrNoLicense)
}

func TestContentProtectionEPUB(t *testing.T) {
	licensed := licensedEPUB(t, "license-rsa.lcpl")
	cleartext := openPublication(t, "testdata/compat.epub", streamer.Config{})

	// Unlocked with the passphrase given to the streamer.
	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{InsecureSkipVerify: true}},
	}).Open(t.Context(), fileAsset(t, licensed), testPassphrase)
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, cleartext, p)
	assert.NotEmpty(t, p.Positions(t.Context()), "services are kept")

	// Unlocked with a user key.
	uk, err := UserKeyFromPassphrase(testPassphrase, ProfileBasic)
	require.NoError(t, err)
	assert.NoError(t, openProtected(t, licensed, ContentProtection{InsecureSkipVerify: true, UserKeys: []UserKey{UserKey(testKey("other")), uk}}, ""))

	// Locked.
	err = openProtected(t, licensed, ContentProtection{InsecureSkipVerify: true}, "wrong passphrase")
	assert.ErrorIs(t, err, ErrWrongUserKey)
	err = openProtected(t, licensed, ContentProtection{InsecureSkipVerify: true, UserKeys: []UserKey{UserKey(testKey("other"))}}, "")
	assert.ErrorIs(t, err, ErrWrongUserKey)
	err = openProtected(t, licensed, ContentProtection{InsecureSkipVerify: true}, "")
	assert.ErrorIs(t, err, ErrNoCredentials)
	assert.ErrorContains(t, err, "Ton mot de passe préféré")
}

func TestContentProtectionVerify(t *testing.T) {
	licensed := licensedEPUB(t, "license-ecdsa.lcpl")
	verifier := Verifier{Roots: loadRoots(t, "test-root-ca.pem"), InsecureSkipRevocation: true}
	assert.NoError(t, openProtected(t, licensed, ContentProtection{Verify: verifier.Verify}, testPassphrase))

	err := openProtected(t, licensed, ContentProtection{Verify: Verifier{Roots: loadRoots(t, "edrlab-test-root-ca-2026.pem"), InsecureSkipRevocation: true}.Verify}, testPassphrase)
	assert.ErrorIs(t, err, ErrInvalidCertificate)

	// Validating licenses is required, unless explicitly skipped.
	err = openProtected(t, licensed, ContentProtection{}, testPassphrase)
	assert.ErrorIs(t, err, errNoVerify)
	assert.NoError(t, openProtected(t, licensed, ContentProtection{InsecureSkipVerify: true}, testPassphrase))
	// Unprotected publications don't need it.
	assert.NoError(t, openProtected(t, "testdata/compat.epub", ContentProtection{}, ""))
}

func TestContentProtectionLicenseOption(t *testing.T) {
	// The publication declares LCP encryption but holds no license.
	err := openProtected(t, "testdata/compat-old-lcp.epub", ContentProtection{InsecureSkipVerify: true}, testPassphrase)
	assert.ErrorIs(t, err, ErrNoLicense)

	cp := ContentProtection{InsecureSkipVerify: true, License: loadLicense(t, "license-rsa.lcpl")}
	p, err := streamer.New(streamer.Config{ContentProtections: []protection.ContentProtection{cp}}).
		Open(t.Context(), fileAsset(t, "testdata/compat-old-lcp.epub"), testPassphrase)
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, openPublication(t, "testdata/compat.epub", streamer.Config{}), p)
}

func TestContentProtectionIgnoresUnprotected(t *testing.T) {
	for _, path := range []string{"testdata/compat.epub", buildRWP(t)} {
		assert.NoError(t, openProtected(t, path, ContentProtection{InsecureSkipVerify: true}, ""), path)
	}
}

func TestContentProtectionRWP(t *testing.T) {
	key := testKey("lcp-compat")
	source := buildRWP(t)
	var encrypted bytes.Buffer
	require.NoError(t, EncryptRWP(t.Context(), fileAsset(t, source), key, &encrypted, Options{}))
	var licensed bytes.Buffer
	require.NoError(t, InjectLicense(t.Context(), fileAsset(t, writeTemp(t, "encrypted.webpub", encrypted.Bytes())), loadLicense(t, "license-rsa.lcpl"), &licensed, Options{}))
	path := writeTemp(t, "licensed.webpub", licensed.Bytes())

	_, names := readZip(t, path)
	assert.Equal(t, []string{"manifest.json", protection.PathRWPLCPLicense}, names[:2])

	p, err := streamer.New(streamer.Config{
		ContentProtections: []protection.ContentProtection{ContentProtection{InsecureSkipVerify: true}},
	}).Open(t.Context(), fileAsset(t, path), testPassphrase)
	require.NoError(t, err)
	defer p.Close()
	assertSameResources(t, openPublication(t, source, streamer.Config{}), p)

	// Decrypting the package with the content key from the license removes it.
	license, err := ReadLicense(t.Context(), fileAsset(t, path), Options{})
	require.NoError(t, err)
	contentKey, err := license.ContentKeyFromPassphrase(testPassphrase)
	require.NoError(t, err)
	var restored bytes.Buffer
	require.NoError(t, DecryptRWP(t.Context(), fileAsset(t, path), contentKey, &restored, Options{}))
	_, restoredNames := readZip(t, writeTemp(t, "restored.webpub", restored.Bytes()))
	assert.NotContains(t, restoredNames, protection.PathRWPLCPLicense)
}

func TestUnlockAndDecryptEPUB(t *testing.T) {
	// The convert --lcp-passphrase flow: license → content key → remux.
	licensed := licensedEPUB(t, "license-rsa.lcpl")
	license, err := ReadLicense(t.Context(), fileAsset(t, licensed), Options{})
	require.NoError(t, err)
	contentKey, err := license.ContentKeyFromPassphrase(testPassphrase)
	require.NoError(t, err)

	restored, names := readZip(t, decryptEPUBToFile(t, licensed, contentKey))
	assert.NotContains(t, names, protection.PathEPUBLCPLicense)
	source, _ := readZip(t, "testdata/compat.epub")
	for name, src := range source {
		if name != protection.PathEPUBEncryption {
			assert.True(t, bytes.Equal(src.content, restored[name].content), "%s differs", name)
		}
	}
}
