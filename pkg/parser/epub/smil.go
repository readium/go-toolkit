package epub

type SMIL struct {
	Body Body `xml:"body"`
}

type Body struct {
	TextRef string `xml:"textref,attr"`
	Seq     []Seq  `xml:"seq"`
	Par     []Par  `xml:"par"`
}

type Seq struct {
	TextRef string `xml:"textref,attr"`
	Par     []Par  `xml:"par"`
	Seq     []Seq  `xml:"seq"`
}

type Par struct {
	Text  Text  `xml:"text"`
	Audio Audio `xml:"audio"`
}

type Text struct {
	Src string `xml:"src,attr"`
}

type Audio struct {
	Src       string `xml:"src,attr"`
	ClipBegin string `xml:"clipBegin,attr"`
	ClipEnd   string `xml:"clipEnd,attr"`
}
