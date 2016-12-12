package main

import (
	"encoding/json"
	"fmt"

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
	j, _ := json.Marshal(publication)
	fmt.Println(string(j))
}
