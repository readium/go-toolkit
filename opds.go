package r2go

import (
	"github.com/jinzhu/copier"
	"github.com/opds-community/libopds2-go/opds2"
	"github.com/readium/r2-streamer-go/pkg/pub"
)

// AddPublicationToFeed filter publication fields and add it to the feed
func AddPublicationToFeed(feed *opds2.Feed, publication pub.Publication, baseURL string) {
	var pub opds2.Publication
	var coverLink opds2.Link

	copier.Copy(&pub, publication)
	l := opds2.Link{}
	l.Rel = []string{"self"}
	l.Href = baseURL + "manifest.json"
	l.TypeLink = "application/webpub+json"
	pub.Links = append(pub.Links, l)
	img, err := publication.GetCover()
	if img.Href != "" && err == nil {
		img.Href = baseURL + img.Href
		copier.Copy(&coverLink, img)
		pub.Images = append(pub.Images, coverLink)
	}

	feed.Publications = append(feed.Publications, pub)
}
