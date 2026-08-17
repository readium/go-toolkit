package webpub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseFileAsset opens the file at [path] with the given media type and runs it
// through the parser, the same way the Streamer would.
func parseFileAsset(t *testing.T, client *http.Client, path string, mt *mediatype.MediaType) (*pub.Builder, error) {
	t.Helper()
	uri, err := url.FromFilepath(path)
	require.NoError(t, err)
	a := asset.FileWithMediaType(uri, mt)
	f, err := a.CreateFetcher(t.Context(), asset.Dependencies{
		ArchiveFactory: archive.NewArchiveFactory(),
	}, "")
	require.NoError(t, err)
	if client == nil {
		client = http.DefaultClient
	}
	return NewParser(client).Parse(t.Context(), a, f)
}

func readPublicationResource(t *testing.T, p *pub.Publication, href string) ([]byte, error) {
	t.Helper()
	res := p.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString(href, false)})
	defer res.Close()
	data, rerr := res.Read(t.Context(), 0, 0)
	if rerr != nil {
		return nil, rerr
	}
	return data, nil
}

func TestParseBareAudiobookManifest(t *testing.T) {
	builder, err := parseFileAsset(t, nil, "testdata/audio/manifest.json", &mediatype.ReadiumAudiobookManifest)
	require.NoError(t, err)
	require.NotNil(t, builder)

	m := builder.Manifest
	assert.Equal(t, "The Art of Letters", m.Metadata.Title())
	assert.True(t, m.ConformsTo(manifest.ProfileAudiobook))
	assert.Len(t, m.ReadingOrder, 3)

	p := builder.Build()
	defer p.Close()

	// Resources are fetched relative to the manifest's location: a sibling file
	// of the manifest must be reachable through the publication.
	data, err := readPublicationResource(t, p, "manifest_noconform1.json")
	require.NoError(t, err)
	expected, err := os.ReadFile("testdata/audio/manifest_noconform1.json")
	require.NoError(t, err)
	assert.Equal(t, expected, data)

	// The manifest itself is reachable at its own name.
	data, err = readPublicationResource(t, p, "manifest.json")
	require.NoError(t, err)
	var mjson map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &mjson))

	// A missing sibling fails.
	_, err = readPublicationResource(t, p, "missing.json")
	assert.Error(t, err)
}

func TestParseBareManifestConformance(t *testing.T) {
	for _, tt := range []struct {
		name      string
		file      string
		mediaType *mediatype.MediaType
		wantErr   bool
	}{
		{"explicit conformsTo", "manifest.json", &mediatype.ReadiumAudiobookManifest, false},
		{"no conformsTo, schema.org type", "manifest_noconform1.json", &mediatype.ReadiumAudiobookManifest, false},
		{"no conformsTo, no type", "manifest_noconform2.json", &mediatype.ReadiumAudiobookManifest, false},
		{"no conformsTo, cover link only", "manifest_noconform3.json", &mediatype.ReadiumAudiobookManifest, false},
		{"not an audiobook as audiobook", "manifest_notaudiobook.json", &mediatype.ReadiumAudiobookManifest, true},
		{"not an audiobook as generic webpub", "manifest_notaudiobook.json", &mediatype.ReadiumWebpubManifest, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			builder, err := parseFileAsset(t, nil, "testdata/audio/"+tt.file, tt.mediaType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, builder)
			}
		})
	}
}

func TestParseAudiobookPackage(t *testing.T) {
	builder, err := parseFileAsset(t, nil, "testdata/audio/art_letters.audiobook", &mediatype.ReadiumAudiobook)
	require.NoError(t, err)
	require.NotNil(t, builder)

	m := builder.Manifest
	assert.Equal(t, "The Art of Letters", m.Metadata.Title())
	assert.True(t, m.ConformsTo(manifest.ProfileAudiobook))
	require.Len(t, m.ReadingOrder, 3)

	// Resources come from inside the package.
	p := builder.Build()
	defer p.Close()
	res := p.Get(t.Context(), m.ReadingOrder[0])
	defer res.Close()
	length, rerr := res.Length(t.Context())
	require.Nil(t, rerr)
	assert.Greater(t, length, int64(0))
}

