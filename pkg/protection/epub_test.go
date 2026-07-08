package protection

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentifyEPUBProtection(t *testing.T) {
	for _, tt := range []struct {
		name string
		file string
		want Scheme
	}{
		{"Readium LCP", "fake-lcp.epub", LCP},
		{"Adobe ADEPT", "fake-adept.epub", Adept},
		{"Barnes & Noble", "fake-bn.epub", BarnesAndNoble},
		{"Apple FairPlay", "fake-fairplay.epub", Fairplay},
		{"Kobo", "fake-kobo.epub", Kobo},
		{"Generic encryption", "yahoo.ypub", Generic},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), "./testdata/"+tt.file)
			require.NoError(t, err)
			defer f.Close()

			got, _, err := IdentifyEPUBProtection(t.Context(), f)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "unexpected scheme for %s", tt.file)
		})
	}
}

func TestIdentifyEPUBProtectionNoDRM(t *testing.T) {
	got, _, err := IdentifyEPUBProtection(t.Context(), fetcher.EmptyFetcher{})
	require.NoError(t, err)
	assert.Equal(t, NoDRM, got)
}
