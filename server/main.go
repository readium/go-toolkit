package main

import (
	"archive/zip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/feedbooks/webpub-streamer/fetcher"
	"github.com/feedbooks/webpub-streamer/models"
	"github.com/feedbooks/webpub-streamer/parser"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
	"rsc.io/letsencrypt"
)

type currentBook struct {
	filename    string
	publication models.Publication
	timestamp   time.Time
}

var currentBookList []currentBook

// Serv TODO add doc
func main() {

	n := negroni.Classic()
	n.Use(negroni.NewStatic(http.Dir("public")))
	n.UseHandler(bookHandler(false))

	if len(os.Args) > 1 && os.Args[1] == "https" {
		var m letsencrypt.Manager
		if err := m.CacheFile("letsencrypt.cache"); err != nil {
			log.Fatal(err)
		}

		s := &http.Server{
			Handler:        n,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Addr:           ":443",
			TLSConfig: &tls.Config{
				GetCertificate: m.GetCertificate,
			},
		}

		log.Fatal(s.ListenAndServeTLS("", ""))

	} else {
		s := &http.Server{
			Handler:        n,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Addr:           ":8080",
		}

		//		log.Fatal(s.ListenAndServeTLS("test.cert", "test.key"))
		log.Fatal(s.ListenAndServe())
	}
}

func bookHandler(test bool) http.Handler {
	serv := mux.NewRouter()

	serv.HandleFunc("/index.html", getBooks)
	serv.HandleFunc("/", getBooks)
	serv.HandleFunc("/viewer.js", viewer)
	serv.HandleFunc("/sw.js", sw)
	serv.HandleFunc("/{filename}/", bookIndex)
	serv.HandleFunc("/{filename}/manifest.json", getManifest)
	serv.HandleFunc("/{filename}/webapp.webmanifest", getWebAppManifest)
	serv.HandleFunc("/{filename}/index.html", bookIndex)
	serv.HandleFunc("/{filename}/{asset:.*}", getAsset)
	return serv
}

func getManifest(w http.ResponseWriter, req *http.Request) {
	var current currentBook
	var publication models.Publication

	vars := mux.Vars(req)
	filename := vars["filename"]
	filenamePath := "books/" + filename

	for _, book := range currentBookList {
		if vars["filename"] == book.filename {
			current = book
		}
	}

	if current.filename == "" {
		manifestURL := "http://" + req.Host + "/" + filename + "/manifest.json"
		publication = parser.Parse(filenamePath, manifestURL)
		for _, book := range currentBookList {
			if filename == book.filename {
				current = book
			}
		}

		currentBookList = append(currentBookList, currentBook{filename: filename, publication: publication, timestamp: time.Now()})
	} else {
		publication = current.publication
	}

	j, _ := json.Marshal(publication)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(j)
	return
}

func getAsset(w http.ResponseWriter, req *http.Request) {
	var current currentBook

	vars := mux.Vars(req)
	assetname := vars["asset"]

	for _, book := range currentBookList {
		if vars["filename"] == book.filename {
			current = book
		}
	}

	if current.filename == "" {
		manifestURL := "http://" + req.Host + "/" + vars["filename"] + "/manifest.json"
		publication := parser.Parse("books/"+vars["filename"], manifestURL)
		currentBookList = append(currentBookList, currentBook{filename: vars["filename"], publication: publication, timestamp: time.Now()})
	}

	buff, mediaType := fetcher.Fetch(current.publication, assetname)
	finalBuffReader := strings.NewReader(buff)

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeContent(w, req, assetname, time.Now(), finalBuffReader)
	return

}

func getWebAppManifest(w http.ResponseWriter, req *http.Request) {
	var opfFileName string
	var webapp models.AppInstall

	vars := mux.Vars(req)
	filename := "books/" + vars["filename"]

	webapp.Display = "standalone"
	webapp.StartURL = "index.html"
	webapp.Icons = models.Icon{
		Size:      "144x144",
		Src:       "/logo.png",
		MediaType: "image/png",
	}

	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, f := range zipReader.File {
		if f.Name == "META-INF/container.xml" {
			rc, errOpen := f.Open()
			if errOpen != nil {
				fmt.Println("error openging " + f.Name)
			}
			doc := etree.NewDocument()
			_, err = doc.ReadFrom(rc)
			if err == nil {
				root := doc.SelectElement("container")
				rootFiles := root.SelectElements("rootfiles")
				for _, rootFileTag := range rootFiles {
					rootFile := rootFileTag.SelectElement("rootfile")
					if rootFile != nil {
						opfFileName = rootFile.SelectAttrValue("full-path", "")
					}
				}
			} else {
				fmt.Println(err)
			}
			rc.Close()
		}
	}

	if opfFileName != "" {
		for _, f := range zipReader.File {
			if f.Name == opfFileName {
				rc, errOpen := f.Open()
				if errOpen != nil {
					fmt.Println("error openging " + f.Name)
				}
				doc := etree.NewDocument()
				_, err = doc.ReadFrom(rc)
				if err == nil {
					root := doc.SelectElement("package")
					meta := root.SelectElement("metadata")

					titleTag := meta.SelectElement("title")
					webapp.Name = titleTag.Text()
					webapp.ShortName = titleTag.Text()

					j, _ := json.Marshal(webapp)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Write(j)
					return
				}
			}
		}
	}

}

func bookIndex(w http.ResponseWriter, req *http.Request) {
	var err error

	vars := mux.Vars(req)
	filename := "books/" + vars["filename"]

	t, err := template.ParseFiles("index.html") // Parse template file.
	if err != nil {
		fmt.Println(err)
	}
	t.Execute(w, filename) // merge.
}

func getBooks(w http.ResponseWriter, req *http.Request) {
	var books []string

	files, _ := ioutil.ReadDir("books")
	for _, f := range files {
		fmt.Println(f.Name())
		books = append(books, f.Name())
	}

	t, err := template.ParseFiles("book_index.html") // Parse template file.
	if err != nil {
		fmt.Println(err)
	}
	t.Execute(w, books)
}

func viewer(w http.ResponseWriter, req *http.Request) {

	f, _ := os.OpenFile("public/viewer.js", os.O_RDONLY, 666)
	buff, _ := ioutil.ReadAll(f)

	w.Header().Set("Content-Type", "text/javascript")
	w.Write(buff)
}

func sw(w http.ResponseWriter, req *http.Request) {

	f, _ := os.OpenFile("public/sw.js", os.O_RDONLY, 666)
	buff, _ := ioutil.ReadAll(f)

	w.Header().Set("Content-Type", "text/javascript")
	w.Write(buff)
}
