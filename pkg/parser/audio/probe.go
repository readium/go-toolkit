package audio

import (
	"context"
	"math"
	"path/filepath"
	"strings"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// probeResult holds the per-resource information extracted while probing an
// audio file.
type probeResult struct {
	Duration float64        // Length of the resource in seconds (0 if unknown).
	Bitrate  float64        // Average bitrate in kbps (0 if unknown).
	Chapters []chapterEntry // Embedded chapters, if any.
}

// probeAudioFile extracts the duration, bitrate and any embedded chapters from a
// single audio resource. concurrency bounds the parallel reads used to fetch
// scattered chapter samples (<= 0 for the default). The bitrate is computed as
// the average over the whole resource, which is the most portable definition
// across the many supported container formats.
func probeAudioFile(ctx context.Context, res fetcher.Resource, link manifest.Link, tags *audioTags, extractChapters bool, concurrency int) probeResult {
	size, _ := res.Length(ctx)

	var result probeResult
	switch audioFamily(link) {
	case familyMP4:
		duration, chapters, _ := probeMP4(ctx, res, extractChapters, concurrency)
		result.Duration = duration
		result.Chapters = chapters
	case familyOgg:
		result.Duration = probeOggDuration(ctx, res, size)
		if extractChapters {
			result.Chapters = vorbisChapters(tags)
		}
	case familyFLAC:
		result.Duration = probeFLACDuration(ctx, res)
		if extractChapters {
			result.Chapters = vorbisChapters(tags)
		}
	case familyWAV:
		result.Duration = probeWAVDuration(ctx, res, size)
	case familyAIFF:
		result.Duration = probeAIFFDuration(ctx, res, size)
	case familyMP3:
		result.Duration = probeMP3Duration(ctx, res, size)
	case familyWebM:
		result.Duration = probeWebMDuration(ctx, res, size)
	case familyAAC:
		result.Duration = probeAACDuration(ctx, res, size)
	}

	if result.Duration > 0 && size > 0 {
		// kbps = bytes * 8 bits / 1000 / seconds, rounded to one decimal place.
		kbps := float64(size) * 8 / 1000 / result.Duration
		result.Bitrate = math.Round(kbps*10) / 10
	}
	return result
}

type audioFormatFamily int

const (
	familyUnknown audioFormatFamily = iota
	familyMP4
	familyOgg
	familyFLAC
	familyWAV
	familyAIFF
	familyMP3
	familyWebM
	familyAAC
)

// audioFamily classifies an audio resource into a parsing family based on its
// file extension.
func audioFamily(link manifest.Link) audioFormatFamily {
	switch linkExtension(link) {
	case "mp4", "m4a", "m4b", "m4p", "m4r", "alac":
		return familyMP4
	case "ogg", "oga", "opus", "mogg":
		return familyOgg
	case "flac":
		return familyFLAC
	case "wav", "wave":
		return familyWAV
	case "aiff", "aif", "aifc":
		return familyAIFF
	case "mp3":
		return familyMP3
	case "webm":
		return familyWebM
	case "aac":
		return familyAAC
	}
	return familyUnknown
}

// linkExtension returns the lower-cased file extension (without the dot) of a
// link's HREF.
func linkExtension(link manifest.Link) string {
	ext := strings.ToLower(filepath.Ext(link.URL(nil, nil).Path()))
	return strings.TrimPrefix(ext, ".")
}

// readRange reads length bytes starting at offset, clamping at the resource's
// end. It returns nil on error.
func readRange(ctx context.Context, res fetcher.Resource, offset, length int64) []byte {
	if length <= 0 {
		return nil
	}
	data, err := res.Read(ctx, offset, offset+length-1)
	if err != nil {
		return nil
	}
	return data
}
