package audio

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseM3U(t *testing.T) {
	content := []byte("#EXTM3U\n" +
		"#EXTINF:62,Chapter One\r\n" +
		"track01.opus\n" +
		"# a stray comment\n" +
		"#EXTINF:120,Chapter Two\n" +
		"sub/track02.opus\n")
	entries := parseM3U(content)
	require.Len(t, entries, 2)
	assert.Equal(t, playlistEntry{Path: "track01.opus", Title: "Chapter One"}, entries[0])
	assert.Equal(t, playlistEntry{Path: "sub/track02.opus", Title: "Chapter Two"}, entries[1])
}

func TestParsePLS(t *testing.T) {
	content := []byte("[playlist]\n" +
		"File1=track01.mp3\nTitle1=Intro\nLength1=30\n" +
		"File2=track02.mp3\nTitle2=Outro\nLength2=45\n" +
		"NumberOfEntries=2\n")
	entries := parsePLS(content)
	require.Len(t, entries, 2)
	assert.Equal(t, playlistEntry{Path: "track01.mp3", Title: "Intro"}, entries[0])
	assert.Equal(t, playlistEntry{Path: "track02.mp3", Title: "Outro"}, entries[1])
}

func TestParseXSPF(t *testing.T) {
	content := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<playlist version="1" xmlns="http://xspf.org/ns/0/">
  <trackList>
    <track><location>track01.flac</location><title>One</title></track>
    <track><location>track02.flac</location><title>Two</title></track>
  </trackList>
</playlist>`)
	entries := parseXSPF(content)
	require.Len(t, entries, 2)
	assert.Equal(t, playlistEntry{Path: "track01.flac", Title: "One"}, entries[0])
	assert.Equal(t, playlistEntry{Path: "track02.flac", Title: "Two"}, entries[1])
}

// Real-world fixtures: a 13-FILE CUE sheet and the matching M3U playlist.
func TestParseCUEFixture(t *testing.T) {
	content, err := os.ReadFile("./testdata/luvsic.cue")
	require.NoError(t, err)
	entries := parseCUE(content)
	require.Len(t, entries, 13, "one entry per FILE/TRACK pair")
	assert.Equal(t, playlistEntry{Path: "01 - Luv(sic).wav", Title: "Luv(sic)", Start: 0}, entries[0])
	// Quoted filename containing doubled single-quotes is preserved verbatim.
	assert.Equal(t, "07 - Luv(sic) 12'' Remix.wav", entries[6].Path)
	assert.Equal(t, "Luv(sic) 12' Remix", entries[6].Title)
	assert.Equal(t, playlistEntry{Path: "13 - Perfect Circle.wav", Title: "Perfect Circle", Start: 0}, entries[12])
}

func TestParseM3UFixture(t *testing.T) {
	content, err := os.ReadFile("./testdata/luvsic.m3u")
	require.NoError(t, err)
	entries := parseM3U(content)
	require.Len(t, entries, 13)
	assert.Equal(t, playlistEntry{Path: "01 - Luv(sic).flac", Title: "Nujabes feat. Shing02 - Luv(sic)"}, entries[0])
	assert.Equal(t, "07 - Luv(sic) 12'' Remix.flac", entries[6].Path)
	assert.Equal(t, `Nujabes feat. Shing02 - Luv(sic) 12" Remix`, entries[6].Title)
	assert.Equal(t, "13 - Perfect Circle.flac", entries[12].Path)
}

func TestParseCUE(t *testing.T) {
	content := []byte(`REM GENRE Audiobook
PERFORMER "Robert Lynd"
TITLE "The Art of Letters"
FILE "art_letters.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Dedication"
    PERFORMER "Robert Lynd"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Mr. Pepys"
    INDEX 00 17:28:00
    INDEX 01 17:30:00
`)
	entries := parseCUE(content)
	require.Len(t, entries, 2)
	// Album-level TITLE is ignored; per-track INDEX 01 (not 00) sets the start.
	assert.Equal(t, playlistEntry{Path: "art_letters.flac", Title: "Dedication", Start: 0}, entries[0])
	assert.Equal(t, playlistEntry{Path: "art_letters.flac", Title: "Mr. Pepys", Start: 1050}, entries[1])
}

