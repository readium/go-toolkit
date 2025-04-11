package epub

import (
	"context"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
)

func loadSmil(ctx context.Context, name string) (*manifest.GuidedNavigationDocument, error) {
	n, rerr := fetcher.ReadResourceAsXML(ctx, fetcher.NewFileResource(manifest.Link{}, "./testdata/smil/"+name+".smil"), map[string]string{
		NamespaceOPS:   "epub",
		NamespaceSMIL:  "smil",
		NamespaceSMIL2: "smil2",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseSMILDocument(n, url.MustURLFromString("OEBPS/page1.smil"))
}

func TestSMILDocTypicalAudio(t *testing.T) {
	doc, err := loadSmil(t.Context(), "audio1")
	if !assert.NoError(t, err) {
		return
	}
	assert.Empty(t, doc.Links)
	if assert.Len(t, doc.Guided, 6) {
		assert.Equal(t, "OEBPS/page1.xhtml#word0", doc.Guided[0].TextRef)
		assert.Equal(t, "OEBPS/audio/page1.m4a#t=0,0.84", doc.Guided[0].AudioRef)
	}
}

func TestSMILW3Examples(t *testing.T) {
	// Examples from the EPUB Media Overlay spec from W3
	for _, v := range []string{"w3-2", "w3-3", "w3-4", "w3-8", "w3-10"} {
		_, err := loadSmil(t.Context(), v)
		assert.NoError(t, err)
	}
}

func TestSMILClipBoundaries(t *testing.T) {
	doc, err := loadSmil(t.Context(), "audio-clip")
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Len(t, doc.Guided, 3) {
		return
	}
	assert.Equal(t, "OEBPS/audio/page1.m4a#t=,0.84", doc.Guided[0].AudioRef)
	assert.Equal(t, "OEBPS/audio/page1.m4a#t=0.84", doc.Guided[1].AudioRef)
	assert.Equal(t, "OEBPS/audio/page1.m4a", doc.Guided[2].AudioRef)
}
