package cache

import (
	"github.com/readium/go-toolkit/pkg/pub"
)

// CachedPublication implements Evictable
type CachedPublication struct {
	*pub.Publication
	Remote bool
}

func EncapsulatePublication(pub *pub.Publication, remote bool) *CachedPublication {
	return &CachedPublication{pub, remote}
}

func (cp *CachedPublication) OnEvict() {
	// Cleanup
	if cp.Publication != nil {
		cp.Publication.Close()
	}
}
