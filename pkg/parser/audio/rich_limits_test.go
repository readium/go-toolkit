package audio

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaylistTOCSkipsOversizedPlaylists(t *testing.T) {
	track := manifest.Link{Href: manifest.MustNewHREFFromString("track1.mp3", false)}
	toc := func(playlist []byte) manifest.LinkList {
		path := filepath.Join(t.TempDir(), "audiobook.zip")
		out, err := os.Create(path)
		require.NoError(t, err)
		zw := zip.NewWriter(out)
		for name, data := range map[string][]byte{"playlist.m3u": playlist, "track1.mp3": []byte("audio")} {
			w, err := zw.Create(name)
			require.NoError(t, err)
			_, err = w.Write(data)
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())
		require.NoError(t, out.Close())
		f, err := fetcher.NewArchiveFetcherFromPath(t.Context(), path)
		require.NoError(t, err)
		defer f.Close()
		return playlistTOC(t.Context(), f, manifest.LinkList{track})
	}

	assert.NotEmpty(t, toc([]byte("track1.mp3\n")))
	// An oversized playlist was read whole, however much it inflated.
	assert.Empty(t, toc(append([]byte("track1.mp3\n"), bytes.Repeat([]byte("#\n"), maxPlaylistSize)...)))
}