func TestParsePDFPackage(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mediaType *mediatype.MediaType
	}{
		{"as webpub", &mediatype.ReadiumWebpub},
		{"as LCP protected PDF", &mediatype.LCPProtectedPDF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			builder, err := parseFileAsset(t, nil, "testdata/pdf/pdf.webpub", tt.mediaType)
			require.NoError(t, err)
			require.NotNil(t, builder)

			m := builder.Manifest
			assert.True(t, m.ConformsTo(manifest.ProfilePDF))
			require.Len(t, m.ReadingOrder, 1)
			assert.NotNil(t, builder.ServicesBuilder.Get(pub.PositionsService_Name))
		})
	}
}

func TestParseLCPDFRequirements(t *testing.T) {
	// An audiobook package parsed as LCPDF must be rejected: the reading order
	// does not contain PDF documents.
	_, err := parseFileAsset(t, nil, "testdata/audio/art_letters.audiobook", &mediatype.LCPProtectedPDF)
	assert.Error(t, err)
}

func TestParseBareManifestOverHTTP(t *testing.T) {
	ts := httptest.NewServer(http.FileServer(http.Dir("testdata/audio")))
	defer ts.Close()

	manifestURL, err := url.AbsoluteURLFromString(ts.URL + "/manifest.json")
	require.NoError(t, err)
	a := asset.HTTPWithMediaType(ts.Client(), manifestURL, &mediatype.ReadiumAudiobookManifest)
	f, err := a.CreateFetcher(t.Context(), asset.Dependencies{}, "")
	require.NoError(t, err)

	builder, err := NewParser(ts.Client()).Parse(t.Context(), a, f)
	require.NoError(t, err)
	require.NotNil(t, builder)
	assert.Equal(t, "The Art of Letters", builder.Manifest.Metadata.Title())

	p := builder.Build()
	defer p.Close()

	// Relative HREFs resolve against the manifest's URL.
	data, err := readPublicationResource(t, p, "manifest_noconform1.json")
	require.NoError(t, err)
	expected, err := os.ReadFile("testdata/audio/manifest_noconform1.json")
	require.NoError(t, err)
	assert.Equal(t, expected, data)
}

