package main 

import (
	"archive/zip"
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/feedbooks/webpub-streamer/models"
	"github.com/feedbooks/webpub-streamer/parser"
	"github.com/gorilla/mux"
	"github.com/kapmahc/epub"
	"github.com/urfave/negroni"
	"rsc.io/letsencrypt"
)

// Serv TODO add doc
func main() {

	n := negroni.Classic()
	n.Use(negroni.NewStatic(http.Dir("public")))
	n.UseHandler(loanHandler(false))

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

func loanHandler(test bool) http.Handler {
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
	vars := mux.Vars(req)
	filename := vars["filename"]
	filenamePath := "books/" + filename

	publication := parser.Parse(filename, filenamePath, req.Host)
	j, _ := json.Marshal(publication)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(j)
	return
}

func getAsset(w http.ResponseWriter, req *http.Request) {
	var buff string

	vars := mux.Vars(req)
	filename := "books/" + vars["filename"]
	assetname := vars["asset"]
	jsInject := req.URL.Query().Get("js")
	cssInject := req.URL.Query().Get("css")

	book, _ := epub.Open(filename)

	extension := filepath.Ext(filename)
	if extension == ".css" {
		w.Header().Set("Content-Type", "text/css")
	}
	if extension == ".xml" {
		w.Header().Set("Content-Type", "application/xhtml+xml")
	}
	if extension == ".js" {
		w.Header().Set("Content-Type", "text/javascript")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")

	fmt.Println(assetname)
	assetFd, _ := book.Open(assetname)
	buffByte, _ := ioutil.ReadAll(assetFd)
	buff = string(buffByte)
	buffReader := strings.NewReader(buff)

	finalBuff := ""
	if cssInject != "" || jsInject != "" {
		scanner := bufio.NewScanner(buffReader)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "</head>") {
				headBuff := ""
				if jsInject != "" {
					headBuff += strings.Replace(scanner.Text(), "</head>", "<script src='/"+jsInject+"'></script></head>", 1)
				}
				if cssInject != "" {
					if headBuff == "" {
						headBuff += strings.Replace(scanner.Text(), "</head>", "<link rel='stylesheet' type='text/css' href='/"+cssInject+"'></script></head>", 1)
					} else {
						headBuff = strings.Replace(headBuff, "</head>", "<link rel='stylesheet' type='text/css' href='/"+cssInject+"'></head>", 1)
					}
				}
				if headBuff == "" {
					headBuff = scanner.Text()
				}
				finalBuff += headBuff + "\n"
			} else {
				finalBuff += scanner.Text() + "\n"
			}
		}
	} else {
		finalBuff = buff
	}

	finalBuffReader := strings.NewReader(finalBuff)
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