func TestParseCUEMultipleFiles(t *testing.T) {
	content := []byte("FILE \"a.wav\" WAVE\n  TRACK 01 AUDIO\n    TITLE \"A\"\n    INDEX 01 00:00:00\n" +
		"FILE \"b.wav\" WAVE\n  TRACK 02 AUDIO\n    TITLE \"B\"\n    INDEX 01 00:00:00\n")
	entries := parseCUE(content)
	require.Len(t, entries, 2)
	assert.Equal(t, "a.wav", entries[0].Path)
	assert.Equal(t, "b.wav", entries[1].Path)
}

func TestParseCUETime(t *testing.T) {
	v, ok := parseCUETime("01:30:37") // 1m 30s 37 frames (75/s)
	require.True(t, ok)
	assert.InDelta(t, 90+37.0/75, v, 0.0001)

	_, ok = parseCUETime("bad")
	assert.False(t, ok)
}

func TestParsePlaylistUnsupported(t *testing.T) {
	assert.Nil(t, parsePlaylist("txt", []byte("not a playlist")))
	assert.Nil(t, parsePlaylist("log", []byte("rip log, not a playlist")))
}

func TestPlaylistEntryName(t *testing.T) {
	assert.Equal(t, "track 01.opus", playlistEntryName("audio/track%2001.opus?x=1"))
	assert.Equal(t, "track.mp3", playlistEntryName("C:\\music\\track.mp3"))
}

func TestFormatFragmentTime(t *testing.T) {
	assert.Equal(t, "0", formatFragmentTime(0))
	assert.Equal(t, "1647.2", formatFragmentTime(1647.2))
	assert.Equal(t, "62.011", formatFragmentTime(62.0109))
}

func TestParseTimecode(t *testing.T) {
	v, ok := parseTimecode("01:02:03.500")
	require.True(t, ok)
	assert.InDelta(t, 3723.5, v, 0.001)

	_, ok = parseTimecode("bad")
	assert.False(t, ok)
}

func TestVorbisChapters(t *testing.T) {
	tags := &audioTags{Raw: map[string]interface{}{
		"CHAPTER000":     "00:00:00.000",
		"CHAPTER000NAME": "Opening",
		"CHAPTER001":     "00:01:30.000",
		"CHAPTER001NAME": "Middle",
		"title":          "ignored",
	}}
	chapters := vorbisChapters(tags)
	require.Len(t, chapters, 2)
	assert.Equal(t, chapterEntry{Title: "Opening", Start: 0}, chapters[0])
	assert.Equal(t, chapterEntry{Title: "Middle", Start: 90}, chapters[1])
}

func TestDecodeExtendedFloat(t *testing.T) {
	// 80-bit IEEE extended representation of 44100.0
	b := []byte{0x40, 0x0E, 0xAC, 0x44, 0, 0, 0, 0, 0, 0}
	assert.InDelta(t, 44100.0, decodeExtendedFloat(b), 0.001)
}

func TestEBMLReadSize(t *testing.T) {
	// 0x81 -> length 1, value 1
	v, n := ebmlReadSize([]byte{0x81})
	assert.Equal(t, 1, n)
	assert.Equal(t, int64(1), v)

	// 0x4002 -> length 2, value 2
	v, n = ebmlReadSize([]byte{0x40, 0x02})
	assert.Equal(t, 2, n)
	assert.Equal(t, int64(2), v)
}

func TestEBMLReadID(t *testing.T) {
	id, n := ebmlReadID([]byte{0x2A, 0xD7, 0xB1})
	assert.Equal(t, 3, n)
	assert.Equal(t, uint64(ebmlTimecodeScale), id)
}

func TestMP3SamplesPerFrame(t *testing.T) {
	assert.Equal(t, 1152, mp3SamplesPerFrame(3, 1)) // MPEG1 Layer III
	assert.Equal(t, 576, mp3SamplesPerFrame(2, 1))  // MPEG2 Layer III
	assert.Equal(t, 384, mp3SamplesPerFrame(3, 3))  // Layer I
}
