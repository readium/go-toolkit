package audio

import (
	"context"
	"encoding/binary"
	"sort"
	"strings"
	"unicode/utf16"

	mp4 "github.com/abema/go-mp4"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"golang.org/x/sync/errgroup"
)

// chapterCoalesceGap is the largest gap between two chapter text samples that is
// still read in a single request. It keeps over-reading small while collapsing a
// chapter track stored contiguously into one read.
const chapterCoalesceGap = 64 << 10

// defaultChapterReadConcurrency bounds how many coalesced sample runs are
// fetched in parallel when the parser doesn't configure a concurrency. Chapter
// titles are often interleaved with the audio (one tiny sample every chapter),
// so a book can need dozens of scattered reads; fetching them concurrently
// hides the per-request latency of remote sources.
const defaultChapterReadConcurrency = 8

// probeMP4 extracts the total duration (in seconds) and any embedded chapters
// from an ISO-BMFF / MP4 container (m4a, m4b, m4p, mp4, …). concurrency bounds
// the parallel reads used to fetch scattered chapter samples (<= 0 for the
// default).
//
// The duration is read from the movie header (`mvhd`). Chapters are read from a
// QuickTime/iTunes chapter track: a track whose media handler is `text`, whose
// samples are length-prefixed UTF strings located in `mdat`, and whose
// per-sample timing comes from the `stts` table.
func probeMP4(ctx context.Context, res fetcher.Resource, extractChapters bool, concurrency int) (duration float64, chapters []chapterEntry, err error) {
	rs := fetcher.NewResourceReadSeeker(res)

	// Total duration from the movie header.
	if boxes, e := mp4.ExtractBoxWithPayload(rs, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()}); e == nil && len(boxes) > 0 {
		if mvhd, ok := boxes[0].Payload.(*mp4.Mvhd); ok && mvhd.Timescale > 0 {
			duration = float64(mvhd.GetDuration()) / float64(mvhd.Timescale)
		}
	}

	if extractChapters {
		chapters = extractMP4Chapters(ctx, res, rs, concurrency)
	}
	return duration, chapters, nil
}

// extractMP4Chapters returns the chapters of an MP4 file, or nil if it has none.
//
// A Nero chapter list (`moov/udta/chpl`) is preferred when present: it lives in
// the movie header, which has already been fetched, so it costs no extra reads.
// Otherwise it falls back to a QuickTime text chapter track, whose title samples
// live in `mdat` and may be scattered throughout the file.
func extractMP4Chapters(ctx context.Context, res fetcher.Resource, rs *fetcher.ResourceReadSeeker, concurrency int) []chapterEntry {
	if chapters := extractNeroChapters(ctx, res, rs); len(chapters) > 0 {
		return chapters
	}

	traks, err := mp4.ExtractBox(rs, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeTrak()})
	if err != nil {
		return nil
	}

	for _, trak := range traks {
		// Only consider tracks whose media handler is "text" (chapter tracks).
		hdlrs, err := mp4.ExtractBoxWithPayload(rs, trak, mp4.BoxPath{mp4.BoxTypeMdia(), mp4.BoxTypeHdlr()})
		if err != nil || len(hdlrs) == 0 {
			continue
		}
		hdlr, ok := hdlrs[0].Payload.(*mp4.Hdlr)
		if !ok || string(hdlr.HandlerType[:]) != "text" {
			continue
		}

		// Media timescale, used to convert sample durations to seconds.
		timescale := uint32(0)
		if mdhds, err := mp4.ExtractBoxWithPayload(rs, trak, mp4.BoxPath{mp4.BoxTypeMdia(), mp4.BoxTypeMdhd()}); err == nil && len(mdhds) > 0 {
			if mdhd, ok := mdhds[0].Payload.(*mp4.Mdhd); ok {
				timescale = mdhd.Timescale
			}
		}
		if timescale == 0 {
			continue
		}

		stblPath := mp4.BoxPath{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl()}

		// Sample durations (stts) -> cumulative start times.
		var sampleDeltas []uint32
		if boxes, err := mp4.ExtractBoxWithPayload(rs, trak, append(stblPath, mp4.BoxTypeStts())); err == nil && len(boxes) > 0 {
			if stts, ok := boxes[0].Payload.(*mp4.Stts); ok {
				for _, e := range stts.Entries {
					for i := uint32(0); i < e.SampleCount; i++ {
						sampleDeltas = append(sampleDeltas, e.SampleDelta)
					}
				}
			}
		}

		// Sample sizes (stsz).
		var sampleSizes []uint32
		if boxes, err := mp4.ExtractBoxWithPayload(rs, trak, append(stblPath, mp4.BoxTypeStsz())); err == nil && len(boxes) > 0 {
			if stsz, ok := boxes[0].Payload.(*mp4.Stsz); ok {
				if stsz.SampleSize != 0 {
					sampleSizes = make([]uint32, stsz.SampleCount)
					for i := range sampleSizes {
						sampleSizes[i] = stsz.SampleSize
					}
				} else {
					sampleSizes = stsz.EntrySize
				}
			}
		}

		// Chunk offsets (stco / co64).
		var chunkOffsets []uint64
		if boxes, err := mp4.ExtractBoxWithPayload(rs, trak, append(stblPath, mp4.BoxTypeStco())); err == nil && len(boxes) > 0 {
			if stco, ok := boxes[0].Payload.(*mp4.Stco); ok {
				for _, o := range stco.ChunkOffset {
					chunkOffsets = append(chunkOffsets, uint64(o))
				}
			}
		}
		if len(chunkOffsets) == 0 {
			if boxes, err := mp4.ExtractBoxWithPayload(rs, trak, append(stblPath, mp4.BoxTypeCo64())); err == nil && len(boxes) > 0 {
				if co64, ok := boxes[0].Payload.(*mp4.Co64); ok {
					chunkOffsets = co64.ChunkOffset
				}
			}
		}

		// Samples-to-chunk mapping (stsc).
		var stscEntries []mp4.StscEntry
		if boxes, err := mp4.ExtractBoxWithPayload(rs, trak, append(stblPath, mp4.BoxTypeStsc())); err == nil && len(boxes) > 0 {
			if stsc, ok := boxes[0].Payload.(*mp4.Stsc); ok {
				stscEntries = stsc.Entries
			}
		}

		offsets := sampleOffsets(sampleSizes, chunkOffsets, stscEntries)
		if len(offsets) == 0 {
			continue
		}

		// Per-sample start time, in sample (time) order.
		starts := make([]float64, len(offsets))
		var cumulative uint64
		for i := range offsets {
			starts[i] = float64(cumulative) / float64(timescale)
			if i < len(sampleDeltas) {
				cumulative += uint64(sampleDeltas[i])
			}
		}

		titles := readChapterTitles(ctx, res, offsets, sampleSizes, concurrency)

		chapters := make([]chapterEntry, 0, len(offsets))
		for i := range offsets {
			if titles[i] == "" {
				continue
			}
			chapters = append(chapters, chapterEntry{Title: titles[i], Start: starts[i]})
		}
		if len(chapters) > 0 {
			return chapters
		}
	}
	return nil
}

