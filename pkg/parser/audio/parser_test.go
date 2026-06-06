package audio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single-file M4B audiobook with embedded metadata, a cover and a chapter
// (text) track providing 26 chapters.
const m4bPath = "./testdata/AroundTheWorldInEightyDays.m4b"

const (
	// A ZAB (Zipped Audio Book): a ZIP whose entries all live under a single
	// "Art of Letters/" root directory containing three .opus tracks.
	zabPath = "./testdata/art_letters.zab"
	// The same audiobook exploded into a directory of .opus tracks, with no
	// enclosing root folder.
	explodedDirPath = "./testdata/art_letters"
	// A single track, used to exercise the standalone-audio-file path.
	standaloneFilePath = "./testdata/art_letters/artofletters_00_lynd.opus"
)

// parseAudio builds a fetcher for the asset at path and runs the AudioParser,
// returning its raw result so that rejection and error paths can be asserted.
func parseAudio(t *testing.T, path string) (*pub.Builder, error) {
	t.Helper()
	u, err := url.FromFilepath(path)
	require.NoError(t, err)
	a := asset.File(u)
	fet, err := a.CreateFetcher(t.Context(), asset.Dependencies{
		ArchiveFactory: archive.NewArchiveFactory(),
	}, "")
	require.NoError(t, err)
	t.Cleanup(fet.Close)
	return AudioParser{}.Parse(t.Context(), a, fet)
}

// withAudioParser parses the asset at path with the AudioParser and passes the
// resulting builder to f, failing the test if the asset is rejected or errors.
func withAudioParser(t *testing.T, path string, f func(*pub.Builder)) {
	t.Helper()
	b, err := parseAudio(t, path)
	require.NoError(t, err)
	require.NotNil(t, b, "parser unexpectedly rejected the asset")
	f(b)
}

// readingOrderFilenames returns the decoded final path segment of every reading
// order link. This is robust to whether the href carries an archive or
// directory prefix.
func readingOrderFilenames(ro manifest.LinkList) []string {
	names := make([]string, 0, len(ro))
	for _, link := range ro {
		names = append(names, link.URL(nil, nil).Filename())
	}
	return names
}

func TestNewParser(t *testing.T) {
	assert.Equal(t, AudioParser{}, NewParser())
}

func TestNewRichParser(t *testing.T) {
	assert.Equal(t, AudioParser{rich: true}, NewRichParser())
	assert.Equal(t, AudioParser{rich: true, skipEmbeddedChapters: true}, NewRichParser(WithoutEmbeddedChapters()))
	assert.Equal(t, AudioParser{rich: true, cacheBlockSize: 512 << 10}, NewRichParser(WithCacheBlockSize(512<<10)))
	assert.Equal(t, AudioParser{rich: true}, NewRichParser(WithCacheBlockSize(0)), "non-positive cache size is ignored")
	assert.Equal(t, AudioParser{rich: true, concurrency: 4}, NewRichParser(WithConcurrency(4)))
	assert.Equal(t, AudioParser{rich: true}, NewRichParser(WithConcurrency(0)), "non-positive concurrency is ignored")
}

// Custom block size and sequential concurrency still produce a correct manifest.
func TestAudioRichWithOptions(t *testing.T) {
	u, err := url.FromFilepath(zabPath)
	require.NoError(t, err)
	a := asset.File(u)
	fet, err := a.CreateFetcher(t.Context(), asset.Dependencies{ArchiveFactory: archive.NewArchiveFactory()}, "")
	require.NoError(t, err)
	t.Cleanup(fet.Close)

	b, err := NewRichParser(WithConcurrency(1), WithCacheBlockSize(64<<10)).Parse(t.Context(), a, fet)
	require.NoError(t, err)
	require.NotNil(t, b)
	m := b.Build().Manifest
	require.Len(t, m.ReadingOrder, 3)
	for _, l := range m.ReadingOrder {
		assert.Greater(t, l.Duration, 0.0)
		assert.Greater(t, l.Bitrate, 0.0)
	}
	assert.Equal(t, "The Art of Letters", m.Metadata.Title())
}

// WithoutEmbeddedChapters skips building the TOC from the M4B chapter track,
// while still extracting durations and metadata.
func TestAudioRichWithoutEmbeddedChapters(t *testing.T) {
	if _, err := os.Stat(m4bPath); err != nil {
		t.Skipf("M4B fixture not available: %v", err)
	}
	m := parseRichAudioWith(t, m4bPath, WithoutEmbeddedChapters()).Build().Manifest

	require.Len(t, m.ReadingOrder, 1)
	assert.Greater(t, m.ReadingOrder[0].Duration, 0.0, "duration is still probed")
	assert.Equal(t, "Around the World in Eighty Days", m.Metadata.Title(), "metadata is still extracted")
	assert.Empty(t, m.TableOfContents, "no TOC is built from embedded chapters when disabled")
}

