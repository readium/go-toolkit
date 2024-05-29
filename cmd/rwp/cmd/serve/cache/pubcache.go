package cache

import (
	"github.com/readium/go-toolkit/pkg/pub"
)

// CachedPublication implements Evictable
type CachedPublication struct {
	*pub.Publication
}

func EncapsulatePublication(pub *pub.Publication) *CachedPublication {
	cp := &CachedPublication{pub}
	return cp
}

func (cp *CachedPublication) OnEvict() {
	// Cleanup
	if cp.Publication != nil {
		cp.Publication.Close()
	}
}
