package audio

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
)

// bytesResource wraps a byte slice as a fetcher.Resource for probe tests.
func bytesResource(b []byte) (fetcher.Resource, int64) {
	link := manifest.Link{Href: manifest.MustNewHREFFromString("probe", false)}
	return fetcher.NewBytesResource(link, func() []byte { return b }), int64(len(b))
}

func TestProbeWAVDuration(t *testing.T) {
	le := binary.LittleEndian
	var buf []byte
	put16 := func(v uint16) { buf = le.AppendUint16(buf, v) }
	put32 := func(v uint32) { buf = le.AppendUint32(buf, v) }

	data := make([]byte, 8000) // 1s of 8000 Hz / mono / 8-bit PCM -> byteRate 8000
	buf = append(buf, "RIFF"...)
	put32(uint32(4 + (8 + 16) + (8 + len(data))))
	buf = append(buf, "WAVE"...)
	buf = append(buf, "fmt "...)
	put32(16)
	put16(1)    // PCM
	put16(1)    // channels
	put32(8000) // sample rate
	put32(8000) // byte rate
	put16(1)    // block align
	put16(8)    // bits per sample
	buf = append(buf, "data"...)
	put32(uint32(len(data)))
	buf = append(buf, data...)

	res, size := bytesResource(buf)
	assert.InDelta(t, 1.0, probeWAVDuration(t.Context(), res, size), 0.0001)
}

// Trailing garbage past the RIFF-declared size must not corrupt the duration.
func TestProbeWAVIgnoresTrailingGarbage(t *testing.T) {
	le := binary.LittleEndian
	var buf []byte
	put16 := func(v uint16) { buf = le.AppendUint16(buf, v) }
	put32 := func(v uint32) { buf = le.AppendUint32(buf, v) }

	data := make([]byte, 8000)
	buf = append(buf, "RIFF"...)
	put32(uint32(4 + (8 + 16) + (8 + len(data)))) // declared size excludes garbage
	buf = append(buf, "WAVE"...)
	buf = append(buf, "fmt "...)
	put32(16)
	put16(1)
	put16(1)
	put32(8000)
	put32(8000)
	put16(1)
	put16(8)
	buf = append(buf, "data"...)
	put32(uint32(len(data)))
	buf = append(buf, data...)

	// Append a bogus "data" chunk that would corrupt the duration if parsed.
	buf = append(buf, "data"...)
	put32(99999)

	res, size := bytesResource(buf)
	assert.InDelta(t, 1.0, probeWAVDuration(t.Context(), res, size), 0.0001)
}

func TestProbeAIFFDuration(t *testing.T) {
	be := binary.BigEndian
	var buf []byte
	put16 := func(v uint16) { buf = be.AppendUint16(buf, v) }
	put32 := func(v uint32) { buf = be.AppendUint32(buf, v) }
	// 80-bit IEEE extended representation of 8000.0
	sampleRate := []byte{0x40, 0x0B, 0xFA, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	buf = append(buf, "FORM"...)
	put32(4 + (8 + 18))
	buf = append(buf, "AIFF"...)
	buf = append(buf, "COMM"...)
	put32(18)
	put16(1)    // channels
	put32(8000) // sample frames -> 1s at 8000 Hz
	put16(8)    // sample size
	buf = append(buf, sampleRate...)

	res, size := bytesResource(buf)
	assert.InDelta(t, 1.0, probeAIFFDuration(t.Context(), res, size), 0.0001)
}

func TestProbeFLACDuration(t *testing.T) {
	streamInfo := make([]byte, 34)
	// Packed fields at byte 10: 8000 Hz sample rate, 8000 total samples.
	streamInfo[10] = 0x01
	streamInfo[11] = 0xF4
	streamInfo[12] = 0x00
	streamInfo[13] = 0xF0 // bps low nibble + total-samples high nibble (0)
	streamInfo[14] = 0x00
	streamInfo[15] = 0x00
	streamInfo[16] = 0x1F
	streamInfo[17] = 0x40 // total samples low 32 bits = 0x1F40 = 8000

	var buf []byte
	buf = append(buf, "fLaC"...)
	buf = append(buf, 0x80, 0x00, 0x00, 0x22) // last block, type 0 (STREAMINFO), len 34
	buf = append(buf, streamInfo...)

	res, _ := bytesResource(buf)
	assert.InDelta(t, 1.0, probeFLACDuration(t.Context(), res), 0.0001)
}

func TestProbeAACDuration(t *testing.T) {
	// Two header-only ADTS frames at 44100 Hz (freq index 4), frame length 7.
	frame := []byte{0xFF, 0xF1, 0x50, 0x80, 0x00, 0xE0, 0x00}
	buf := append(append([]byte{}, frame...), frame...)

	res, size := bytesResource(buf)
	expected := 2 * 1024.0 / 44100.0
	assert.InDelta(t, expected, probeAACDuration(t.Context(), res, size), 0.0001)
}

func TestProbeWebMDuration(t *testing.T) {
	durationData := make([]byte, 4)
	binary.BigEndian.PutUint32(durationData, math.Float32bits(5000.0)) // 5000 * 1ms = 5s

	var info []byte
	info = append(info, 0x2A, 0xD7, 0xB1, 0x84, 0x00, 0x0F, 0x42, 0x40) // TimecodeScale = 1,000,000
	info = append(info, 0x44, 0x89, 0x84)                               // Duration, 4-byte payload
	info = append(info, durationData...)

	var segment []byte
	segment = append(segment, 0x15, 0x49, 0xA9, 0x66, byte(0x80|len(info)))
	segment = append(segment, info...)

	var buf []byte
	buf = append(buf, 0x1A, 0x45, 0xDF, 0xA3, 0x80)                    // EBML header, empty
	buf = append(buf, 0x18, 0x53, 0x80, 0x67, byte(0x80|len(segment))) // Segment
	buf = append(buf, segment...)

	res, size := bytesResource(buf)
	assert.InDelta(t, 5.0, probeWebMDuration(t.Context(), res, size), 0.0001)
}