// parseRichAudio runs the rich AudioParser against the asset at path.
func parseRichAudio(t *testing.T, path string) *pub.Builder {
	return parseRichAudioWith(t, path)
}

// parseRichAudioWith runs the rich AudioParser with the given options.
func parseRichAudioWith(t *testing.T, path string, opts ...Option) *pub.Builder {
	t.Helper()
	u, err := url.FromFilepath(path)
	require.NoError(t, err)
	a := asset.File(u)
	fet, err := a.CreateFetcher(t.Context(), asset.Dependencies{
		ArchiveFactory: archive.NewArchiveFactory(),
	}, "")
	require.NoError(t, err)
	t.Cleanup(fet.Close)
	b, err := NewRichParser(opts...).Parse(t.Context(), a, fet)
	require.NoError(t, err)
	require.NotNil(t, b, "rich parser unexpectedly rejected the asset")
	return b
}

func tocTitles(toc manifest.LinkList) []string {
	titles := make([]string, 0, len(toc))
	for _, l := range toc {
		titles = append(titles, l.Title)
	}
	return titles
}

// Rich parsing of an Opus ZAB sets per-track durations/bitrates, the total
// duration, and pulls the title/author from the first file's Vorbis comments.
func TestAudioRichOpusZAB(t *testing.T) {
	m := parseRichAudio(t, zabPath).Build().Manifest
	require.Len(t, m.ReadingOrder, 3)

	var sum float64
	for _, l := range m.ReadingOrder {
		assert.Greater(t, l.Duration, 0.0, "every track must have a duration")
		assert.Greater(t, l.Bitrate, 0.0, "every track must have a bitrate")
		assert.Empty(t, l.URL(nil, nil).Fragment(), "reading order must not carry fragments")
		sum += l.Duration
	}

	// First track is ~62s ("00 - Dedication").
	assert.InDelta(t, 62.0, m.ReadingOrder[0].Duration, 1.0)

	require.NotNil(t, m.Metadata.Duration)
	assert.InDelta(t, sum, *m.Metadata.Duration, 0.01, "metadata duration is the sum of the tracks")

	// Title comes from the album tag, author from the artist tag.
	assert.Equal(t, "The Art of Letters", m.Metadata.Title())
	require.Len(t, m.Metadata.Authors, 1)
	assert.Equal(t, "Robert Lynd", m.Metadata.Authors[0].Name())
	assert.Equal(t, "http://schema.org/Audiobook", m.Metadata.Type)

	// No playlist or embedded chapters, so the TOC lists the per-file titles.
	require.Len(t, m.TableOfContents, 3)
	assert.Equal(t, "00 - Dedication", m.TableOfContents[0].Title)
}

// Rich parsing of an M4B reads the movie duration, the iTunes metadata + cover,
// and the chapter track to build the table of contents.
func TestAudioRichM4B(t *testing.T) {
	if _, err := os.Stat(m4bPath); err != nil {
		t.Skipf("M4B fixture not available: %v", err)
	}

	b := parseRichAudio(t, m4bPath)
	p := b.Build()
	m := p.Manifest

	require.Len(t, m.ReadingOrder, 1)
	assert.InDelta(t, 23610.91, m.ReadingOrder[0].Duration, 2.0)
	assert.Greater(t, m.ReadingOrder[0].Bitrate, 0.0)
	require.NotNil(t, m.Metadata.Duration)
	assert.InDelta(t, 23610.91, *m.Metadata.Duration, 2.0)

	assert.Equal(t, "Around the World in Eighty Days", m.Metadata.Title())
	require.Len(t, m.Metadata.Authors, 1)
	assert.Equal(t, "Jules Verne", m.Metadata.Authors[0].Name())

	// Cover extracted from the `covr` atom and exposed via the cover service.
	cover := m.Links.FirstWithRel("cover")
	require.NotNil(t, cover, "a cover link should be present")
	assert.Greater(t, cover.Width, uint(0))
	assert.Greater(t, cover.Height, uint(0))
	data, rerr := p.Get(t.Context(), *cover).Read(t.Context(), 0, 0)
	require.Nil(t, rerr)
	assert.NotEmpty(t, data, "the cover resource should serve bytes")

	// 26 chapters from the text track; the first starts at the very beginning.
	require.Len(t, m.TableOfContents, 26)
	assert.Equal(t, "01 - Chapters 01 - 03", m.TableOfContents[0].Title)
	assert.Empty(t, m.TableOfContents[0].URL(nil, nil).Fragment(), "first chapter starts at 0")
	assert.NotEmpty(t, m.TableOfContents[1].URL(nil, nil).Fragment(), "later chapters carry a time fragment")
	assert.True(t, strings.HasPrefix(m.TableOfContents[1].URL(nil, nil).Fragment(), "t="),
		"chapter fragments use media-fragment time syntax")
}

