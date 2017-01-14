package parser

import (
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testDataStruct struct {
	filepath     string
	err          error
	title        string
	authorName   string
	identifier   string
	thirdChapter string
	tocChildren  bool
	Source       string
}

func TestPublication(t *testing.T) {
	testData := []testDataStruct{
		{"../test/empty.epub", errors.New("can't open or parse epub file with err : open ../test/empty.epub: no such file or directory"), "", "", "", "", false, ""},
		{"../test/moby-dick.epub", nil, "Moby-Dick", "Herman Melville", "code.google.com.epub-samples.moby-dick-basic", "ETYMOLOGY.", false, ""},
		{"../test/kusamakura.epub", nil, "草枕", "夏目 漱石", "http://www.aozora.gr.jp/cards/000148/card776.html", "三", false, ""},
		{"../test/feedbooks_book_6816.epub", nil, "Mémoires d'Outre-tombe", "François-René de Chateaubriand", "urn:uuid:47f6aaf6-aa7e-11e6-8357-4c72b9252ec6", "Partie 1", true, "www.ebooksfrance.com"},
	}

	for _, d := range testData {
		Convey("Given "+d.title+" book", t, func() {
			publication, err := Parse(d.filepath, "http://localhost/")
			Convey("There no exception parsing", func() {
				if d.err != nil {
					So(err.Error(), ShouldEqual, d.err.Error())
				} else {
					So(err, ShouldEqual, nil)
				}
			})
			Convey("The title is good", func() {
				So(publication.Metadata.Title, ShouldEqual, d.title)
			})

			if err == nil {
				Convey("There must be an author", func() {
					So(len(publication.Metadata.Author), ShouldBeGreaterThanOrEqualTo, 1)
				})
			}

			if d.authorName != "" && len(publication.Metadata.Author) > 0 {
				Convey("first author is good", func() {
					So(publication.Metadata.Author[0].Name, ShouldEqual, d.authorName)
				})
			}

			Convey("Identifier is good", func() {
				So(publication.Metadata.Identifier, ShouldEqual, d.identifier)
			})

			Convey("The third chapter is good", func() {
				if len(publication.TOC) > 3 {
					So(publication.TOC[2].Title, ShouldEqual, d.thirdChapter)
				}
			})

			Convey("There Chapter with children", func() {
				emptyChildren := false
				for _, toc := range publication.TOC {
					if len(toc.Children) > 0 {
						emptyChildren = true
					}
				}
				if d.tocChildren == true {
					So(emptyChildren, ShouldBeTrue)
				} else {
					So(emptyChildren, ShouldBeFalse)
				}
			})

			Convey("dc:source is good", func() {
				So(publication.Metadata.Source, ShouldEqual, d.Source)
			})

		})
	}

}
