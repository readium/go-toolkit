package api

type ServerConfig struct {
	Bind      string   // The address to listen on
	Origins   []string // The CORS origins allowed (not yet implemented)
	SentryDSN string   // Sentry DSN (not yet implemented)
	CacheDSN  string   // Cache DSN (not yet implemented)
	// TODO replace PublicationPath with storage interface DSNs
	PublicationPath string // Filesystem path leading to stored publications
	StaticPath      string // Filesystem path leading to static assets to be served
}
