package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"

	"github.com/readium/go-toolkit/pkg/fetcher"
)

// skipID3v2 returns the byte offset right after a leading ID3v2 tag, or 0 if the
// resource doesn't start with one. ID3v2 tags can be prepended to MP3, FLAC and
// AAC streams.
func skipID3v2(ctx context.Context, res fetcher.Resource) int64 {
	header := readRange(ctx, res, 0, 10)
	if len(header) < 10 || !bytes.HasPrefix(header, []byte("ID3")) {
		return 0
	}
	// The size is a 28-bit synch-safe integer (7 bits per byte).
	size := int64(header[6]&0x7F)<<21 | int64(header[7]&0x7F)<<14 | int64(header[8]&0x7F)<<7 | int64(header[9]&0x7F)
	footer := int64(0)
	if header[5]&0x10 != 0 { // footer present flag
		footer = 10
	}
	return 10 + size + footer
}

// probeFLACDuration reads the FLAC STREAMINFO metadata block to compute the
// duration from the total sample count and sample rate.
func probeFLACDuration(ctx context.Context, res fetcher.Resource) float64 {
	base := skipID3v2(ctx, res)
	magic := readRange(ctx, res, base, 4)
	if !bytes.Equal(magic, []byte("fLaC")) {
		return 0
	}
	// STREAMINFO is always the first metadata block: 4-byte block header
	// followed by 34 bytes of payload.
	block := readRange(ctx, res, base+4, 4+34)
	if len(block) < 4+34 {
		return 0
	}
	if block[0]&0x7F != 0 { // block type 0 == STREAMINFO
		return 0
	}
	si := block[4:]
	// 20 bits sample rate, 3 bits channels, 5 bits bits-per-sample, 36 bits total
	// samples, packed starting at byte 10 of STREAMINFO.
	sampleRate := uint32(si[10])<<12 | uint32(si[11])<<4 | uint32(si[12])>>4
	totalSamples := uint64(si[13]&0x0F)<<32 | uint64(si[14])<<24 | uint64(si[15])<<16 | uint64(si[16])<<8 | uint64(si[17])
	if sampleRate == 0 || totalSamples == 0 {
		return 0
	}
	return float64(totalSamples) / float64(sampleRate)
}

