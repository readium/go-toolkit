package epub

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

func loadSmil(name string) (*manifest.GuidedNavigationDocument, error) {
	n, rerr := fetcher.NewFileResource(manifest.Link{}, "./testdata/smil/"+name+".smil").ReadAsXML(map[string]string{
		NamespaceOPS:   "epub",
		NamespaceSMIL:  "smil",
		NamespaceSMIL2: "smil2",
	})
	if rerr != nil {
		return nil, rerr.Cause
	}

	return ParseSMILDocument(n, "OEBPS/page1.smil")
}

func TestSMILDocTypicalAudio(t *testing.T) {
	doc, err := loadSmil("audio1")
	if !assert.NoError(t, err) {
		return
	}
	assert.Empty(t, doc.Links)
	if assert.Len(t, doc.Guided, 6) {
		assert.Equal(t, "OEBPS/page1.xhtml#word0", doc.Guided[0].TextRef)
		assert.Equal(t, "OEBPS/audio/page1.m4a#t=0,0.84", doc.Guided[0].AudioRef)
	}
}

// TODO test more documents, especially atypical
// For example, missing either clipBegin or clipEnd
// Or ones with <seq> elements
