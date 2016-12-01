package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/feedbooks/webpub-streamer/parser"
)

func main() {

	_, file := filepath.Split(os.Args[1])
	publication := parser.Parse(file, os.Args[1], "localhost")
	j, _ := json.Marshal(publication)
	fmt.Println(string(j))
}