func TestParseBareManifestAbsoluteHrefs(t *testing.T) {
	// A local manifest referencing a remote resource with an absolute HTTP HREF:
	// relative HREFs are served from the manifest's directory, absolute HTTP(S)
	// HREFs with the parser's HTTP client.
	remote := []byte("remote audio bytes")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track.opus" {
			http.NotFound(w, r)
			return
		}
		w.Write(remote)
	}))
	defer ts.Close()

	local := []byte("local cover bytes")
	dir := t.TempDir()
	manifestJSON := `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {
			"@type": "http://schema.org/Audiobook",
			"title": "Absolute HREF test"
		},
		"links": [],
		"readingOrder": [
			{"href": "` + ts.URL + `/track.opus", "type": "audio/opus"}
		],
		"resources": [
			{"href": "images/cover.jpg", "type": "image/jpeg", "rel": "cover"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "images"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "images", "cover.jpg"), local, 0o644))

	builder, err := parseFileAsset(t, ts.Client(), filepath.ToSlash(filepath.Join(dir, "manifest.json")), &mediatype.ReadiumAudiobookManifest)
	require.NoError(t, err)
	require.NotNil(t, builder)

	p := builder.Build()
	defer p.Close()

	data, err := readPublicationResource(t, p, ts.URL+"/track.opus")
	require.NoError(t, err)
	assert.Equal(t, remote, data)

	data, err = readPublicationResource(t, p, "images/cover.jpg")
	require.NoError(t, err)
	assert.Equal(t, local, data)
}

// A bare manifest in a subdirectory referencing resources in sibling directories
// through `../` HREFs, a common layout for exploded publications:
//
//	pub/manifest/manifest.json  -> ../audiobook/track.opus, ../cover/cover.jpg
//
// The parser normalizes the HREFs to the publication root (`pub/`), like the
// resources of a package, so that the publication can be served under a single
// base URL: `audiobook/track.opus`, `cover/cover.jpg`, `manifest/manifest.json`.
func TestParseExplodedPublicationLayout(t *testing.T) {
	audio := []byte("audio bytes")
	cover := []byte("cover bytes")
	dir := t.TempDir()
	manifestJSON := `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {
			"@type": "http://schema.org/Audiobook",
			"title": "Exploded layout test"
		},
		"links": [
			{"href": "../cover/cover.jpg", "type": "image/jpeg", "rel": "cover"},
			{"href": "manifest.json", "type": "application/audiobook+json", "rel": "self"}
		],
		"readingOrder": [
			{"href": "../audiobook/track.opus", "type": "audio/opus", "duration": 60}
		],
		"toc": [
			{"href": "#part1", "title": "Part 1"},
			{"href": "?page=2", "title": "Page 2"}
		]
	}`
	for sub, content := range map[string][]byte{
		filepath.Join("manifest", "manifest.json"): []byte(manifestJSON),
		filepath.Join("audiobook", "track.opus"):   audio,
		filepath.Join("cover", "cover.jpg"):        cover,
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, sub)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub), content, 0o644))
	}
	manifestPath := filepath.ToSlash(filepath.Join(dir, "manifest", "manifest.json"))

	for _, tt := range []struct {
		name      string
		mediaType *mediatype.MediaType
	}{
		{"explicit media type", &mediatype.ReadiumAudiobookManifest},
		{"sniffed media type", nil}, // relies on SniffWebpub's heavy sniffing
	} {
		t.Run(tt.name, func(t *testing.T) {
			builder, err := parseFileAsset(t, nil, manifestPath, tt.mediaType)
			require.NoError(t, err)
			require.NotNil(t, builder)

			m := builder.Manifest
			assert.True(t, m.ConformsTo(manifest.ProfileAudiobook))

			// HREFs are normalized to the publication root.
			require.Len(t, m.ReadingOrder, 1)
			assert.Equal(t, "audiobook/track.opus", m.ReadingOrder[0].Href.String())
			require.NotNil(t, m.Links.FirstWithRel("cover"))
			assert.Equal(t, "cover/cover.jpg", m.Links.FirstWithRel("cover").Href.String())
			require.NotNil(t, m.Links.FirstWithRel("self"))
			assert.Equal(t, "manifest/manifest.json", m.Links.FirstWithRel("self").Href.String())

			// Fragment-only and query-only HREFs point within the manifest itself
			// and must not be rewritten into path links.
			require.Len(t, m.TableOfContents, 2)
			assert.Equal(t, "#part1", m.TableOfContents[0].Href.String())
			assert.Equal(t, "?page=2", m.TableOfContents[1].Href.String())

			p := builder.Build()
			defer p.Close()

			// Resources are fetched through the manifest's own links...
			res := p.Get(t.Context(), m.ReadingOrder[0])
			data, rerr := res.Read(t.Context(), 0, 0)
			res.Close()
			require.Nil(t, rerr)
			assert.Equal(t, audio, data)

			// ...and through root-relative HREFs, as a web server would request them.
			data, err = readPublicationResource(t, p, "cover/cover.jpg")
			require.NoError(t, err)
			assert.Equal(t, cover, data)

			data, err = readPublicationResource(t, p, "manifest/manifest.json")
			require.NoError(t, err)
			assert.Equal(t, []byte(manifestJSON), data)
		})
	}

	// The same layout served over HTTP: the manifest URL is in a subdirectory, and the
	// normalized HREFs must resolve against the publication root on the server.
	t.Run("over HTTP", func(t *testing.T) {
		ts := httptest.NewServer(http.FileServer(http.Dir(dir)))
		defer ts.Close()

		manifestURL, err := url.AbsoluteURLFromString(ts.URL + "/manifest/manifest.json")
		require.NoError(t, err)
		a := asset.HTTPWithMediaType(ts.Client(), manifestURL, &mediatype.ReadiumAudiobookManifest)
		f, err := a.CreateFetcher(t.Context(), asset.Dependencies{}, "")
		require.NoError(t, err)

		builder, err := NewParser(ts.Client()).Parse(t.Context(), a, f)
		require.NoError(t, err)
		require.NotNil(t, builder)
		require.Len(t, builder.Manifest.ReadingOrder, 1)
		assert.Equal(t, "audiobook/track.opus", builder.Manifest.ReadingOrder[0].Href.String())

		p := builder.Build()
		defer p.Close()

		data, err := readPublicationResource(t, p, "audiobook/track.opus")
		require.NoError(t, err)
		assert.Equal(t, audio, data)

		data, err = readPublicationResource(t, p, "cover/cover.jpg")
		require.NoError(t, err)
		assert.Equal(t, cover, data)
	})
}

// HREF normalization for an exploded manifest with maxUp>0 must handle the HREFs
// that don't relativize to the publication root, and reach metadata links too.
func TestParseExplodedManifestHrefNormalization(t *testing.T) {
	dir := t.TempDir()
	// `../audiobook/track.opus` forces the publication root one level above the
	// manifest, triggering normalization of every other HREF.
	manifestJSON := `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {
			"@type": "http://schema.org/Audiobook",
			"title": "HREF normalization test",
			"author": [
				{"name": "Author", "links": [{"href": "bio.json", "type": "application/json"}]}
			]
		},
		"links": [
			{"href": "http://other-host.invalid/pub/audio/remote.opus", "type": "audio/opus", "rel": "alternate"},
			{"href": "/site-cover.jpg", "type": "image/jpeg", "rel": "cover"}
		],
		"readingOrder": [
			{"href": "../audiobook/track.opus", "type": "audio/opus", "duration": 60}
		]
	}`
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "manifest"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest", "manifest.json"), []byte(manifestJSON), 0o644))

	builder, err := parseFileAsset(t, nil, filepath.ToSlash(filepath.Join(dir, "manifest", "manifest.json")), &mediatype.ReadiumAudiobookManifest)
	require.NoError(t, err)
	require.NotNil(t, builder)
	m := builder.Manifest

	// A `../` HREF is rebased to the publication root.
	assert.Equal(t, "audiobook/track.opus", m.ReadingOrder[0].Href.String())

	// A cross-origin absolute HREF is left untouched, not stripped to a path under
	// the root (which would make the publication serve the wrong resource).
	require.NotNil(t, m.Links.FirstWithRel("alternate"))
	assert.Equal(t, "http://other-host.invalid/pub/audio/remote.opus", m.Links.FirstWithRel("alternate").Href.String())

	// A root-absolute HREF is left as-is, not mangled into a `file://` URL.
	require.NotNil(t, m.Links.FirstWithRel("cover"))
	assert.Equal(t, "/site-cover.jpg", m.Links.FirstWithRel("cover").Href.String())

	// Metadata links are normalized like the rest: a link sitting next to the
	// manifest is rebased under the manifest's own directory.
	require.Len(t, m.Metadata.Authors, 1)
	require.Len(t, m.Metadata.Authors[0].Links, 1)
	assert.Equal(t, "manifest/bio.json", m.Metadata.Authors[0].Links[0].Href.String())
}

