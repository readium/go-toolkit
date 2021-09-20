package epub

//Ncx OPS/toc.ncx
type Ncx struct {
	Points   []NavPoint `xml:"navMap>navPoint"`
	PageList PageList   `xml:"pageList"`
}

//NavPoint nav point
type NavPoint struct {
	ID          string     `xml:"id,attr"`
	Text        string     `xml:"navLabel>text"`
	Content     Content    `xml:"content"`
	Points      []NavPoint `xml:"navPoint"`
	PlayerOrder int        `xml:"playOrder,attr"`
}

//Content nav-point content
type Content struct {
	Src string `xml:"src,attr"`
}

// PageList page list
type PageList struct {
	PageTarget []PageTarget `xml:"pageTarget"`
	Class      string       `xml:"class,attr"`
	ID         string       `xml:"id,attr"`
}

// PageTarget page target
type PageTarget struct {
	ID        string  `xml:"id,attr"`
	Text      string  `xml:"navLabel>text"`
	Value     string  `xml:"value,attr"`
	Type      string  `xml:"type,attr"`
	PlayOrder int     `xml:"playOrder,attr"`
	Content   Content `xml:"content"`
}
