package r2go

import (
	"time"

	"github.com/blevesearch/bleve"
	"github.com/readium/r2-streamer-go/pkg/pub"
)

type R2GoConfig struct {
	Bind      string   // The address to listen on
	Origins   []string // The CORS origins allowed (not yet implemented)
	SentryDSN string   // Sentry DSN (not yet implemented)
	CacheDSN  string   // Cache DSN (not yet implemented)
	// TODO replace PublicationPath with storage interface DSNs
	PublicationPath string // Filesystem path leading to stored publications
	StaticPath      string // Filesystem path leading to static assets to be served
}

type currentBook struct {
	filename    string
	publication pub.Publication
	timestamp   time.Time
	bleveIndex  bleve.Index
	indexed     bool
}
