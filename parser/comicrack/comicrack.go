package comicrack

import (
	"encoding/xml"
	"fmt"
	"io"
)

// ComicInfo struct for comicrack file metadata
type ComicInfo struct {
	Title           string `xml:"Title"`
	Series          string `xml:"Series"`
	Volume          int    `xml:"Volume"`
	Number          int    `xml:"Number"`
	Writer          string `xml:"Writer"`
	Penciller       string `xml:"Penciller"`
	Inker           string `xml:"Inker"`
	Colorist        string `xml:"Colorist"`
	ScanInformation string `xml:"ScanInformation"`
	Summary         string `xml:"Summary"`
	Year            int    `xml:"Year"`
	PageCount       int    `xml:"PageCount"`
	Pages           []struct {
		Image       int    `xml:"Image,attr"`
		Bookmark    string `xml:"Bookmark,attr"`
		Type        string `xml:"Type,attr"`
		ImageSize   int    `xml:"ImageSize,attr"`
		ImageWidth  int    `xml:"ImageWidth,attr"`
		ImageHeight int    `xml:"ImageHeight,attr"`
	} `xml:"Pages>Page"`
}

// Parse get the data and parse it to the ComicRack struct
func Parse(fd io.ReadCloser) ComicInfo {
	var cr ComicInfo

	dec := xml.NewDecoder(fd)
	err := dec.Decode(&cr)
	if err != nil {
		fmt.Println(err)
	}

	return cr
}