// probeWAVDuration walks the RIFF chunks of a WAV file to compute the duration
// from the `data` chunk size and the byte rate declared in the `fmt ` chunk.
func probeWAVDuration(ctx context.Context, res fetcher.Resource, size int64) float64 {
	header := readRange(ctx, res, 0, 12)
	if len(header) < 12 || !bytes.Equal(header[0:4], []byte("RIFF")) || !bytes.Equal(header[8:12], []byte("WAVE")) {
		return 0
	}

	var byteRate uint32
	var sampleRate uint32
	var channels uint16
	var bitsPerSample uint16
	var dataSize int64 = -1
	var factSamples int64 = -1

	// Parsing is bounded by both the resource size and the RIFF-declared chunk
	// range (bytes 4-7), so trailing garbage after the RIFF payload is ignored.
	limit := riffLimit(int64(binary.LittleEndian.Uint32(header[4:8])), size)
	offset := int64(12)
	for offset+8 <= limit {
		chunkHeader := readRange(ctx, res, offset, 8)
		if len(chunkHeader) < 8 {
			break
		}
		id := string(chunkHeader[0:4])
		chunkSize := int64(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		body := offset + 8

		switch id {
		case "fmt ":
			fmtBody := readRange(ctx, res, body, min64(chunkSize, 16))
			if len(fmtBody) >= 16 {
				channels = binary.LittleEndian.Uint16(fmtBody[2:4])
				sampleRate = binary.LittleEndian.Uint32(fmtBody[4:8])
				byteRate = binary.LittleEndian.Uint32(fmtBody[8:12])
				bitsPerSample = binary.LittleEndian.Uint16(fmtBody[14:16])
			}
		case "fact":
			factBody := readRange(ctx, res, body, 4)
			if len(factBody) >= 4 {
				factSamples = int64(binary.LittleEndian.Uint32(factBody[0:4]))
			}
		case "data":
			dataSize = chunkSize
		}

		// Chunks are word-aligned: an odd size is followed by a pad byte.
		next := body + chunkSize
		if chunkSize%2 == 1 {
			next++
		}
		if next <= offset { // guard against overflow or a non-advancing chunk
			break
		}
		offset = next
	}

	// Prefer the exact sample count from `fact` (used by compressed WAV).
	if factSamples > 0 && sampleRate > 0 {
		return float64(factSamples) / float64(sampleRate)
	}
	if dataSize > 0 && byteRate > 0 {
		return float64(dataSize) / float64(byteRate)
	}
	// Fall back to reconstructing the byte rate for PCM.
	if dataSize > 0 && sampleRate > 0 && channels > 0 && bitsPerSample > 0 {
		computed := float64(sampleRate) * float64(channels) * float64(bitsPerSample) / 8
		if computed > 0 {
			return float64(dataSize) / computed
		}
	}
	return 0
}

// probeAIFFDuration reads the COMM chunk of an AIFF/AIFC file to compute the
// duration from the number of sample frames and the sample rate.
func probeAIFFDuration(ctx context.Context, res fetcher.Resource, size int64) float64 {
	header := readRange(ctx, res, 0, 12)
	if len(header) < 12 || !bytes.Equal(header[0:4], []byte("FORM")) {
		return 0
	}
	form := string(header[8:12])
	if form != "AIFF" && form != "AIFC" {
		return 0
	}

	limit := riffLimit(int64(binary.BigEndian.Uint32(header[4:8])), size)
	offset := int64(12)
	for offset+8 <= limit {
		chunkHeader := readRange(ctx, res, offset, 8)
		if len(chunkHeader) < 8 {
			break
		}
		id := string(chunkHeader[0:4])
		chunkSize := int64(binary.BigEndian.Uint32(chunkHeader[4:8]))
		body := offset + 8

		if id == "COMM" {
			commBody := readRange(ctx, res, body, 18)
			if len(commBody) >= 18 {
				numSampleFrames := binary.BigEndian.Uint32(commBody[2:6])
				sampleRate := decodeExtendedFloat(commBody[8:18])
				if numSampleFrames > 0 && sampleRate > 0 {
					return float64(numSampleFrames) / sampleRate
				}
			}
			return 0
		}

		next := body + chunkSize
		if chunkSize%2 == 1 {
			next++
		}
		if next <= offset { // guard against overflow or a non-advancing chunk
			break
		}
		offset = next
	}
	return 0
}

// riffLimit returns the smaller of the resource size and the RIFF/FORM-declared
// payload end (the declared content size plus the 8-byte top-level header),
// clamping to the resource size when the declaration is absent or too large.
func riffLimit(declaredSize, size int64) int64 {
	end := declaredSize + 8
	if end >= 12 && end < size {
		return end
	}
	return size
}

// decodeExtendedFloat decodes an 80-bit IEEE 754 extended-precision float, as
// used for the sample rate in AIFF COMM chunks.
func decodeExtendedFloat(b []byte) float64 {
	if len(b) < 10 {
		return 0
	}
	sign := 1.0
	if b[0]&0x80 != 0 {
		sign = -1.0
	}
	exponent := int(uint16(b[0]&0x7F)<<8 | uint16(b[1]))
	mantissa := binary.BigEndian.Uint64(b[2:10])
	if exponent == 0 && mantissa == 0 {
		return 0
	}
	return sign * float64(mantissa) * math.Pow(2, float64(exponent-16383-63))
}

var mp3SampleRates = [4][3]uint32{
	{11025, 12000, 8000},  // MPEG 2.5
	{0, 0, 0},             // reserved
	{22050, 24000, 16000}, // MPEG 2
	{44100, 48000, 32000}, // MPEG 1
}

// mp3Bitrates is indexed by [versionBit][layerBit][bitrateIndex] in kbps, where
// versionBit is 1 for MPEG1 and 0 for MPEG2/2.5, and layerBit is 1/2/3.
var mp3Bitrates = map[[3]int][16]uint32{
	{1, 1, 0}: {0, 32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448, 0}, // V1 L1
	{1, 2, 0}: {0, 32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 0},    // V1 L2
	{1, 3, 0}: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},     // V1 L3
	{0, 1, 0}: {0, 32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256, 0},    // V2 L1
	{0, 2, 0}: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},         // V2 L2/L3
}

