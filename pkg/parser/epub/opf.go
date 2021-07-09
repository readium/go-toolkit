package epub

//Opf content.opf
type Opf struct {
	Metadata         Metadata    `xml:"metadata"`
	Manifest         []Manifest  `xml:"manifest>item"`
	Spine            Spine       `xml:"spine"`
	Guide            []Reference `xml:"guide>reference"`
	UniqueIdentifier string      `xml:"unique-identifier,attr"`
	Dir              string      `xml:"dir,attr"`
	Version          string      `xml:"version,attr"`
}

//Metadata metadata
type Metadata struct {
	Title       []Title      `xml:"title"`
	Language    []string     `xml:"language"`
	Identifier  []Identifier `xml:"identifier"`
	Creator     []Author     `xml:"creator"`
	Subject     []Subject    `xml:"subject"`
	Description []string     `xml:"description"`
	Publisher   []string     `xml:"publisher"`
	Contributor []Author     `xml:"contributor"`
	Date        []Date       `xml:"date"`
	Type        []string     `xml:"type"`
	Format      []string     `xml:"format"`
	Source      []string     `xml:"source"`
	Relation    []string     `xml:"relation"`
	Coverage    []string     `xml:"coverage"`
	Rights      []string     `xml:"rights"`
	Meta        []Metafield  `xml:"meta"`
}

// Identifier identifier
type Identifier struct {
	Data   string `xml:",chardata"`
	ID     string `xml:"id,attr"`
	Scheme string `xml:"scheme,attr"`
}

// Subject subject
type Subject struct {
	Data      string `xml:",chardata"`
	Term      string `xml:"term,attr"`
	Authority string `xml:"authority,attr"`
	Lang      string `xml:"lang,attr"`
}

// Title title
type Title struct {
	Data string `xml:",chardata"`
	ID   string `xml:"id,attr"`
	Lang string `xml:"lang,attr"`
	Dir  string `xml:"dir,attr"`
}

// Author author
type Author struct {
	Data   string `xml:",chardata"`
	FileAs string `xml:"file-as,attr"`
	Role   string `xml:"role,attr"`
	ID     string `xml:"id,attr"`
}

// Date date
type Date struct {
	Data  string `xml:",chardata"`
	Event string `xml:"event,attr"`
}

// Metafield metafield
type Metafield struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Refine   string `xml:"refines,attr"`
	Property string `xml:"property,attr"`
	Data     string `xml:",chardata"`
	ID       string `xml:"id,attr"`
	Lang     string `xml:"lang,attr"`
}

//Manifest manifest
type Manifest struct {
	ID           string `xml:"id,attr"`
	Href         string `xml:"href,attr"`
	MediaType    string `xml:"media-type,attr"`
	Fallback     string `xml:"media-fallback,attr"`
	Properties   string `xml:"properties,attr"`
	MediaOverlay string `xml:"media-overlay,attr"`
}

// Spine spine
type Spine struct {
	ID              string      `xml:"id,attr"`
	Toc             string      `xml:"toc,attr"`
	PageProgression string      `xml:"page-progression-direction,attr"`
	Items           []SpineItem `xml:"itemref"`
}

// SpineItem spine item
type SpineItem struct {
	IDref      string `xml:"idref,attr"`
	Linear     string `xml:"linear,attr"`
	ID         string `xml:"id,attr"`
	Properties string `xml:"properties,attr"`
}

// Reference reference in guide
type Reference struct {
	Href  string `xml:"href,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}
