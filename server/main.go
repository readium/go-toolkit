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
	"sync"
	"time"

	"github.com/feedbooks/webpub-streamer/fetcher"
	"github.com/feedbooks/webpub-streamer/models"
	"github.com/feedbooks/webpub-streamer/parser"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

type currentBook struct {
	filename    string
	publication models.Publication
	timestamp   time.Time
}

var currentBookList []currentBook
var zipMutex sync.Mutex

// Serv TODO add doc
func main() {

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
	serv.HandleFunc("/{filename}/{asset:.*}", getAsset)
	return serv
}

func getManifest(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	filename := vars["filename"]

	publication := getPublication(filename, req)

	j, _ := json.Marshal(publication)

	var identJSON bytes.Buffer
	json.Indent(&identJSON, j, "", " ")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	identJSON.WriteTo(w)
	return
}

func getAsset(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	assetname := vars["asset"]

	publication := getPublication(vars["filename"], req)
	zipMutex.Lock()
	epubReader, mediaType := fetcher.Fetch(publication, assetname)
	zipMutex.Unlock()
	runtime.Gosched()

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeContent(w, req, assetname, time.Now(), epubReader)
	return

}

func getPublication(filename string, req *http.Request) models.Publication {
	var current currentBook
	var publication models.Publication

	for _, book := range currentBookList {
		if filename == book.filename {
			current = book
		}
	}

	if current.filename == "" {
		manifestURL := "http://" + req.Host + "/" + filename + "/manifest.json"
		filenamePath, _ := base64.StdEncoding.DecodeString(filename)

		publication = parser.Parse(string(filenamePath), manifestURL)

		currentBookList = append(currentBookList, currentBook{filename: filename, publication: publication, timestamp: time.Now()})
	} else {
		publication = current.publication
	}

	return publication
}
