package epub

import (
	"context"
	"errors"
	"testing"

	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/protection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEPUBAsset is a minimal [asset.PublicationAsset] for parser tests.
// CreateFetcher is never invoked because the tests construct the fetcher
// themselves and pass it to [Parser.Parse] directly.
type fakeEPUBAsset struct{ name string }

func (a fakeEPUBAsset) Name() string                                 { return a.name }
func (a fakeEPUBAsset) MediaType(context.Context) mediatype.MediaType { return mediatype.EPUB }
func (a fakeEPUBAsset) CreateFetcher(context.Context, asset.Dependencies, string) (fetcher.Fetcher, error) {
	return nil, errors.New("unused in tests")
}

func openProtectionFixture(t *testing.T, file string) *fetcher.ArchiveFetcher {
	t.Helper()
	f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), "../../protection/testdata/"+file)
	require.NoError(t, err)
	t.Cleanup(f.Close)
	return f
}

// TestParserEndToEnd exercises [Parser.Parse] against every DRM fixture in
// pkg/protection/testdata, asserting the parse succeeds and surfaces the
// expected publication metadata. expectedScheme is the manifest.Encryption
// scheme URI that every encrypted resource should carry — empty means either
// no encryption.xml is present (so no resource should be tagged) or the
// encryption is generic (no DRM scheme attached).
func TestParserEndToEnd(t *testing.T) {
	for _, tt := range []struct {
		name             string
		file             string
		title            string
		readingOrderSize int
		hasEncryptionXML bool
		expectedScheme   string
	}{
		{"Adobe ADEPT", "fake-adept.epub", "Fake Adept DRM", 1, false, ""},
		{"Barnes & Noble", "fake-bn.epub", "Fake B&N DRM", 1, false, ""},
		{"Apple FairPlay", "fake-fairplay.epub", "Fake Fairplay DRM", 1, false, ""},
		{"Kobo", "fake-kobo.epub", "Fake Kobo DRM", 1, false, ""},
		{"Readium LCP", "fake-lcp.epub", "The Level 999 Villager　Chapter 3", 35, true, protection.SchemeLCP},
		{"Generic encryption (Yahoo)", "yahoo.ypub", "週刊少年マガジン 2019年8号[2019年1月23日発売]", 540, true, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := openProtectionFixture(t, tt.file)
			builder, err := NewParser(nil).Parse(t.Context(), fakeEPUBAsset{name: tt.file}, f)
			require.NoError(t, err)
			require.NotNil(t, builder)

			m := builder.Manifest
			assert.Equal(t, tt.title, m.Metadata.Title())
			assert.Lenf(t, m.ReadingOrder, tt.readingOrderSize,
				"expected reading order size %d, got %d", tt.readingOrderSize, len(m.ReadingOrder))

			encryptedCount := 0
			for _, link := range append(m.ReadingOrder, m.Resources...) {
				enc, ok := link.Properties["encrypted"].(map[string]interface{})
				if !ok {
					continue
				}
				encryptedCount++
				if tt.expectedScheme == "" {
					_, hasScheme := enc["scheme"]
					assert.Falsef(t, hasScheme, "%s should not have a scheme", link.Href.String())
				} else {
					assert.Equalf(t, tt.expectedScheme, enc["scheme"], "scheme mismatch for %s", link.Href.String())
				}
			}
			if tt.hasEncryptionXML {
				assert.Greater(t, encryptedCount, 0, "expected at least one resource to carry encryption properties")
			} else {
				assert.Zero(t, encryptedCount, "no resource should carry encryption properties")
			}
		})
	}
}

// TestParseEncryptionDataScheme exercises the same two-step the Parser uses:
// [protection.IdentifyEPUBProtection] followed by parseEncryptionData, and
// verifies the detected scheme reaches every encryption.xml entry.
func TestParseEncryptionDataScheme(t *testing.T) {
	for _, tt := range []struct {
		name          string
		file          string
		expectScheme  string
		expectEntries bool
	}{
		{"Readium LCP", "fake-lcp.epub", protection.SchemeLCP, true},
		{"Generic encryption (Yahoo)", "yahoo.ypub", "", true},
		// EPUBs without META-INF/encryption.xml yield no entries at all.
		{"Adobe ADEPT (no encryption.xml)", "fake-adept.epub", "", false},
		{"Kobo (no encryption.xml)", "fake-kobo.epub", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := openProtectionFixture(t, tt.file)

			scheme, err := protection.IdentifyEPUBProtection(t.Context(), f)
			require.NoError(t, err)

			enc, err := parseEncryptionData(t.Context(), f, scheme.URI())
			require.NoError(t, err)
			if !tt.expectEntries {
				assert.Empty(t, enc)
				return
			}
			require.NotEmpty(t, enc)
			for u, e := range enc {
				assert.Equalf(t, tt.expectScheme, e.Scheme, "scheme mismatch for %s", u)
			}
		})
	}
}