// The fetcher created for a bare manifest is sandboxed to the publication root:
// HREFs handed to the publication later (e.g. from an incoming server request)
// must not reach files outside of it, even with `..` or root-absolute paths.
func TestParseExplodedPublicationSandboxed(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "pub")
	audio := []byte("audio bytes")
	require.NoError(t, os.WriteFile(filepath.Join(base, "secret.txt"), []byte("secret"), 0o644))
	manifestJSON := `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {
			"@type": "http://schema.org/Audiobook",
			"title": "Sandbox test"
		},
		"links": [],
		"readingOrder": [
			{"href": "../audiobook/track.opus", "type": "audio/opus", "duration": 60}
		]
	}`
	for sub, content := range map[string][]byte{
		filepath.Join("manifest", "manifest.json"): []byte(manifestJSON),
		filepath.Join("audiobook", "track.opus"):   audio,
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, sub)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub), content, 0o644))
	}

	builder, err := parseFileAsset(t, nil, filepath.ToSlash(filepath.Join(dir, "manifest", "manifest.json")), &mediatype.ReadiumAudiobookManifest)
	require.NoError(t, err)
	require.NotNil(t, builder)
	p := builder.Build()
	defer p.Close()

	// Resources inside the root are served, with queries and fragments dropped.
	data, err := readPublicationResource(t, p, "audiobook/track.opus?token=abc#t=30")
	require.NoError(t, err)
	assert.Equal(t, audio, data)

	// Escape attempts fail, even though the target file exists.
	for _, href := range []string{
		"../secret.txt",
		"../../secret.txt",
		"audiobook/../../secret.txt",
		"/secret.txt",
	} {
		_, err := readPublicationResource(t, p, href)
		assert.Error(t, err, href)
	}
}

