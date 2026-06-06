package audio

import (
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
)

// playlistEntry is a single reference in a playlist file: a resource path, an
// optional human-readable title and an optional start offset within the
// resource (in seconds), used by formats such as CUE sheets that split a single
// audio file into tracks.
type playlistEntry struct {
	Path  string
	Title string
	Start float64
}

// parsePlaylist parses the supported playlist formats (M3U/M3U8, PLS, XSPF, CUE)
// into an ordered list of entries. It returns nil for formats it cannot parse,
// in which case callers fall back to other table-of-contents sources.
func parsePlaylist(ext string, content []byte) []playlistEntry {
	switch strings.ToLower(ext) {
	case "m3u", "m3u8":
		return parseM3U(content)
	case "pls":
		return parsePLS(content)
	case "xspf":
		return parseXSPF(content)
	case "cue":
		return parseCUE(content)
	default:
		return nil
	}
}

var extinfRegexp = regexp.MustCompile(`(?i)^#EXTINF:[^,]*,(.*)$`)

// parseM3U parses an (extended) M3U playlist. #EXTINF lines provide the title of
// the following resource path.
func parseM3U(content []byte) []playlistEntry {
	var entries []playlistEntry
	var pendingTitle string
	for raw := range strings.SplitSeq(string(content), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if m := extinfRegexp.FindStringSubmatch(line); m != nil {
				pendingTitle = strings.TrimSpace(m[1])
			}
			continue
		}
		entries = append(entries, playlistEntry{Path: line, Title: pendingTitle})
		pendingTitle = ""
	}
	return entries
}

// parsePLS parses a PLS playlist (an INI-like format with FileN/TitleN keys).
func parsePLS(content []byte) []playlistEntry {
	files := map[int]string{}
	titles := map[int]string{}
	maxIndex := 0
	for raw := range strings.SplitSeq(string(content), "\n") {
		line := strings.TrimSpace(raw)
		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(before))
		value := strings.TrimSpace(after)
		switch {
		case strings.HasPrefix(key, "file"):
			if n, err := strconv.Atoi(key[4:]); err == nil {
				files[n] = value
				if n > maxIndex {
					maxIndex = n
				}
			}
		case strings.HasPrefix(key, "title"):
			if n, err := strconv.Atoi(key[5:]); err == nil {
				titles[n] = value
			}
		}
	}

	var entries []playlistEntry
	for i := 1; i <= maxIndex; i++ {
		path, ok := files[i]
		if !ok {
			continue
		}
		entries = append(entries, playlistEntry{Path: path, Title: titles[i]})
	}
	return entries
}

// parseXSPF parses an XSPF (XML Shareable Playlist Format) playlist.
func parseXSPF(content []byte) []playlistEntry {
	var doc struct {
		Tracks []struct {
			Location string `xml:"location"`
			Title    string `xml:"title"`
		} `xml:"trackList>track"`
	}
	if err := xml.Unmarshal(content, &doc); err != nil {
		return nil
	}
	var entries []playlistEntry
	for _, t := range doc.Tracks {
		location := strings.TrimSpace(t.Location)
		if location == "" {
			continue
		}
		entries = append(entries, playlistEntry{Path: location, Title: strings.TrimSpace(t.Title)})
	}
	return entries
}

// parseCUE parses a CUE sheet, which splits one or more audio files into tracks
// marked by INDEX timestamps. Each TRACK becomes an entry pointing at the
// preceding FILE, with its start offset taken from INDEX 01 (or INDEX 00 as a
// fallback).
func parseCUE(content []byte) []playlistEntry {
	var entries []playlistEntry
	var currentFile string

	var (
		inTrack  bool
		title    string
		start    float64
		hasStart bool
	)
	flush := func() {
		if inTrack && currentFile != "" {
			entries = append(entries, playlistEntry{Path: currentFile, Title: title, Start: start})
		}
		inTrack, title, start, hasStart = false, "", 0, false
	}

	for raw := range strings.SplitSeq(string(content), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			continue
		}
		keyword, rest, _ := strings.Cut(line, " ")
		switch strings.ToUpper(keyword) {
		case "FILE":
			flush() // a pending track belongs to the previous FILE
			currentFile = cueValue(rest, true)
		case "TRACK":
			flush()
			inTrack = true
		case "TITLE":
			if inTrack {
				title = cueValue(rest, false)
			}
		case "INDEX":
			if !inTrack {
				continue
			}
			// INDEX 01 is the canonical track start; INDEX 00 (pregap) is only a
			// fallback. Index lines are ordered 00 before 01, so 01 wins.
			if num, t, ok := cueIndex(rest); ok {
				if num == 1 {
					start, hasStart = t, true
				} else if num == 0 && !hasStart {
					start = t
				}
			}
		}
	}
	flush()
	return entries
}

// cueValue extracts a CUE field value: the text between double quotes when
// quoted, otherwise either the first whitespace-delimited token (firstToken) or
// the whole remainder.
func cueValue(s string, firstToken bool) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' {
		if end := strings.IndexByte(s[1:], '"'); end >= 0 {
			return s[1 : 1+end]
		}
	}
	if firstToken {
		if i := strings.IndexByte(s, ' '); i >= 0 {
			return s[:i]
		}
	}
	return s
}

// cueIndex parses an INDEX line body ("NN MM:SS:FF") into its index number and
// time offset in seconds.
func cueIndex(rest string) (int, float64, bool) {
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return 0, 0, false
	}
	num, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, false
	}
	t, ok := parseCUETime(fields[1])
	if !ok {
		return 0, 0, false
	}
	return num, t, true
}

// parseCUETime parses a CUE timecode "MM:SS:FF" (FF = frames, 75 per second)
// into seconds.
func parseCUETime(s string) (float64, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, false
	}
	mm, e1 := strconv.Atoi(parts[0])
	ss, e2 := strconv.Atoi(parts[1])
	ff, e3 := strconv.Atoi(parts[2])
	if e1 != nil || e2 != nil || e3 != nil {
		return 0, false
	}
	return float64(mm)*60 + float64(ss) + float64(ff)/75, true
}