// probeMP3Duration computes the duration of an MP3 stream. It honours a VBR
// header (Xing/Info/VBRI) when present, and otherwise assumes constant bitrate.
func probeMP3Duration(ctx context.Context, res fetcher.Resource, size int64) float64 {
	start := skipID3v2(ctx, res)

	// Find the first valid frame header within a small window after the tag.
	window := readRange(ctx, res, start, 8192)
	frameOff := -1
	var header []byte
	for i := 0; i+4 <= len(window); i++ {
		if window[i] == 0xFF && window[i+1]&0xE0 == 0xE0 {
			if h := parseMP3Header(window[i : i+4]); h != nil {
				frameOff = i
				header = window[i : i+4]
				break
			}
		}
	}
	if frameOff < 0 {
		return 0
	}

	versionBits := (header[1] >> 3) & 0x03
	layerBits := (header[1] >> 1) & 0x03
	sampleRate := mp3SampleRates[versionBits][(header[2]>>2)&0x03]
	if sampleRate == 0 {
		return 0
	}
	samplesPerFrame := mp3SamplesPerFrame(versionBits, layerBits)

	// Side-information size determines where a VBR header sits in the frame.
	mpeg1 := versionBits == 3
	channels := (header[3] >> 6) & 0x03
	mono := channels == 3
	var sideInfo int
	switch {
	case mpeg1 && mono:
		sideInfo = 17
	case mpeg1:
		sideInfo = 32
	case mono:
		sideInfo = 9
	default:
		sideInfo = 17
	}

	frameStart := start + int64(frameOff)
	frame := readRange(ctx, res, frameStart, 1024)
	if frame != nil {
		// Xing/Info header.
		xingOff := 4 + sideInfo
		if xingOff+8 <= len(frame) {
			tag := string(frame[xingOff : xingOff+4])
			if tag == "Xing" || tag == "Info" {
				flags := binary.BigEndian.Uint32(frame[xingOff+4 : xingOff+8])
				if flags&0x01 != 0 && xingOff+12 <= len(frame) {
					frames := binary.BigEndian.Uint32(frame[xingOff+8 : xingOff+12])
					if frames > 0 {
						return float64(frames) * float64(samplesPerFrame) / float64(sampleRate)
					}
				}
			}
		}
		// VBRI header, always 32 bytes after the frame header.
		vbriOff := 4 + 32
		if vbriOff+18 <= len(frame) && string(frame[vbriOff:vbriOff+4]) == "VBRI" {
			frames := binary.BigEndian.Uint32(frame[vbriOff+14 : vbriOff+18])
			if frames > 0 {
				return float64(frames) * float64(samplesPerFrame) / float64(sampleRate)
			}
		}
	}

	// Constant-bitrate fallback.
	bitrate := mp3Bitrate(versionBits, layerBits, (header[2]>>4)&0x0F)
	if bitrate == 0 {
		return 0
	}
	audioBytes := size - frameStart
	if audioBytes <= 0 {
		return 0
	}
	return float64(audioBytes) * 8 / (float64(bitrate) * 1000)
}

func parseMP3Header(b []byte) []byte {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return nil
	}
	version := (b[1] >> 3) & 0x03
	layer := (b[1] >> 1) & 0x03
	bitrateIdx := (b[2] >> 4) & 0x0F
	sampleIdx := (b[2] >> 2) & 0x03
	if version == 1 || layer == 0 || bitrateIdx == 0 || bitrateIdx == 15 || sampleIdx == 3 {
		return nil // reserved/invalid combinations
	}
	return b
}

func mp3SamplesPerFrame(versionBits, layerBits uint8) int {
	switch layerBits {
	case 3: // Layer I
		return 384
	case 2: // Layer II
		return 1152
	default: // Layer III
		if versionBits == 3 { // MPEG1
			return 1152
		}
		return 576
	}
}

func mp3Bitrate(versionBits, layerBits, index uint8) uint32 {
	verKey := 0
	if versionBits == 3 {
		verKey = 1
	}
	layer := int(4 - layerBits) // layerBits 3->1, 2->2, 1->3
	key := [3]int{verKey, layer, 0}
	if verKey == 0 && (layer == 2 || layer == 3) {
		key = [3]int{0, 2, 0}
	}
	table, ok := mp3Bitrates[key]
	if !ok || int(index) >= len(table) {
		return 0
	}
	return table[index]
}

// EBML / Matroska element IDs used to locate the WebM duration.
const (
	ebmlSegment       = 0x18538067
	ebmlInfo          = 0x1549A966
	ebmlTimecodeScale = 0x2AD7B1
	ebmlDuration      = 0x4489
)

