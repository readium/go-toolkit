package serve

import (
	"github.com/gorilla/mux"
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
}

func NewServer(config ServerConfig) *Server {
	return &Server{
		config: config,
	}
}
