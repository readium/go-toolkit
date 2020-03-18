package parser

import (
	"errors"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

type testDataStruct struct {
	filepath        string
	err             error
	title           string
	authorName      string
	identifier      string
	thirdChapter    string
	tocChildren     bool
	Source          string
	NoLinear        string
	MultipleLang    bool
	HasSubject      string
	PublicationDate string
}

type testDataFixedStruct struct {
	filepath             string
	renditionLayout      string
	renditionOrientation string
	renditionSpread      string
	linkLayout           string
	linkOrientation      string
	linkSpread           string
	linkPage             string
}

func TestPublication(t *testing.T) {
	testData := []testDataStruct{
		{"../test/empty.epub", errors.New("can't open or parse epub file with err : open ../test/empty.epub: no such file or directory"), "", "", "", "", false, "", "", false, "", ""},
		{"../test/moby-dick.epub", nil, "Moby-Dick", "Herman Melville", "code.google.com.epub-samples.moby-dick-basic", "ETYMOLOGY.", false, "", "cover.xhtml", false, "", ""},
		{"../test/kusamakura.epub", nil, "草枕", "夏目 漱石", "http://www.aozora.gr.jp/cards/000148/card776.html", "三", false, "", "", true, "", ""},
		{"../test/feedbooks_book_6816.epub", nil, "Mémoires d'Outre-tombe", "François-René de Chateaubriand", "urn:uuid:47f6aaf6-aa7e-11e6-8357-4c72b9252ec6", "Partie 1", true, "www.ebooksfrance.com", "", false, "Non-Fiction", "1850-01-01"},
		{"../test/readium-test-files/demos/alice3/", nil, "Alice's Adventures in Wonderland", "", "urn:uuid:7408D53A-5383-40AA-8078-5256C872AE41", "III. A Caucus-Race and a Long Tale", false, "", "", false, "", "1865-07-04"},
	}

	for _, d := range testData {
		Convey("Given "+d.title+" book", t, func() {
			publication, err := Parse(d.filepath)
			Convey("There no exception parsing", func() {
				if d.err != nil {
					So(err.Error(), ShouldEqual, d.err.Error())
				} else {
					So(err, ShouldEqual, nil)
				}
			})

			if d.MultipleLang == true {
				Convey("The title has multiple language", func() {
					So(publication.Metadata.Title.MultiString, ShouldNotBeEmpty)
				})

				Convey("The title is good", func() {
					So(publication.Metadata.Title.MultiString[publication.Metadata.Language[0]], ShouldEqual, d.title)
				})
			} else {
				Convey("The title is good", func() {
					So(publication.Metadata.Title.String(), ShouldEqual, d.title)
				})
			}

			if err == nil && d.authorName != "" {
				Convey("There must be an author", func() {
					So(len(publication.Metadata.Author), ShouldBeGreaterThanOrEqualTo, 1)
				})
			}

			if d.authorName != "" && len(publication.Metadata.Author) > 0 {
				Convey("first author is good", func() {
					So(publication.Metadata.Author[0].Name.String(), ShouldEqual, d.authorName)
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

			if d.NoLinear != "" {
				Convey("item no linear is not in spine", func() {
					findItemInSpine := false

					for _, it := range publication.ReadingOrder {
						if it.Href == d.NoLinear {
							findItemInSpine = true
						}
					}

					So(findItemInSpine, ShouldEqual, false)
				})
			}

			Convey("readingOrder and resources are mutually exclusive", func() {
				findItemInResources := false

				for _, this := range publication.ReadingOrder {
					for _, that := range publication.Resources {
						if this.Href == that.Href {
							findItemInResources = true
						}
					}
				}

				So(findItemInResources, ShouldEqual, false)
			})

			if d.HasSubject != "" {
				Convey("There "+d.HasSubject+"Subject in book", func() {
					findSubject := false
					for _, s := range publication.Metadata.Subject {
						if s.Name == d.HasSubject {
							findSubject = true
						}
					}
					So(findSubject, ShouldEqual, true)
				})
			}

			if d.PublicationDate != "" {
				Convey("There Publication date in book", func() {
					dateParsed, _ := time.Parse("2006-01-02", d.PublicationDate)

					sameDate := dateParsed.Equal(*publication.Metadata.PublicationDate)
					So(sameDate, ShouldBeTrue)
				})
			}

		})
	}

}

func TestFixedPublication(t *testing.T) {
	testData := []testDataFixedStruct{
		{"../test/page-blanche.epub", "fixed", "auto", "auto", "", "", "", "right"},
		{"../test/cole-voyage-of-life.epub", "", "", "", "fixed", "landscape", "none", ""},
	}

	for _, d := range testData {
		Convey("Given "+d.filepath+" book", t, func() {
			publication, _ := Parse(d.filepath)
			if d.renditionLayout != "" {
				Convey("There Layout info", func() {
					So(publication.Metadata.Rendition.Layout, ShouldEqual, d.renditionLayout)
				})
			}
			if d.renditionOrientation != "" {
				Convey("There Orientation info", func() {
					So(publication.Metadata.Rendition.Orientation, ShouldEqual, d.renditionOrientation)
				})
			}
			if d.renditionSpread != "" {
				Convey("There Spread info", func() {
					So(publication.Metadata.Rendition.Spread, ShouldEqual, d.renditionSpread)
				})
			}

			if d.linkLayout != "" {
				Convey("There layout info in link", func() {
					layout := false
					for _, item := range publication.ReadingOrder {
						if item.Properties != nil {
							if item.Properties.Layout == d.linkLayout {
								layout = true
							}
						}
					}
					So(layout, ShouldEqual, true)
				})

			}

			if d.linkOrientation != "" {
				Convey("There orientation info in link", func() {
					orientation := false
					for _, item := range publication.ReadingOrder {
						if item.Properties != nil {
							if item.Properties.Orientation == d.linkOrientation {
								orientation = true
							}
						}
					}
					So(orientation, ShouldEqual, true)
				})
			}

			if d.linkSpread != "" {
				Convey("There spread info in link", func() {
					spread := false
					for _, item := range publication.ReadingOrder {
						if item.Properties != nil {
							if item.Properties.Spread == d.linkSpread {
								spread = true
							}
						}
					}
					So(spread, ShouldEqual, true)
				})
			}

			if d.linkPage != "" {
				Convey("There page info in link", func() {
					page := false
					for _, item := range publication.ReadingOrder {
						if item.Properties != nil {
							if item.Properties.Page == d.linkPage {
								page = true
							}
						}
					}
					So(page, ShouldEqual, true)
				})
			}
		})
	}
}

func TestSmilTime(t *testing.T) {
	testData := [][]string{
		{"12.345", "12.345"},
		{"2345ms", "2.345"},
		{"345ms", "0.345"},
		{"7.75h", "27900"},
		{"76.2s", "76.2"},
		{"00:56.78", "56.78"},
		{"09:58", "598"},
		{"0:00:04", "4"},
		{"0:05:01.2", "301.2"},
		{"124:59:36", "449976"},
		{"5:34:31.396", "20071.396"},
		{"", ""},
	}

	for _, d := range testData {
		Convey("Given "+d[0], t, func() {
			Convey("This should convert to "+d[1], func() {
				So(smilTimeToSeconds(d[0]), ShouldEqual, d[1])
			})
		})
	}
}
