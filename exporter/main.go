package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/feedbooks/webpub-streamer/parser"
)

var (
	filename = kingpin.Flag("file", "file to parse").Required().Short('f').String()
	url      = kingpin.Flag("url", "URL for the manifest").Short('u').String()
)

func main() {

	kingpin.Version("0.0.1")
	kingpin.Parse()

	publication := parser.Parse(*filename, *url)
	cover := publication.GetCover()
	fmt.Println(cover.Href)
	nav := publication.GetNavDoc()
	fmt.Println(nav.Href)

	j, _ := json.Marshal(publication)
	var identJSON bytes.Buffer
	json.Indent(&identJSON, j, "", " ")
	identJSON.WriteTo(os.Stdout)
}
