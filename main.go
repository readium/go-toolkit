package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	"github.com/feedbooks/r2-streamer-go/fetcher"
	"github.com/feedbooks/r2-streamer-go/models"
	"github.com/feedbooks/r2-streamer-go/parser"
	"github.com/feedbooks/r2-streamer-go/searcher"
	"github.com/gorilla/mux"
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
	serv.HandleFunc("/{filename}/search", search)
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
	http.ServeContent(w, req, assetname, time.Now(), epubReader)
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
	searchReturn, err := searcher.Search(publication, searchTerm)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	j, _ := json.Marshal(searchReturn)
	json.Indent(&returnJSON, j, "", "  ")
	returnJSON.WriteTo(w)
}

func getPublication(filename string, req *http.Request) (models.Publication, error) {
	var current currentBook
	var publication models.Publication
	var err error

	for _, book := range currentBookList {
		if filename == book.filename {
			current = book
		}
	}

	if current.filename == "" {
		manifestURL := "http://" + req.Host + "/" + filename + "/manifest.json"
		filenamePath, _ := base64.StdEncoding.DecodeString(filename)

		publication, err = parser.Parse(string(filenamePath))
		hasMediaOverlay := false
		for i, l := range publication.Resources {
			if l.Properties != nil && l.Properties.MediaOverlay != "" {
				publication.Resources[i].Properties.MediaOverlay = strings.Replace(publication.Resources[i].Properties.MediaOverlay, "{url}", "/"+filename+"/media-overlay?resource=", 1)
				hasMediaOverlay = true
			}
		}
		for i, l := range publication.Spine {
			if l.Properties != nil && l.Properties.MediaOverlay != "" {
				publication.Spine[i].Properties.MediaOverlay = strings.Replace(publication.Spine[i].Properties.MediaOverlay, "{url}", "/"+filename+"/media-overlay?resource=", 1)
				hasMediaOverlay = true
			}
		}

		if err != nil {
			return models.Publication{}, err
		}
		publication.AddLink("application/vnd.readium.mo+json", []string{"media-overlay"}, "http://"+req.Host+"/"+filename+"/media-overlay?resource={path}", true)
		if hasMediaOverlay {
			publication.AddLink("application/webpub+json", []string{"self"}, manifestURL, false)
		}
		if searcher.CanBeSearch(publication) {
			publication.AddLink("", []string{"search"}, "http://"+req.Host+"/"+filename+"/search?query={searchTerms}", true)
		}
		current = currentBook{filename: filename, publication: publication, timestamp: time.Now(), indexed: false}
		currentBookList = append(currentBookList, current)
		if searcher.CanBeSearch(publication) {
			go indexBook(publication)
		}
	} else {
		publication = current.publication
		if searcher.CanBeSearch(publication) {
			go indexBook(publication)
		}
	}

	return publication, nil
}

func indexBook(publication models.Publication) {
	searcher.Index(publication)
}
