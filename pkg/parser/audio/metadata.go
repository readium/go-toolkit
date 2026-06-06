package audio

import (
	"strings"

	"github.com/dhowden/tag"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
)

// audioTags holds the metadata extracted from a single audio file's tags
// (ID3, MP4 atoms, Vorbis comments, …).
type audioTags struct {
	Title       string
	Album       string
	Artist      string
	AlbumArtist string
	Composer    string
	Genre       string
	Comment     string
	Year        int
	Picture     *tag.Picture
	Raw         map[string]any
}

// readAudioTags reads the embedded tags of an audio resource. It returns nil
// when no tags could be parsed (which is not an error: many audio files are
// untagged).
func readAudioTags(res fetcher.Resource) *audioTags {
	m, err := tag.ReadFrom(fetcher.NewResourceReadSeeker(res))
	if err != nil {
		return nil
	}
	return &audioTags{
		Title:       strings.TrimSpace(m.Title()),
		Album:       strings.TrimSpace(m.Album()),
		Artist:      strings.TrimSpace(m.Artist()),
		AlbumArtist: strings.TrimSpace(m.AlbumArtist()),
		Composer:    strings.TrimSpace(m.Composer()),
		Genre:       strings.TrimSpace(m.Genre()),
		Comment:     strings.TrimSpace(m.Comment()),
		Year:        m.Year(),
		Picture:     m.Picture(),
		Raw:         m.Raw(),
	}
}

// applyTagsToMetadata enriches the publication metadata using the tags of the
// first audio file. Only fields that are present in the tags are set, so an
// untagged file leaves the manifest untouched.
//
// For audiobooks the album typically holds the book title while the per-track
// title holds the chapter name, so the album is preferred for the publication
// title.
func applyTagsToMetadata(m *manifest.Metadata, t *audioTags) {
	if t == nil {
		return
	}

	if title := firstNonEmpty(t.Album, t.Title); title != "" {
		m.LocalizedTitle = manifest.NewLocalizedStringFromString(title)
		if m.Type == "" {
			m.Type = "http://schema.org/Audiobook"
		}
	}

	if author := firstNonEmpty(t.Artist, t.Composer); author != "" {
		m.Authors = []manifest.Contributor{{
			LocalizedName: manifest.NewLocalizedStringFromString(author),
		}}
	}

	// When the album artist differs from the author, treat it as the narrator,
	// which is the usual convention for audiobooks.
	if t.AlbumArtist != "" && t.AlbumArtist != t.Artist {
		m.Narrators = []manifest.Contributor{{
			LocalizedName: manifest.NewLocalizedStringFromString(t.AlbumArtist),
		}}
	}

	if t.Comment != "" {
		m.Description = t.Comment
	} else if d := rawString(t.Raw, "description"); d != "" {
		m.Description = d
	}

	if t.Genre != "" {
		m.Subjects = []manifest.Subject{{
			LocalizedName: manifest.NewLocalizedStringFromString(t.Genre),
		}}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func rawString(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	if v, ok := raw[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