// A playlist file takes precedence over both embedded chapters and the per-file
// fallback when building the table of contents.
func TestAudioRichPlaylistTOCPreferred(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"artofletters_00_lynd.opus",
		"artofletters_01_lynd.opus",
		"artofletters_02_lynd.opus",
	} {
		src, err := os.ReadFile(filepath.Join(explodedDirPath, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), src, 0o644))
	}
	playlist := strings.Join([]string{
		"#EXTM3U",
		"#EXTINF:62,Chapter One",
		"artofletters_00_lynd.opus",
		"#EXTINF:1050,Chapter Two",
		"artofletters_01_lynd.opus",
		"#EXTINF:789,Chapter Three",
		"artofletters_02_lynd.opus",
		"",
	}, "\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "playlist.m3u"), []byte(playlist), 0o644))

	m := parseRichAudio(t, dir).Build().Manifest
	require.Len(t, m.ReadingOrder, 3, "playlist file is not part of the reading order")
	require.Len(t, m.TableOfContents, 3)
	assert.Equal(t, []string{"Chapter One", "Chapter Two", "Chapter Three"}, tocTitles(m.TableOfContents),
		"the TOC should come from the playlist, not the per-file titles")
}

// A CUE sheet splits a single audio file into tracks, producing a TOC with
// media-fragment time offsets.
func TestAudioRichCUETOC(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(explodedDirPath, "artofletters_01_lynd.opus"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "book.opus"), src, 0o644))

	cue := strings.Join([]string{
		`TITLE "The Art of Letters"`,
		`PERFORMER "Robert Lynd"`,
		`FILE "book.opus" WAVE`,
		`  TRACK 01 AUDIO`,
		`    TITLE "Opening"`,
		`    INDEX 01 00:00:00`,
		`  TRACK 02 AUDIO`,
		`    TITLE "Halfway"`,
		`    INDEX 01 05:00:00`,
		"",
	}, "\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "book.cue"), []byte(cue), 0o644))

	m := parseRichAudio(t, dir).Build().Manifest
	require.Len(t, m.ReadingOrder, 1)
	require.Len(t, m.TableOfContents, 2)
	assert.Equal(t, []string{"Opening", "Halfway"}, tocTitles(m.TableOfContents))
	assert.Empty(t, m.TableOfContents[0].URL(nil, nil).Fragment(), "first track starts at 0")
	assert.Equal(t, "t=300", m.TableOfContents[1].URL(nil, nil).Fragment(),
		"the second track points into the file at its INDEX time")
}

func TestAudioZABAccepted(t *testing.T) {
	withAudioParser(t, zabPath, func(b *pub.Builder) {
		assert.NotNil(t, b)
	})
}

func TestAudioExplodedDirectoryAccepted(t *testing.T) {
	withAudioParser(t, explodedDirPath, func(b *pub.Builder) {
		assert.NotNil(t, b)
	})
}

func TestAudioStandaloneFileAccepted(t *testing.T) {
	withAudioParser(t, standaloneFilePath, func(b *pub.Builder) {
		assert.NotNil(t, b)
	})
}

func TestAudioConformsToAudiobookProfile(t *testing.T) {
	withAudioParser(t, zabPath, func(b *pub.Builder) {
		m := b.Build().Manifest
		assert.Equal(t, manifest.Profiles{manifest.ProfileAudiobook}, m.Metadata.ConformsTo)
	})
}

func TestAudioManifestContext(t *testing.T) {
	withAudioParser(t, zabPath, func(b *pub.Builder) {
		m := b.Build().Manifest
		assert.Equal(t, manifest.Strings{manifest.WebpubManifestContext}, m.Context)
	})
}

