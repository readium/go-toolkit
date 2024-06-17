package serve

import (
	"net/http"
	"net/http/pprof"

	"github.com/CAFxX/httpcompression"
	"github.com/gorilla/mux"
)

func (s *Server) Routes() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	if s.config.Debug {
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)

		r.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
		r.Handle("/debug/pprof/block", pprof.Handler("block"))
		r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		r.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
		r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	}

	r.HandleFunc("/list.json", s.demoList).Name("demo_list")

	pub := r.PathPrefix("/{path}").Subrouter()
	// TODO: publication loading middleware with pub.Use()
	pub.Use(func(h http.Handler) http.Handler {
		adapter, _ := httpcompression.DefaultAdapter(httpcompression.ContentTypes(compressableMimes, false))
		return adapter(h)
	})
	pub.HandleFunc("/manifest.json", s.getManifest).Name("manifest")
	pub.HandleFunc("/{asset:.*}", s.getAsset).Name("asset")

	s.router = r
	return r
}