// probeWebMDuration parses the EBML structure of a WebM/Matroska file to read
// the Duration element from the Segment Info, scaled by the TimecodeScale.
func probeWebMDuration(ctx context.Context, res fetcher.Resource, size int64) float64 {
	// The Segment Info element is normally near the beginning of the file.
	readLen := int64(1 << 20)
	if readLen > size {
		readLen = size
	}
	buf := readRange(ctx, res, 0, readLen)
	if len(buf) < 4 || !bytes.HasPrefix(buf, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return 0
	}

	timecodeScale := uint64(1000000) // default: 1 ms in nanoseconds
	var duration float64

	var walk func(b []byte, depth int)
	walk = func(b []byte, depth int) {
		pos := 0
		for pos < len(b) {
			id, idLen := ebmlReadID(b[pos:])
			if idLen == 0 {
				return
			}
			pos += idLen
			length, sizeLen := ebmlReadSize(b[pos:])
			if sizeLen == 0 {
				return
			}
			pos += sizeLen
			// Clamp to the available window. This also handles "unknown size"
			// master elements (a live/streamed Segment), whose declared length
			// spans the rest of the stream.
			end := pos + int(length)
			if length < 0 || end > len(b) {
				end = len(b)
			}
			data := b[pos:end]
			switch id {
			case ebmlSegment, ebmlInfo:
				if depth < 3 {
					walk(data, depth+1)
				}
			case ebmlTimecodeScale:
				timecodeScale = ebmlReadUint(data)
			case ebmlDuration:
				duration = ebmlReadFloat(data)
			}
			pos = end
		}
	}
	walk(buf, 0)

	if duration <= 0 {
		return 0
	}
	return duration * float64(timecodeScale) / 1e9
}

// ebmlReadID reads a variable-length EBML element ID, keeping the length marker.
func ebmlReadID(b []byte) (uint64, int) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0
	}
	length := 1
	for mask := byte(0x80); mask != 0; mask >>= 1 {
		if b[0]&mask != 0 {
			break
		}
		length++
	}
	if length > 4 || length > len(b) {
		return 0, 0
	}
	var id uint64
	for i := 0; i < length; i++ {
		id = id<<8 | uint64(b[i])
	}
	return id, length
}

// ebmlReadSize reads a variable-length EBML data size, stripping the marker bit.
func ebmlReadSize(b []byte) (int64, int) {
	if len(b) == 0 || b[0] == 0 {
		return -1, 0
	}
	length := 1
	mask := byte(0x80)
	for mask != 0 {
		if b[0]&mask != 0 {
			break
		}
		length++
		mask >>= 1
	}
	if length > 8 || length > len(b) {
		return -1, 0
	}
	value := uint64(b[0] & (mask - 1))
	for i := 1; i < length; i++ {
		value = value<<8 | uint64(b[i])
	}
	return int64(value), length
}

func ebmlReadUint(b []byte) uint64 {
	var v uint64
	for _, x := range b {
		v = v<<8 | uint64(x)
	}
	return v
}

func ebmlReadFloat(b []byte) float64 {
	switch len(b) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b)))
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	}
	return 0
}

var aacSampleRates = [16]uint32{
	96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050,
	16000, 12000, 11025, 8000, 7350, 0, 0, 0,
}

// aacScanWindow caps how many bytes of a raw AAC stream are read into memory.
// ADTS has no global header, so for longer streams the duration is extrapolated
// from the average frame size measured within this window.
const aacScanWindow = 16 << 20 // 16 MiB

// probeAACDuration counts ADTS frames in a raw AAC stream to compute the
// duration. Each frame carries 1024 samples per raw data block.
func probeAACDuration(ctx context.Context, res fetcher.Resource, size int64) float64 {
	start := skipID3v2(ctx, res)
	if size <= start {
		return 0
	}
	remaining := size - start
	readLen := remaining
	if readLen > aacScanWindow {
		readLen = aacScanWindow
	}
	data := readRange(ctx, res, start, readLen)
	if len(data) < 7 {
		return 0
	}

	var sampleRate uint32
	var totalSamples uint64
	pos := 0
	consumed := 0
	for pos+7 <= len(data) {
		if data[pos] != 0xFF || data[pos+1]&0xF0 != 0xF0 {
			pos++
			continue
		}
		freqIdx := (data[pos+2] >> 2) & 0x0F
		rate := aacSampleRates[freqIdx]
		frameLen := int(uint32(data[pos+3]&0x03)<<11 | uint32(data[pos+4])<<3 | uint32(data[pos+5])>>5)
		if rate == 0 || frameLen < 7 {
			pos++
			continue
		}
		if pos+frameLen > len(data) {
			break // frame extends past the read window
		}
		if sampleRate == 0 {
			sampleRate = rate
		}
		blocks := uint64(data[pos+6]&0x03) + 1
		totalSamples += 1024 * blocks
		pos += frameLen
		consumed = pos
	}

	if sampleRate == 0 || totalSamples == 0 || consumed == 0 {
		return 0
	}
	samples := float64(totalSamples)
	if readLen < remaining {
		// Extrapolate to the full stream, assuming a roughly uniform frame rate.
		samples *= float64(remaining) / float64(consumed)
	}
	return samples / float64(sampleRate)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