func TestParseServiceFactories(t *testing.T) {
	// An EPUB-profile WebPub gets a positions service and, having HTML contents,
	// a guided navigation service.
	dir := t.TempDir()
	manifestJSON := `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {
			"conformsTo": "https://readium.org/webpub-manifest/profiles/epub",
			"title": "EPUB profile test"
		},
		"links": [],
		"readingOrder": [
			{"href": "chapter1.xhtml", "type": "application/xhtml+xml"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644))

	builder, err := parseFileAsset(t, nil, filepath.ToSlash(filepath.Join(dir, "manifest.json")), &mediatype.ReadiumWebpubManifest)
	require.NoError(t, err)
	require.NotNil(t, builder)

	assert.NotNil(t, builder.ServicesBuilder.Get(pub.PositionsService_Name))
	assert.NotNil(t, builder.ServicesBuilder.Get(pub.GuidedNavigationService_Name))
}

// HTML documents among the resources are enough to get a guided navigation service,
// even when the reading order has none. Without SMIL alternates there is no media
// overlay service.
func TestParseGuidedNavigationForHTMLResources(t *testing.T) {
	dir := t.TempDir()
	manifestJSON := `{
		"@context": "https://readium.org/webpub-manifest/context.jsonld",
		"metadata": {
			"conformsTo": "https://readium.org/webpub-manifest/profiles/audiobook",
			"title": "Audiobook with sync text",
			"duration": 60
		},
		"links": [],
		"readingOrder": [
			{"href": "track1.mp3", "type": "audio/mpeg", "duration": 60}
		],
		"resources": [
			{"href": "sync.xhtml", "type": "application/xhtml+xml"}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644))

	builder, err := parseFileAsset(t, nil, filepath.ToSlash(filepath.Join(dir, "manifest.json")), &mediatype.ReadiumWebpubManifest)
	require.NoError(t, err)
	require.NotNil(t, builder)

	p := builder.Build()
	defer p.Close()
	assert.NotNil(t, p.FindService(pub.GuidedNavigationService_Name))
	assert.Nil(t, p.FindService(pub.MediaOverlayService_Name))
}

// The parser must not swallow assets which aren't WebPub flavored.
func TestParseSkipsOtherMediaTypes(t *testing.T) {
	builder, err := parseFileAsset(t, nil, "testdata/audio/manifest.json", &mediatype.JSON)
	assert.NoError(t, err)
	assert.Nil(t, builder)
}
