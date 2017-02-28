package epub

//Ncx OPS/toc.ncx
type Ncx struct {
	Points   []NavPoint `xml:"navMap>navPoint"`
	PageList PageList   `xml:"pageList"`
}

//NavPoint nav point
type NavPoint struct {
	Text        string     `xml:"navLabel>text"`
	Content     Content    `xml:"content"`
	Points      []NavPoint `xml:"navPoint"`
	PlayerOrder int        `xml:"playOrder"`
}

//Content nav-point content
type Content struct {
	Src string `xml:"src,attr" json:"src"`
}

// PageList page list
type PageList struct {
	PageTarget []PageTarget `xml:"pageTarget"`
	Class      string       `xml:"class"`
	ID         string       `xml:"id"`
}

// PageTarget page target
type PageTarget struct {
	Text      string  `xml:"navLabel>text"`
	Value     string  `xml:"value"`
	Type      string  `xml:"type"`
	PlayOrder int     `xml:"playOrder"`
	Content   Content `xml:"content"`
}
