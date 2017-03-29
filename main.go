package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/gorilla/mux"
	"github.com/readium/r2-streamer-go/decoder/lcp"
	"github.com/readium/r2-streamer-go/fetcher"
	"github.com/readium/r2-streamer-go/models"
	"github.com/readium/r2-streamer-go/parser"
	"github.com/readium/r2-streamer-go/searcher"
	"github.com/urfave/negroni"
)

type currentBook struct {
	filename    string
	publication models.Publication
	timestamp   time.Time
	bleveIndex  bleve.Index
	indexed     bool
}

var currentBookList []currentBook
var zipMutex sync.Mutex

// Serv TODO add doc
func main() {

	if len(os.Args) < 2 {
		fmt.Println("missing filename")
		os.Exit(1)
	}

	filename := os.Args[1]

	n := negroni.Classic()
	n.Use(negroni.NewStatic(http.Dir("public")))
	n.UseHandler(bookHandler(false))

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}

	freePort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	s := &http.Server{
		Handler:        n,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Addr:           "localhost:" + strconv.Itoa(freePort),
	}

	filenamePath := base64.StdEncoding.EncodeToString([]byte(filename))
	fmt.Println("http://localhost:" + strconv.Itoa(freePort) + "/" + filenamePath + "/manifest.json")

	log.Fatal(s.ListenAndServe())
}

func bookHandler(test bool) http.Handler {
	serv := mux.NewRouter()

	serv.HandleFunc("/{filename}/manifest.json", getManifest)
	serv.HandleFunc("/{filename}/license-handler.json", pushPassphrase)
	serv.HandleFunc("/{filename}/license.lcpl", getLCPLicense)
	serv.HandleFunc("/{filename}/search", search)
	serv.HandleFunc("/{filename}/media-overlay", mediaOverlay)
	serv.HandleFunc("/{filename}/{asset:.*}", getAsset)
	return serv
}

func getManifest(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	filename := vars["filename"]

	publication, err := getPublication(filename, req)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	j, _ := json.Marshal(publication)

	var identJSON bytes.Buffer

	json.Indent(&identJSON, j, "", " ")
	w.Header().Set("Content-Type", "application/webpub+json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	hashJSONRaw := sha256.Sum256(identJSON.Bytes())
	hashJSON := base64.RawStdEncoding.EncodeToString(hashJSONRaw[:])

	if match := req.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, hashJSON) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Etag", hashJSON)

	links := publication.GetPreFetchResources()
	if len(links) > 0 {
		prefetch := ""
		for _, l := range links {
			prefetch = prefetch + "<" + l.Href + ">;" + "rel=prefetch,"
		}
		w.Header().Set("Link", prefetch)
	}

	identJSON.WriteTo(w)
	return
}

func getAsset(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	assetname := vars["asset"]

	publication, err := getPublication(vars["filename"], req)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	zipMutex.Lock()
	epubReader, mediaType, err := fetcher.Fetch(publication, assetname)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	zipMutex.Unlock()
	runtime.Gosched()

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public,max-age=86400")
	http.ServeContent(w, req, assetname, time.Time{}, epubReader)
	return

}

func search(w http.ResponseWriter, req *http.Request) {
	var returnJSON bytes.Buffer
	vars := mux.Vars(req)

	publication, err := getPublication(vars["filename"], req)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	searchTerm := req.URL.Query().Get("query")
	searchReturn, err := searcher.Search(*publication, searchTerm)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	j, _ := json.Marshal(searchReturn)
	json.Indent(&returnJSON, j, "", "  ")
	returnJSON.WriteTo(w)
}

func getLCPLicense(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	publication, err := getPublication(vars["filename"], req)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	data := publication.GetLCPJSON()
	if string(data) == "" {
		w.WriteHeader(404)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.readium.lcp.license-1.0+json")
	w.Write(data)
}

func pushPassphrase(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	publication, err := getPublication(vars["filename"], req)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	data, err := publication.GetLCPHandlerInfo()
	if err != nil {
		w.WriteHeader(500)
		return
	}

	if req.Method == http.MethodPost {
		var postInfo models.LCPHandlerPost
		buff, errRead := ioutil.ReadAll(req.Body)
		if errRead != nil {
			fmt.Println("can't read body")
			w.WriteHeader(401)
		} else {
			errUnMarsh := json.Unmarshal(buff, &postInfo)
			if errUnMarsh != nil {
				fmt.Println("can't unmarshal " + errUnMarsh.Error())
				w.WriteHeader(401)
			} else {
				key, _ := base64.StdEncoding.DecodeString(postInfo.Key.Hash)
				publication.AddLCPHash(key)
				if lcp.HasGoodKey(publication) == false {
					w.WriteHeader(401)
				} else {
					data.Key.Ready = true
					updatePublication(*publication, vars["filename"])
				}
			}
		}
	}

	j, _ := json.Marshal(data)

	var identJSON bytes.Buffer
	json.Indent(&identJSON, j, "", " ")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	identJSON.WriteTo(w)
	return
}

func mediaOverlay(w http.ResponseWriter, req *http.Request) {
	var returnJSON bytes.Buffer
	var media []models.MediaOverlayNode

	vars := mux.Vars(req)
	var mediaOverlay struct {
		MediaOverlay []models.MediaOverlayNode `json:"media-overlay"`
	}

	publication, err := getPublication(vars["filename"], req)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	resource := req.URL.Query().Get("resource")
	if resource == "" {
		media = publication.FindAllMediaOverlay()

	} else {
		media = publication.FindMediaOverlayByHref(resource)
	}

	mediaOverlay.MediaOverlay = media
	j, _ := json.Marshal(mediaOverlay)
	json.Indent(&returnJSON, j, "", "  ")
	w.Header().Set("Content-Type", "application/vnd.readium.mo+json")
	returnJSON.WriteTo(w)
}

func getPublication(filename string, req *http.Request) (*models.Publication, error) {
	var current currentBook

	for _, book := range currentBookList {
		if filename == book.filename {
			current = book
		}
	}

	if current.filename == "" {
		manifestURL := "http://" + req.Host + "/" + filename + "/manifest.json"
		filenamePath, _ := base64.StdEncoding.DecodeString(filename)

		publication, err := parser.Parse(string(filenamePath))
		hasMediaOverlay := false
		for _, l := range publication.Spine {
			if l.Properties != nil && l.Properties.MediaOverlay != "" {
				hasMediaOverlay = true
			}
		}

		if err != nil {
			return &models.Publication{}, err
		}

		publication.AddLink("application/webpub+json", []string{"self"}, manifestURL, false)
		if hasMediaOverlay {
			publication.AddLink("application/vnd.readium.mo+json", []string{"media-overlay"}, "http://"+req.Host+"/"+filename+"/media-overlay?resource={path}", true)
		}
		if searcher.CanBeSearch(publication) {
			publication.AddLink("", []string{"search"}, "http://"+req.Host+"/"+filename+"/search?query={searchTerms}", true)
		}
		current = currentBook{filename: filename, publication: publication, timestamp: time.Now(), indexed: false}
		currentBookList = append(currentBookList, current)
		// if searcher.CanBeSearch(publication) {
		// 	go indexBook(publication)
		// }
		return &publication, nil
	}
	return &current.publication, nil
	// if searcher.CanBeSearch(publication) {
	// 	go indexBook(publication)
	// }
}

func updatePublication(publicaton models.Publication, filename string) {
	for i, book := range currentBookList {
		if filename == book.filename {
			currentBookList[i].publication = publicaton
		}
	}

}

// func indexBook(publication models.Publication) {
// 	searcher.Index(publication)
// }
