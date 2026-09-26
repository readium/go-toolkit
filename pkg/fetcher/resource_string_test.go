package fetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func TestReadResourceAsStringUTF8MatchesDecoder(t *testing.T) {
	inputs := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"ASCII", []byte("<p>Chapter 1\nA new paragraph.</p>")},
		{"multilingual", []byte("Français 日本語 العربية हिन्दी 🦉")},
		{"BOM", []byte("\ufeff<p>日本語</p>")},
		{"embeddedBOM", []byte("before\ufeffafter")},
		{"NUL", []byte("before\x00after")},
		{"invalidByte", []byte("before\xffafter")},
		{"overlong", []byte{0xc0, 0xaf}},
		{"surrogate", []byte{0xed, 0xa0, 0x80}},
		{"truncated", []byte{0xe2, 0x82}},
		{"aboveUnicodeLimit", []byte{0xf4, 0x90, 0x80, 0x80}},
	}
	for _, charset := range []struct {
		name      string
		mediaType *mediatype.MediaType
	}{
		{"noMediaType", nil},
		{"default", &mediatype.HTML},
		{"UTF8", mediatype.MaybeNewOfString("text/plain;charset=utf-8")},
		{"unknownCharset", mediatype.MaybeNewOfString("text/plain;charset=unknown-test-charset")},
	} {
		for _, input := range inputs {
			t.Run(charset.name+"/"+input.name, func(t *testing.T) {
				resource := &BytesResource{link: manifest.Link{MediaType: charset.mediaType}, _bytes: input.data}
				want, wantErr := readResourceAsStringDecoder(t.Context(), resource)
				got, gotErr := ReadResourceAsString(t.Context(), resource)
				require.Nil(t, wantErr)
				require.Nil(t, gotErr)
				assert.Equal(t, want, got)
				if strings.Contains(input.name, "BOM") {
					assert.Contains(t, got, "\ufeff")
				}
			})
		}
	}
}

func TestReadResourceAsStringLegacyEncodings(t *testing.T) {
	for _, test := range []struct {
		charset string
		data    []byte
		want    string
	}{
		{"windows-1252", []byte("caf\xe9 \x80"), "café €"},
		{"iso-8859-1", []byte("caf\xe9 \x80"), "café €"},
		// These bytes are valid UTF-8, but the declared charset still controls
		// their interpretation, including embedded NULs in UTF-16 input.
		{"windows-1252", []byte("é"), "Ã©"},
		{"utf-16le", []byte("A\x00B\x00"), "AB"},
		{"utf-16be", []byte("\x00A\x00B"), "AB"},
		{"shift_jis", []byte{0x93, 0xfa, 0x96, 0x7b}, "日本"},
	} {
		t.Run(test.charset+"/"+test.want, func(t *testing.T) {
			mediaType, err := mediatype.NewOfString("text/plain;charset=" + test.charset)
			require.NoError(t, err)
			resource := &BytesResource{link: manifest.Link{MediaType: &mediaType}, _bytes: test.data}
			want, wantErr := readResourceAsStringDecoder(t.Context(), resource)
			got, gotErr := ReadResourceAsString(t.Context(), resource)
			require.Nil(t, wantErr)
			require.Nil(t, gotErr)
			assert.Equal(t, test.want, want)
			assert.Equal(t, want, got)
		})
	}
}

func TestReadResourceAsStringFailure(t *testing.T) {
	wantErr := NotFound(errors.New("missing chapter"))
	got, gotErr := ReadResourceAsString(t.Context(), NewFailureResource(manifest.Link{}, wantErr))
	assert.Empty(t, got)
	assert.Same(t, wantErr, gotErr)
}

func TestReadResourceAsStringOwnsReturnedBytes(t *testing.T) {
	data := []byte("Chapter 1 日本語")
	resource := &BytesResource{_bytes: data}
	got, err := ReadResourceAsString(t.Context(), resource)
	require.Nil(t, err)
	clear(data)
	assert.Equal(t, "Chapter 1 日本語", got)
}

// readResourceAsStringDecoder retains the original decoding path for output
// equivalence checks and side-by-side benchmarks.
func readResourceAsStringDecoder(ctx context.Context, r Resource) (string, *ResourceError) {
	bin, ex := r.Read(ctx, 0, 0)
	if ex != nil {
		return "", ex
	}
	var cs encoding.Encoding
	if r.Link().MediaType != nil {
		cs = r.Link().MediaType.Charset()
	}
	if cs == nil {
		cs = unicode.UTF8
	}
	utf8bytes, err := cs.NewDecoder().Bytes(bin)
	if err != nil {
		return "", Other(err)
	}
	return string(utf8bytes), nil
}

func BenchmarkReadResourceAsString(b *testing.B) {
	for _, input := range []struct {
		name, text string
		mediaType  *mediatype.MediaType
	}{
		{"ASCII", "<p>Chapter one: a paragraph of publication content.</p>\n", &mediatype.HTML},
		{"multilingual", "<p>Français 日本語 العربية हिन्दी 🦉</p>\n", mediatype.MaybeNewOfString("text/html;charset=utf-8")},
		{"invalidUTF8", "<p>Malformed byte \xff in publication content.</p>\n", &mediatype.HTML},
		{"legacy", "<p>Un caf\xe9 co\xfbte 5 \x80.</p>\n", mediatype.MaybeNewOfString("text/html;charset=windows-1252")},
	} {
		for _, size := range []int{4 << 10, 64 << 10, 512 << 10} {
			b.Run(fmt.Sprintf("%s/%dKiB", input.name, size>>10), func(b *testing.B) {
				data := bytes.Repeat([]byte(input.text), (size+len(input.text)-1)/len(input.text))
				resource := &BytesResource{link: manifest.Link{MediaType: input.mediaType}, _bytes: data}
				want, err := readResourceAsStringDecoder(b.Context(), resource)
				if err != nil {
					b.Fatal(err)
				}
				for _, implementation := range []struct {
					name string
					read func(context.Context, Resource) (string, *ResourceError)
				}{
					{"decoder", readResourceAsStringDecoder},
					{"optimized", ReadResourceAsString},
				} {
					b.Run(implementation.name, func(b *testing.B) {
						got, err := implementation.read(b.Context(), resource)
						if err != nil || got != want {
							b.Fatalf("decoder output mismatch: %v", err)
						}
						b.ReportAllocs()
						b.SetBytes(int64(len(data)))
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							got, err := implementation.read(b.Context(), resource)
							if err != nil || len(got) != len(want) {
								b.Fatalf("read failed: %v", err)
							}
						}
					})
				}
			})
		}
	}
}
