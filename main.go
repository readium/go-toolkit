package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/beevik/etree"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
	graceful "gopkg.in/tylerb/graceful.v1"
)

// Metadata metadata struct
type Metadata struct {
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	Identifier string    `json:"identifier"`
	Language   string    `json:"language"`
	Modified   time.Time `json:"modified"`
}

// Link link struct
type Link struct {
	Rel      string `json:"rel,omitempty"`
	Href     string `json:"href"`
	TypeLink string `json:"type"`
	Height   int    `json:"height,omitempty"`
	Width    int    `json:"width,omitempty"`
}

// Manifest manifest struct
type Manifest struct {
	Metadata  Metadata `json:"metadata"`
	Links     []Link   `json:"links"`
	Spine     []Link   `json:"spine,omitempty"`
	Resources []Link   `json:"resources"`
}

func main() {

	n := negroni.Classic()
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
	})
	n.Use(c)
	n.UseHandler(loanHandler(false))

	graceful.Run(":8096", time.Duration(10)*time.Second, n)

}

func loanHandler(test bool) http.Handler {
	serv := mux.NewRouter()

	serv.HandleFunc("/manifest/{filename}/manifest.json", getManifest)
	serv.HandleFunc("/assets/{filename}/{asset:.*}", getAsset)

	return serv

}

func getManifest(w http.ResponseWriter, req *http.Request) {
	var opfFileName string
	var manifestStruct Manifest
	var metaStruct Metadata

	metaStruct.Modified = time.Now()

	vars := mux.Vars(req)
	filename := vars["filename"]

	self := Link{
		Rel:      "self",
		Href:     "http://" + req.Host + "/manifest/" + filename + "/manifest.json",
		TypeLink: "application/epub+zip",
	}
	manifestStruct.Links = make([]Link, 1)
	manifestStruct.Resources = make([]Link, 0)
	manifestStruct.Resources = make([]Link, 0)
	manifestStruct.Links[0] = self

	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		fmt.Println(err)
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
					metaStruct.Title = titleTag.Text()

					langTag := meta.SelectElement("language")
					metaStruct.Language = langTag.Text()

					identifierTag := meta.SelectElement("identifier")
					metaStruct.Identifier = identifierTag.Text()

					creatorTag := meta.SelectElement("creator")
					metaStruct.Author = creatorTag.Text()

					bookManifest := root.SelectElement("manifest")
					itemsManifest := bookManifest.SelectElements("item")
					for _, item := range itemsManifest {
						linkItem := Link{}
						linkItem.TypeLink = item.SelectAttrValue("media-type", "")
						linkItem.Href = "https://proto.myopds.com/assets/" + filename + "/" + item.SelectAttrValue("href", "")
						if linkItem.TypeLink == "application/xhtml+xml" {
							manifestStruct.Spine = append(manifestStruct.Spine, linkItem)
						} else {
							manifestStruct.Resources = append(manifestStruct.Resources, linkItem)
						}
					}

					manifestStruct.Metadata = metaStruct
					j, _ := json.Marshal(manifestStruct)
					fmt.Println(string(j))
					w.Write(j)
					return
				}
			}
		}
	}

}

func getAsset(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	filename := vars["filename"]
	assetname := vars["asset"]

	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		fmt.Println(err)
	}

	for _, f := range zipReader.File {
		fmt.Println(f.Name)
		if f.Name == "OPS/"+assetname {
			rc, errOpen := f.Open()
			if errOpen != nil {
				fmt.Println("error openging " + f.Name)
			}
			buff, _ := ioutil.ReadAll(rc)
			defer rc.Close()
			extension := filepath.Ext(f.Name)
			if extension == ".css" {
				w.Header().Set("Content-Type", "text/css")
			}
			if extension == ".xml" {
				w.Header().Set("Content-Type", "application/xhtml+xml")
			}
			if extension == ".js" {
				w.Header().Set("Content-Type", "text/javascript")
			}
			w.Write(buff)
			return
		}
	}

}