// sampleOffsets resolves the absolute byte offset of every sample from the
// sample-size, chunk-offset and sample-to-chunk tables.
func sampleOffsets(sampleSizes []uint32, chunkOffsets []uint64, stsc []mp4.StscEntry) []uint64 {
	if len(sampleSizes) == 0 || len(chunkOffsets) == 0 {
		return nil
	}

	// Expand stsc into a "samples per chunk" value for each chunk.
	samplesPerChunk := make([]uint32, len(chunkOffsets))
	for ci := range chunkOffsets {
		chunkNum := uint32(ci) + 1
		spc := uint32(1)
		for _, e := range stsc {
			if chunkNum >= e.FirstChunk {
				spc = e.SamplesPerChunk
			} else {
				break
			}
		}
		samplesPerChunk[ci] = spc
	}

	offsets := make([]uint64, 0, len(sampleSizes))
	si := 0
	for ci, chunkOff := range chunkOffsets {
		off := chunkOff
		for j := uint32(0); j < samplesPerChunk[ci] && si < len(sampleSizes); j++ {
			offsets = append(offsets, off)
			off += uint64(sampleSizes[si])
			si++
		}
		if si >= len(sampleSizes) {
			break
		}
	}
	return offsets
}

// readChapterTitles reads and decodes every chapter text sample, returning the
// titles indexed by sample.
//
// Chapter title samples are tiny (a few dozen bytes) but can be scattered across
// mdat. Reading each one through the block cache would pull a whole 256 KiB block
// per title, so instead this reuses already-cached blocks where possible and
// otherwise reads the exact sample bytes from the underlying resource, coalescing
// neighbouring samples into a single read.
func readChapterTitles(ctx context.Context, res fetcher.Resource, offsets []uint64, sizes []uint32, concurrency int) []string {
	titles := make([]string, len(offsets))
	if len(offsets) == 0 {
		return titles
	}
	if concurrency <= 0 {
		concurrency = defaultChapterReadConcurrency
	}

	// Underlying resource for exact reads, plus the cache (if any) to reuse.
	var cache *readCache
	raw := res
	if rc, ok := res.(*readCache); ok {
		cache = rc
		raw = rc.Resource
	}

	// Process samples in offset order so neighbours can be coalesced.
	order := make([]int, len(offsets))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return offsets[order[a]] < offsets[order[b]] })

	// Coalesce neighbouring samples into runs of [lo, hi] covering order[i:j].
	type sampleRun struct {
		i, j   int
		lo, hi int64
	}
	var runs []sampleRun
	for i := 0; i < len(order); {
		runStart := offsets[order[i]]
		runEnd := runStart + uint64(sizes[order[i]]) // exclusive
		j := i + 1
		for j < len(order) {
			off := offsets[order[j]]
			if off > runEnd+chapterCoalesceGap {
				break
			}
			if e := off + uint64(sizes[order[j]]); e > runEnd {
				runEnd = e
			}
			j++
		}
		runs = append(runs, sampleRun{i: i, j: j, lo: int64(runStart), hi: int64(runEnd) - 1})
		i = j
	}

	// Fetch the runs concurrently: titles are often interleaved with the audio,
	// one run per chapter, and sequential round trips would dominate the open
	// time on remote sources. Each goroutine writes to distinct title indexes.
	var g errgroup.Group
	g.SetLimit(concurrency)
	for _, run := range runs {
		g.Go(func() error {
			var data []byte
			if cache != nil {
				if b, hit := cache.cachedSlice(run.lo, run.hi); hit {
					data = b
				} else if cache.retain {
					// The cache is being retained for serving the publication:
					// pull whole blocks through it (costing up to a block of
					// extra transfer per run) so the ranges a browser requests
					// around each chapter are later served from memory.
					if b, err := cache.Read(ctx, run.lo, run.hi); err == nil {
						data = b
					}
				}
			}
			if data == nil {
				if b, err := raw.Read(ctx, run.lo, run.hi); err == nil {
					data = b
				}
			}

			for k := run.i; k < run.j; k++ {
				idx := order[k]
				start := int64(offsets[idx]) - run.lo
				end := start + int64(sizes[idx])
				if start >= 0 && end <= int64(len(data)) {
					titles[idx] = decodeChapterTitle(data[start:end])
				}
			}
			return nil
		})
	}
	_ = g.Wait()
	return titles
}

