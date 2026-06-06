package audio

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/readium/go-toolkit/pkg/manifest"
)

// chapterEntry is a single chapter extracted from an audio file: a title and a
// start offset (in seconds) within that file.
type chapterEntry struct {
	Title string
	Start float64
}

// chaptersToLinks turns the chapters found within a single audio resource into
// table-of-contents links pointing into that resource using media-fragment time
// offsets (e.g. `track.m4b#t=123.4`).
func chaptersToLinks(entries []chapterEntry, base manifest.Link) manifest.LinkList {
	links := make(manifest.LinkList, 0, len(entries))
	for _, e := range entries {
		links = append(links, chapterLink(base, e.Title, e.Start))
	}
	return links
}

// chapterLink builds a TOC link into base at the given start offset. A negative
// or zero offset produces a link to the whole resource.
func chapterLink(base manifest.Link, title string, start float64) manifest.Link {
	href := base.URL(nil, nil).String()
	if start > 0 {
		href += "#t=" + formatFragmentTime(start)
	}
	return manifest.Link{
		Href:      manifest.MustNewHREFFromString(href, false),
		MediaType: base.MediaType,
		Title:     title,
	}
}

// formatFragmentTime formats a number of seconds for a media fragment, rounded
// to the millisecond and without trailing zeros.
func formatFragmentTime(seconds float64) string {
	rounded := math.Round(seconds*1000) / 1000
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

var vorbisChapterKey = regexp.MustCompile(`^chapter(\d+)$`)

// vorbisChapters extracts chapters declared via the Vorbis comment chapter
// extension (CHAPTERxxx / CHAPTERxxxNAME), used by Ogg and FLAC files.
func vorbisChapters(tags *audioTags) []chapterEntry {
	if tags == nil || tags.Raw == nil {
		return nil
	}

	type idxChapter struct {
		index int
		entry chapterEntry
	}
	var found []idxChapter
	for key, value := range tags.Raw {
		m := vorbisChapterKey.FindStringSubmatch(strings.ToLower(key))
		if m == nil {
			continue
		}
		timecode, ok := value.(string)
		if !ok {
			continue
		}
		start, ok := parseTimecode(timecode)
		if !ok {
			continue
		}
		index, _ := strconv.Atoi(m[1])
		name, _ := tags.Raw["CHAPTER"+m[1]+"NAME"].(string)
		if name == "" {
			name, _ = tags.Raw["chapter"+m[1]+"name"].(string)
		}
		found = append(found, idxChapter{index: index, entry: chapterEntry{Title: strings.TrimSpace(name), Start: start}})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].index < found[j].index })
	entries := make([]chapterEntry, 0, len(found))
	for _, c := range found {
		entries = append(entries, c.entry)
	}
	return entries
}

// parseTimecode parses a "HH:MM:SS.mmm" timecode into seconds.
func parseTimecode(s string) (float64, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 3 {
		return 0, false
	}
	h, err1 := strconv.ParseFloat(parts[0], 64)
	m, err2 := strconv.ParseFloat(parts[1], 64)
	sec, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return h*3600 + m*60 + sec, true
}
