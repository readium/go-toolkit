package serve

import (
	"time"

	"github.com/gorilla/mux"
	"github.com/readium/go-toolkit/cmd/rwp/cmd/serve/cache"
	"github.com/readium/go-toolkit/pkg/streamer"
)

type ServerConfig struct {
	Debug             bool
	BaseDirectory     string
	JSONIndent        string
	InferA11yMetadata streamer.InferA11yMetadata
}

type Server struct {
	config ServerConfig
	router *mux.Router
	lfu    *cache.TinyLFU
}

const MaxCachedPublicationAmount = 10
const MaxCachedPublicationTTL = time.Second * time.Duration(600)

func NewServer(config ServerConfig) *Server {
	return &Server{
		config: config,
		lfu:    cache.NewTinyLFU(MaxCachedPublicationAmount, MaxCachedPublicationTTL),
	}
}
