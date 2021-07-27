package main

import "flag"

func main() {
	nFlag := flag.String("f", "", "path to publication")

	flag.Parse()
}
