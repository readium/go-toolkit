package serve

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
	httprange "github.com/gotd/contrib/http_range"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/cmd/rwp/cmd/serve/cache"
	"github.com/readium/go-toolkit/pkg/archive"
	"github.com/readium/go-toolkit/pkg/asset"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/pub"
	"github.com/readium/go-toolkit/pkg/streamer"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/zeebo/xxh3"
)

type demoListItem struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

func (s *Server) demoList(w http.ResponseWriter, req *http.Request) {
	fi, err := os.ReadDir(s.config.BaseDirectory)
	if err != nil {
		slog.Error("failed reading publications directory", "error", err)
		w.WriteHeader(500)
		return
	}
	files := make([]demoListItem, len(fi))
	for i, f := range fi {
		files[i] = demoListItem{
			Filename: f.Name(),
			Path:     base64.RawURLEncoding.EncodeToString([]byte(f.Name())),
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", s.config.JSONIndent)
	enc.Encode(files)
}

func (s *Server) getPublication(ctx context.Context, filename string) (*pub.Publication, bool, error) {
	fpath, err := base64.RawURLEncoding.DecodeString(filename)
	if err != nil {
		return nil, false, err
	}
	loc, err := url.URLFromString(string(fpath))
	if err != nil {
		return nil, false, errors.Wrap(err, "failed creating URL from filepath")
	}
	u := url.BaseFile.Resolve(loc).(url.AbsoluteURL) // Turn relative filepaths into file:/// URLs

	dat, ok := s.lfu.Get(u.String())
	if !ok {
		var pub *pub.Publication
		var remote bool
		config := streamer.Config{
			InferA11yMetadata: s.config.InferA11yMetadata,
			HttpClient:        s.remote.HTTP,
		}
		if u.IsFile() {
			path, err := url.FromFilepath(filepath.Join(s.config.BaseDirectory, path.Clean(u.Path())))
			if err != nil {
				return nil, remote, errors.Wrap(err, "failed creating URL from filepath")
			}

			pub, err = streamer.New(config).Open(ctx, asset.File(path), "")
			if err != nil {
				return nil, remote, errors.Wrap(err, "failed opening "+path.String())
			}
		} else {
			switch u.Scheme() {
			case url.SchemeS3:
				remote = true
				if s.remote.S3 == nil {
					return nil, remote, errors.New("S3 client not configured")
				}
				config.ArchiveFactory = archive.NewS3ArchiveFactory(s.remote.S3, archive.NewDefaultRemoteArchiveConfig())
				pub, err = streamer.New(config).Open(ctx, asset.S3(s.remote.S3, u), "")
				if err != nil {
					return nil, remote, errors.Wrap(err, "failed opening "+u.String())
				}
			case url.SchemeGS:
				remote = true
				if s.remote.GCS == nil {
					return nil, remote, errors.New("GCS client not configured")
				}
				config.ArchiveFactory = archive.NewGCSArchiveFactory(s.remote.GCS, archive.NewDefaultRemoteArchiveConfig())
				pub, err = streamer.New(config).Open(ctx, asset.GCS(s.remote.GCS, u), "")
				if err != nil {
					return nil, remote, errors.Wrap(err, "failed opening "+u.String())
				}
			case url.SchemeHTTP, url.SchemeHTTPS:
				remote = true
				if s.remote.HTTP == nil {
					return nil, remote, errors.New("HTTP client not configured")
				}
				config.ArchiveFactory = archive.NewHTTPArchiveFactory(s.remote.HTTP, archive.NewDefaultRemoteArchiveConfig())
				pub, err = streamer.New(config).Open(ctx, asset.HTTP(s.remote.HTTP, u), "")
				if err != nil {
					return nil, remote, errors.Wrap(err, "failed opening "+u.String())
				}
			default:
				return nil, remote, errors.New("unsupported scheme " + u.Scheme().String())
			}
		}

		// Cache the publication
		encPub := cache.EncapsulatePublication(pub, remote)
		s.lfu.Set(u.String(), encPub)

		return encPub.Publication, remote, nil
	}
	cp := dat.(*cache.CachedPublication)
	return cp.Publication, cp.Remote, nil
}

func (s *Server) getManifest(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	filename := vars["path"]

	// Load the publication
	publication, _, err := s.getPublication(req.Context(), filename)
	if err != nil {
		slog.Error("failed opening publication", "error", err)
		w.WriteHeader(500)
		if s.config.Debug {
			w.Write([]byte(err.Error()))
		}
		return
	}

	// Create "self" link in manifest
	scheme := "http://"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		// Note: this is never going to be 100% accurate behind proxies,
		// but it's better than nothing for a dev server.
		scheme = "https://"
	}
	rPath, _ := s.router.Get("manifest").URLPath("path", vars["path"])
	conformsTo := conformsToAsMimetype(publication.Manifest.Metadata.ConformsTo)

	selfUrl, err := url.AbsoluteURLFromString(scheme + req.Host + rPath.String())
	if err != nil {
		slog.Error("failed creating self URL", "error", err)
		w.WriteHeader(500)
		if s.config.Debug {
			w.Write([]byte(err.Error()))
		}
		return
	}

	selfLink := &manifest.Link{
		Rels:      manifest.Strings{"self"},
		MediaType: &conformsTo,
		Href:      manifest.NewHREF(selfUrl),
	}

	// Marshal the manifest
	j, err := json.Marshal(publication.Manifest.ToMap(selfLink))
	if err != nil {
		slog.Error("failed marshalling manifest JSON", "error", err)
		w.WriteHeader(500)
		if s.config.Debug {
			w.Write([]byte(err.Error()))
		}
		return
	}

	// Indent JSON
	var identJSON bytes.Buffer
	if s.config.JSONIndent == "" {
		_, err = identJSON.Write(j)
		if err != nil {
			slog.Error("failed writing manifest JSON to buffer", "error", err)
			w.WriteHeader(500)
			if s.config.Debug {
				w.Write([]byte(err.Error()))
			}
			return
		}
	} else {
		err = json.Indent(&identJSON, j, "", s.config.JSONIndent)
		if err != nil {
			slog.Error("failed indenting manifest JSON", "error", err)
			w.WriteHeader(500)
			if s.config.Debug {
				w.Write([]byte(err.Error()))
			}
			return
		}
	}

	// Add headers
	w.Header().Set("content-type", conformsTo.String()+"; charset=utf-8")
	w.Header().Set("cache-control", "private, must-revalidate")
	w.Header().Set("access-control-allow-origin", "*") // TODO: provide options?

	// Etag based on hash of the manifest bytes
	etag := `"` + strconv.FormatUint(xxh3.Hash(identJSON.Bytes()), 36) + `"`
	w.Header().Set("Etag", etag)
	if match := req.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Write response body
	_, err = identJSON.WriteTo(w)
	if err != nil {
		slog.Error("failed writing manifest JSON to response writer", "error", err)
		w.WriteHeader(500)
		if s.config.Debug {
			w.Write([]byte(err.Error()))
		}
		return
	}
}

func (s *Server) getAsset(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	filename := vars["path"]

	// Load the publication
	publication, remote, err := s.getPublication(r.Context(), filename)
	if err != nil {
		slog.Error("failed opening publication", "error", err)
		w.WriteHeader(500)
		if s.config.Debug {
			w.Write([]byte(err.Error()))
		}
		return
	}

	// Parse asset path from mux vars
	href, err := url.URLFromDecodedPath(path.Clean(vars["asset"]))
	if err != nil {
		slog.Error("failed parsing asset path as URL", "error", err)
		w.WriteHeader(400)
		if s.config.Debug {
			w.Write([]byte(err.Error()))
		}
		return
	}
	rawHref := href.Raw()
	rawHref.RawQuery = r.URL.Query().Encode() // Add the query parameters of the URL
	href, _ = url.RelativeURLFromGo(rawHref)  // Turn it back into a go-toolkit relative URL

	// Make sure the asset exists in the publication
	link := publication.LinkWithHref(href)
	if link == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	finalLink := *link

	// Expand templated links to include URL query parameters
	if finalLink.Href.IsTemplated() {
		finalLink.Href = manifest.NewHREF(finalLink.URL(nil, convertURLValuesToMap(r.URL.Query())))
	}

	// Get the asset from the publication
	res := publication.Get(r.Context(), finalLink)
	defer res.Close()

	// Get asset length in bytes
	l, rerr := res.Length(r.Context())
	if rerr != nil {
		w.WriteHeader(rerr.HTTPStatus())
		w.Write([]byte(rerr.Error()))
		return
	}

	// Patch mimetype where necessary
	contentType := link.MediaType.String()
	if sub, ok := mimeSubstitutions[contentType]; ok {
		contentType = sub
	}
	if slices.Contains(utfCharsetNeeded, contentType) {
		contentType += "; charset=utf-8"
	}
	w.Header().Set("content-type", contentType)
	w.Header().Set("cache-control", "private, max-age=86400, immutable")
	w.Header().Set("content-length", strconv.FormatInt(l, 10))
	w.Header().Set("access-control-allow-origin", "*") // TODO: provide options?

	var start, end int64
	// Range reading assets
	rangeHeader := r.Header.Get("range")
	if rangeHeader != "" {
		rng, err := httprange.ParseRange(rangeHeader, l)
		if err != nil {
			slog.Error("failed parsing range header", "error", err)
			w.WriteHeader(http.StatusLengthRequired)
			return
		}
		if len(rng) > 1 {
			slog.Error("no support for multiple read ranges")
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		if len(rng) > 0 {
			w.Header().Set("content-range", rng[0].ContentRange(l))
			start = rng[0].Start
			end = start + rng[0].Length - 1
			w.Header().Set("content-length", strconv.FormatInt(rng[0].Length, 10))
		}
	}
	if w.Header().Get("content-range") != "" {
		w.WriteHeader(http.StatusPartialContent)
	}

	cres, ok := res.(fetcher.CompressedResource)
	normalResponse := func() {
		if remote {
			var bin []byte
			bin, rerr = res.Read(r.Context(), start, end)
			if rerr == nil {
				_, err = w.Write(bin)
				if err != nil {
					rerr = fetcher.Other(err)
				}
			}
		} else {
			_, rerr = res.Stream(r.Context(), w, start, end)
		}
	}
	if ok && cres.CompressedAs(archive.CompressionMethodDeflate) && start == 0 && end == 0 {
		// Stream the asset in compressed format if supported by the user agent
		if supportsEncoding(r, "deflate") {
			headers := func() {
				w.Header().Set("content-encoding", "deflate")
				w.Header().Set("content-length", strconv.FormatInt(cres.CompressedLength(r.Context()), 10))
			}
			if remote {
				var bin []byte
				bin, rerr = cres.ReadCompressed(r.Context())
				if rerr == nil {
					headers()
					_, err = w.Write(bin)
					if err != nil {
						rerr = fetcher.Other(err)
					}
				}
			} else {
				headers()
				_, rerr = cres.StreamCompressed(r.Context(), w)
			}
		} else if supportsEncoding(r, "gzip") && l <= archive.GzipMaxLength {
			headers := func() {
				w.Header().Set("content-encoding", "gzip")
				w.Header().Set("content-length", strconv.FormatInt(cres.CompressedLength(r.Context())+archive.GzipWrapperLength, 10))
			}
			if remote {
				var bin []byte
				bin, rerr = cres.ReadCompressedGzip(r.Context())
				if rerr == nil {
					headers()
					_, err = w.Write(bin)
					if err != nil {
						rerr = fetcher.Other(err)
					}
				}
			} else {
				headers()
				_, rerr = cres.StreamCompressedGzip(r.Context(), w)
			}
		} else {
			normalResponse()
		}
	} else {
		normalResponse()
	}

	if rerr != nil {
		if errors.Is(rerr.Cause, syscall.EPIPE) || errors.Is(rerr.Cause, syscall.ECONNRESET) {
			// Ignore client errors
			return
		}

		slog.Error("failed streaming asset", "error", rerr.Error())
	}

}
