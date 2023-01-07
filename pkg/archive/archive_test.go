package archive

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var archives = []string{"./testdata/epub.epub", "./testdata/epub"}

var entryList = []string{
	"mimetype",
	"EPUB/cover.xhtml",
	"EPUB/css/epub.css",
	"EPUB/css/nav.css",
	"EPUB/images/cover.png",
	"EPUB/nav.xhtml",
	"EPUB/package.opf",
	"EPUB/s04.xhtml",
	"EPUB/toc.ncx",
	"META-INF/container.xml",
}

// Helper to make it easy to run the same test with different archives
func withArchives(t *testing.T, callback func(archive Archive)) {
	for _, archivePath := range archives {
		t.Log(archivePath)
		archive, err := DefaultArchiveFactory{}.Open(archivePath, "")
		assert.NoError(t, err)
		callback(archive)
	}
}

func TestArchiveOpens(t *testing.T) {
	withArchives(t, func(archive Archive) {
		// Do Nothing
	})
}

func TestArchiveEntryListCorrect(t *testing.T) {
	withArchives(t, func(archive Archive) {
		entries := archive.Entries()
		for _, ele := range entryList {
			contains := false
			for _, entry := range entries {
				if entry.Path() == ele {
					contains = true
					break
				}
			}
			assert.True(t, contains, "archive should contain entry "+ele)
		}
	})
}

func TestArchiveMissingEntry(t *testing.T) {
	withArchives(t, func(archive Archive) {
		_, err := archive.Entry("unknown")
		assert.Error(t, err, "archive shouldn't contain an \"unknown\" entry")
	})
}

func TestArchiveFullReading(t *testing.T) {
	withArchives(t, func(archive Archive) {
		entry, err := archive.Entry("mimetype")
		if assert.NoError(t, err) {
			b, err := entry.Read(0, 0)
			if assert.NoError(t, err) {
				assert.Equal(t, "application/epub+zip", string(b))
			}

			var tmp bytes.Buffer
			n, err := entry.Stream(&tmp, 0, 0)
			if assert.NoError(t, err) {
				assert.EqualValues(t, 20, n)
				assert.Equal(t, "application/epub+zip", tmp.String())
			}
		}
	})
}

func TestArchivePartialReading(t *testing.T) {
	withArchives(t, func(archive Archive) {
		entry, err := archive.Entry("mimetype")
		if assert.NoError(t, err) {
			b, err := entry.Read(0, 10)
			if assert.NoError(t, err) {
				assert.Equal(t, "application", string(b))
				assert.Equal(t, 11, len(b))
			}

			var tmp bytes.Buffer
			n, err := entry.Stream(&tmp, 0, 10)
			if assert.NoError(t, err) {
				assert.EqualValues(t, 11, n)
				s := tmp.String()
				assert.Equal(t, "application", s)
				assert.Equal(t, 11, len(s))
			}
		}
	})
}

func TestArchiveOutOfRangeClamping(t *testing.T) {
	withArchives(t, func(archive Archive) {
		entry, err := archive.Entry("mimetype")
		if assert.NoError(t, err) {
			b, err := entry.Read(-5, 60)
			if assert.NoError(t, err) {
				assert.Equal(t, "application/epub+zip", string(b))
				assert.Equal(t, 20, len(b))
			}

			var tmp bytes.Buffer
			n, err := entry.Stream(&tmp, -5, 60)
			if assert.NoError(t, err) {
				assert.EqualValues(t, 20, n)
				s := tmp.String()
				assert.Equal(t, "application/epub+zip", s)
				assert.Equal(t, 20, len(s))
			}
		}
	})
}

func TestArchiveDecreasingRange(t *testing.T) {
	withArchives(t, func(archive Archive) {
		entry, err := archive.Entry("mimetype")
		if assert.NoError(t, err) {
			_, err = entry.Read(60, 20)
			assert.Error(t, err, "decreasing ranges are not satisfiable")

			var tmp bytes.Buffer
			_, err = entry.Stream(&tmp, 60, 20)
			assert.Error(t, err, "decreasing ranges are not satisfiable")
		}
	})
}

func TestArchiveEntrySize(t *testing.T) {
	withArchives(t, func(archive Archive) {
		entry, err := archive.Entry("mimetype")
		if assert.NoError(t, err) {
			assert.EqualValues(t, 20, entry.Length())
		}
	})
}
