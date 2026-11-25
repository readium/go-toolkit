//go:build unix

package fetcher

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileFetcherNamedPipes(t *testing.T) {
	pub := symlinkTree(t)
	require.NoError(t, syscall.Mkfifo(filepath.Join(pub, "fifo"), 0o644))
	f := NewFileFetcher("", pub)
	defer f.Close()

	// Opening the pipe would block forever.
	done := make(chan struct{})
	go func() {
		defer close(done)
		links, err := f.Links(t.Context())
		assert.NoError(t, err)
		for _, l := range links {
			assert.NotEqual(t, "fifo", l.Href.String())
		}
		_, rerr := f.Get(t.Context(), manifest.Link{Href: manifest.MustNewHREFFromString("fifo", false)}).Read(t.Context(), 0, 0)
		if assert.NotNil(t, rerr) {
			assert.Equal(t, CodeNotFound, rerr.Code)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("blocked on the named pipe")
	}
}
