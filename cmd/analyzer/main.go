package main

import (
	"encoding/json"
	"os"

	"github.com/readium/go-toolkit/pkg/analyzer"
	"github.com/readium/go-toolkit/pkg/manifest"
)

func main() {
	if len(os.Args) < 2 {
		panic("usage: " + os.Args[0] + " <test dir image name>")
	}

	r, err := os.OpenRoot("./test")
	if err != nil {
		panic(err)
	}
	defer r.Close()
	fs := r.FS()

	link, _, err := analyzer.Image(fs, manifest.Link{
		Href: manifest.MustNewHREFFromString(os.Args[1], false),
	}, true)
	if err != nil {
		panic(err)
	}

	bin, _ := json.Marshal(link)
	println(string(bin))
}