func TestAudioZABReadingOrderSortedAlphabetically(t *testing.T) {
	withAudioParser(t, zabPath, func(b *pub.Builder) {
		ro := b.Build().Manifest.ReadingOrder
		require.Len(t, ro, 3, "every audio track should be in the reading order")

		// Relativize against the archive root to assert the exact, ordered hrefs.
		base, _ := url.URLFromDecodedPath("Art of Letters/")
		hrefs := make([]string, 0, len(ro))
		for _, link := range ro {
			hrefs = append(hrefs, base.Relativize(link.URL(nil, nil)).String())
		}
		assert.Exactly(t, []string{
			"artofletters_00_lynd.opus",
			"artofletters_01_lynd.opus",
			"artofletters_02_lynd.opus",
		}, hrefs, "reading order should be sorted alphabetically")
	})
}

func TestAudioExplodedDirectoryReadingOrder(t *testing.T) {
	withAudioParser(t, explodedDirPath, func(b *pub.Builder) {
		ro := b.Build().Manifest.ReadingOrder
		require.Len(t, ro, 3)
		assert.Exactly(t, []string{
			"artofletters_00_lynd.opus",
			"artofletters_01_lynd.opus",
			"artofletters_02_lynd.opus",
		}, readingOrderFilenames(ro), "reading order should be sorted alphabetically")
	})
}

// Hrefs for files in an exploded directory must be relative (no leading "/").
func TestAudioExplodedDirectoryRelativeHrefs(t *testing.T) {
	m := parseRichAudio(t, explodedDirPath).Build().Manifest
	require.Len(t, m.ReadingOrder, 3)
	for _, l := range m.ReadingOrder {
		assert.False(t, strings.HasPrefix(l.Href.String(), "/"),
			"reading order href %q should not be absolute", l.Href.String())
	}
	require.NotEmpty(t, m.TableOfContents)
	for _, l := range m.TableOfContents {
		assert.False(t, strings.HasPrefix(l.Href.String(), "/"),
			"toc href %q should not be absolute", l.Href.String())
	}
	assert.Equal(t, "artofletters_00_lynd.opus", m.ReadingOrder[0].Href.String())
}

func TestAudioStandaloneFile(t *testing.T) {
	withAudioParser(t, standaloneFilePath, func(b *pub.Builder) {
		m := b.Build().Manifest
		require.Len(t, m.ReadingOrder, 1)
		assert.Equal(t, "artofletters_00_lynd.opus", m.ReadingOrder[0].URL(nil, nil).Filename())
		assert.Equal(t, manifest.Profiles{manifest.ProfileAudiobook}, m.Metadata.ConformsTo)
		// No archive or parent folder to infer a title from, so it falls back to
		// the asset's filename.
		assert.Equal(t, "artofletters_00_lynd.opus", m.Metadata.Title())
	})
}

func TestAudioTitleFromArchiveRootDirectory(t *testing.T) {
	withAudioParser(t, zabPath, func(b *pub.Builder) {
		assert.Equal(t, "Art of Letters", b.Build().Manifest.Metadata.Title(),
			"title should be inferred from the archive's common root directory")
	})
}

func TestAudioTitleFallsBackToFilename(t *testing.T) {
	withAudioParser(t, explodedDirPath, func(b *pub.Builder) {
		// The tracks share no common parent folder inside the fetcher, so the
		// title falls back to the asset's name (the directory name).
		assert.Equal(t, "art_letters", b.Build().Manifest.Metadata.Title())
	})
}

func TestAudioReadingOrderHasNoCover(t *testing.T) {
	withAudioParser(t, zabPath, func(b *pub.Builder) {
		// Unlike the image parser, the audio parser does not promote the first
		// resource to a cover.
		assert.Nil(t, b.Build().Manifest.ReadingOrder.FirstWithRel("cover"))
	})
}

func TestAudioRejectsNonAudioArchive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "book.pdf"), []byte("%PDF-1.7\n"), 0o644))

	b, err := parseAudio(t, dir)
	assert.NoError(t, err)
	assert.Nil(t, b, "an asset containing non-audio files should be rejected")
}

func TestAudioErrorsWhenNoAudioFile(t *testing.T) {
	dir := t.TempDir()
	// ".txt" is an accepted "extra" extension (e.g. liner notes) but is not an
	// actual audio track, so the parser accepts the asset yet finds nothing to
	// add to the reading order.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "liner-notes.txt"), []byte("Robert Lynd"), 0o644))

	b, err := parseAudio(t, dir)
	assert.Nil(t, b)
	require.Error(t, err)
	assert.EqualError(t, err, "no audio file found in the publication")
}
