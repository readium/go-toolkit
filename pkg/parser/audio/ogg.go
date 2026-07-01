package audio

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/readium/go-toolkit/pkg/fetcher"
)

const oggCapturePattern = "OggS"

// probeOggDuration computes the duration (in seconds) of an Ogg-encapsulated
// stream (Opus or Vorbis). It reads the identification header from the start of
// the stream to determine the sample rate, then the last page from the tail to
// read the final granule position. This avoids scanning the whole file.
func probeOggDuration(ctx context.Context, res fetcher.Resource, size int64) float64 {
	if size <= 0 {
		return 0
	}

	headLen := min(int64(8192), size)
	head, err := res.Read(ctx, 0, headLen-1)
	if err != nil || len(head) == 0 {
		return 0
	}

	rate, preSkip, ok := oggIdentification(head)
	if !ok || rate == 0 {
		return 0
	}

	// Read a window at the end of the file large enough to contain the last page.
	tailLen := int64(65536)
	if tailLen > size {
		tailLen = size
	}
	tail, err := res.Read(ctx, size-tailLen, size-1)
	if err != nil || len(tail) == 0 {
		return 0
	}

	granule, ok := lastOggGranule(tail)
	if !ok {
		return 0
	}

	samples := int64(granule) - int64(preSkip)
	if samples <= 0 {
		return 0
	}
	return float64(samples) / float64(rate)
}

// oggIdentification parses the first Ogg page and returns the granule sample
// rate and pre-skip (pre-skip is zero for non-Opus streams).
//
// For Opus the granule positions are always expressed at 48 kHz, regardless of
// the original input sample rate, and the pre-skip must be subtracted. For
// Vorbis the granule rate is the audio sample rate declared in the header.
func oggIdentification(head []byte) (rate uint32, preSkip uint16, ok bool) {
	body := firstPageBody(head)
	if body == nil {
		return 0, 0, false
	}

	switch {
	case len(body) >= 19 && bytes.HasPrefix(body, []byte("OpusHead")):
		preSkip = binary.LittleEndian.Uint16(body[10:12])
		return 48000, preSkip, true
	case len(body) >= 16 && bytes.HasPrefix(body, []byte("\x01vorbis")):
		// version(4) channels(1) then sample rate at offset 12.
		return binary.LittleEndian.Uint32(body[12:16]), 0, true
	}
	return 0, 0, false
}

// firstPageBody returns the body bytes of the first Ogg page in head.
func firstPageBody(head []byte) []byte {
	if !bytes.HasPrefix(head, []byte(oggCapturePattern)) || len(head) < 27 {
		return nil
	}
	nseg := int(head[26])
	if len(head) < 27+nseg {
		return nil
	}
	segTable := head[27 : 27+nseg]
	bodyLen := 0
	for _, s := range segTable {
		bodyLen += int(s)
	}
	bodyStart := 27 + nseg
	if bodyStart+bodyLen > len(head) {
		bodyLen = len(head) - bodyStart
	}
	return head[bodyStart : bodyStart+bodyLen]
}

// lastOggGranule scans a tail window for the last Ogg page carrying a valid
// granule position (i.e. not -1, which marks a page where no packet completes).
func lastOggGranule(tail []byte) (uint64, bool) {
	pattern := []byte(oggCapturePattern)
	var granule uint64
	found := false
	for i := 0; i+27 <= len(tail); {
		idx := bytes.Index(tail[i:], pattern)
		if idx < 0 {
			break
		}
		pos := i + idx
		if pos+27 > len(tail) {
			break
		}
		g := binary.LittleEndian.Uint64(tail[pos+6 : pos+14])
		if g != 0xFFFFFFFFFFFFFFFF {
			granule = g
			found = true
		}
		i = pos + 4
	}
	return granule, found
}