// decodeChapterTitle decodes a timed-text sample: a 16-bit big-endian length
// followed by the text payload, which may be UTF-8 or (with a BOM) UTF-16.
func decodeChapterTitle(sample []byte) string {
	if len(sample) < 2 {
		return ""
	}
	textLen := int(binary.BigEndian.Uint16(sample[:2]))
	if textLen <= 0 || 2+textLen > len(sample) {
		// Be lenient: fall back to whatever bytes follow the length prefix.
		textLen = len(sample) - 2
	}
	return decodeText(sample[2 : 2+textLen])
}

// extractNeroChapters reads a Nero chapter list (`moov/udta/chpl`) when present.
// The box lives in the movie header (already fetched), so this costs no extra
// reads. Returns nil when there is no chpl box.
func extractNeroChapters(ctx context.Context, res fetcher.Resource, rs *fetcher.ResourceReadSeeker) []chapterEntry {
	boxes, err := mp4.ExtractBox(rs, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeUdta(), mp4.StrToBoxType("chpl")})
	if err != nil || len(boxes) == 0 {
		return nil
	}
	info := boxes[0]
	data, rerr := res.Read(ctx, int64(info.Offset+info.HeaderSize), int64(info.Offset+info.Size)-1)
	if rerr != nil {
		return nil
	}
	return parseNeroChapters(data)
}

// parseNeroChapters parses a Nero `chpl` box payload: a FullBox header, an
// 8-bit chapter count (preceded by a 4-byte reserved field in version 1), then
// per chapter an 8-byte start time in 100 ns units and a length-prefixed UTF-8
// title.
func parseNeroChapters(b []byte) []chapterEntry {
	if len(b) < 5 {
		return nil
	}
	version := b[0]
	pos := 4 // version (1) + flags (3)
	if version != 0 {
		if len(b) < pos+5 {
			return nil
		}
		pos += 4 // reserved
	}
	count := int(b[pos])
	pos++

	chapters := make([]chapterEntry, 0, count)
	for c := 0; c < count; c++ {
		if pos+9 > len(b) {
			break
		}
		start := binary.BigEndian.Uint64(b[pos : pos+8])
		pos += 8
		titleLen := int(b[pos])
		pos++
		if pos+titleLen > len(b) {
			break
		}
		title := strings.TrimSpace(decodeText(b[pos : pos+titleLen]))
		pos += titleLen
		if title != "" {
			chapters = append(chapters, chapterEntry{Title: title, Start: float64(start) / 1e7})
		}
	}
	return chapters
}

// decodeText decodes a chapter title, honouring an optional UTF-16 byte-order
// mark and otherwise assuming UTF-8.
func decodeText(b []byte) string {
	if len(b) >= 2 {
		switch {
		case b[0] == 0xFE && b[1] == 0xFF:
			return decodeUTF16(b[2:], binary.BigEndian)
		case b[0] == 0xFF && b[1] == 0xFE:
			return decodeUTF16(b[2:], binary.LittleEndian)
		}
	}
	return string(b)
}

func decodeUTF16(b []byte, order binary.ByteOrder) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = order.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16))
}
