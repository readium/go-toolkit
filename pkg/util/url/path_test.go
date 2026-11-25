package url

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLFromPath(t *testing.T) {
	for path, expected := range map[string]string{
		"chapter.xhtml":         "chapter.xhtml",
		"OEBPS/chapter 1.xhtml": "OEBPS/chapter%201.xhtml",
		"é/ü.txt":               "%C3%A9/%C3%BC.txt",
		"../a b.css":            "../a%20b.css",
		// Brackets aren't allowed in URL paths, though Go keeps them.
		"[x].jpg": "%5Bx%5D.jpg",
		// Delimiters are encoded, rather than taken for escapes, queries,
		// fragments or schemes.
		"page%20one.txt": "page%2520one.txt",
		"100%.png":       "100%25.png",
		"a#b.txt":        "a%23b.txt",
		"a?b.txt":        "a%3Fb.txt",
		"c:file.txt":     "./c:file.txt",
	} {
		u, err := URLFromPath(path)
		require.NoError(t, err, path)
		assert.Equal(t, expected, u.String(), path)
		assert.Equal(t, path, u.Path(), path)
		parsed, err := URLFromString(u.String())
		require.NoError(t, err, path)
		assert.Equal(t, path, parsed.Normalize().Path(), "%s round trip", path)
	}

	// A path starting with "//" would be taken for a host.
	_, err := URLFromPath("//cdn.example.com/x")
	assert.Error(t, err)
}

func TestNormalizedPathEscapes(t *testing.T) {
	for _, prefix := range []string{"", "https://example.org/"} {
		u := MustURLFromString(prefix + "a%2fb/%70age")
		assert.Equal(t, prefix+"a%2Fb/page", u.Normalize().String())
	}
}
