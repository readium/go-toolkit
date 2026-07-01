package audio

import (
	"bytes"
	"context"
	"image"
	// Register decoders so cover dimensions can be read.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/dhowden/tag"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

// coverHref is the synthetic HREF under which an extracted cover image is served.
const coverHref = "~readium/cover"

// coverServiceFactory builds a [pub.ServiceFactory] serving the cover image
// embedded in an audio file's tags. It returns nil when there is no usable
// cover.
func coverServiceFactory(pic *tag.Picture) pub.ServiceFactory {
	if pic == nil || len(pic.Data) == 0 {
		return nil
	}

	ext := pic.Ext
	if ext == "" {
		ext = "jpg"
	}

	var mt *mediatype.MediaType
	if pic.MIMEType != "" {
		if m, err := mediatype.NewOfString(pic.MIMEType); err == nil {
			mt = &m
		}
	}
	if mt == nil {
		mt = mediatype.OfExtension(ext)
	}

	link := manifest.Link{
		Href:      manifest.MustNewHREFFromString(coverHref+"."+ext, false),
		MediaType: mt,
		Rels:      manifest.Strings{"cover"},
	}

	if cfg, _, err := image.DecodeConfig(bytes.NewReader(pic.Data)); err == nil {
		link.Width = uint(cfg.Width)
		link.Height = uint(cfg.Height)
	}

	data := pic.Data
	return func(_ pub.Context, _ bool) pub.Service {
		return coverService{link: link, data: data}
	}
}

// coverService exposes an in-memory cover image as a publication resource.
type coverService struct {
	link manifest.Link
	data []byte
}

func (s coverService) Links() manifest.LinkList {
	return manifest.LinkList{s.link}
}

func (s coverService) Get(_ context.Context, link manifest.Link) (fetcher.Resource, bool) {
	if !link.URL(nil, nil).Equivalent(s.link.URL(nil, nil)) {
		return nil, false
	}
	data := s.data
	return fetcher.NewBytesResource(s.link, func() []byte { return data }), true
}

func (s coverService) Close() {}
