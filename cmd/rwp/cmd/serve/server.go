package serve

import (
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/mux"
	"github.com/readium/go-toolkit/cmd/rwp/cmd/serve/cache"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/streamer"
)

type Remote struct {
	S3     *s3.Client      // AWS S3-compatible storage
	GCS    *storage.Client // Google Cloud Storage
	HTTP   *http.Client    // HTTP-requested storage
	Config archive.RemoteArchiveConfig
}

type ServerConfig struct {
	Debug             bool
	BaseDirectory     string
	JSONIndent        string
	InferA11yMetadata streamer.InferA11yMetadata
}

type Server struct {
	config ServerConfig
	remote Remote
	router *mux.Router
	lfu    *cache.TinyLFU
}

const MaxCachedPublicationAmount = 10
const MaxCachedPublicationTTL = time.Second * time.Duration(600)

func NewServer(config ServerConfig, remote Remote) *Server {
	return &Server{
		config: config,
		remote: remote,
		lfu:    cache.NewTinyLFU(MaxCachedPublicationAmount, MaxCachedPublicationTTL),
	}
}
