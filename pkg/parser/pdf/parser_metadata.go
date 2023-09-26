package pdf

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/trimmer-io/go-xmp/xmp"
)

// This is completely random
// var UUIDNameSpaceForPDF = uuid.Must(uuid.Parse("4a706cb0-458c-4180-9601-086121ee8d9f"))

func loadDecoder(meta pdfcpu.Metadata) (*xmp.Document, []byte, error) {
	metabin, err := io.ReadAll(meta.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed reading XMP metadata block")
	}
	doc := &xmp.Document{}
	if err := xmp.Unmarshal(metabin, doc); err != nil {
		return nil, nil, errors.Wrap(err, "failed decoding XMP metadata")
	}
	return doc, metabin, nil
}

func ParseMetadata(ctx *model.Context, link *manifest.Link) (m manifest.Manifest, err error) {
	if link != nil {
		m.ReadingOrder = manifest.LinkList{{
			Href:       strings.TrimPrefix(link.Href, "/"),
			Type:       mediatype.PDF.String(),
			Title:      link.Title,
			Rels:       link.Rels,
			Properties: link.Properties,
			Alternates: link.Alternates,
			Children:   link.Children,
			Languages:  link.Languages,
		}}
	}
	m.Metadata.ConformsTo = manifest.Profiles{manifest.ProfilePDF}

	// hashmaterial := make([]string, 0, 64)
	metas, _ := pdfcpu.ExtractMetadata(ctx)
	for _, meta := range metas {
		doc, _, derr := loadDecoder(meta)
		if derr != nil {
			err = derr
			return
		}
		defer doc.Close()
		// hashmaterial = append(hashmaterial, string(metabin))

		// b, _ := json.MarshalIndent(doc, "", "  ")
		// println(string(b))

		err = ParseXMPMetadata(doc, &m.Metadata)
		if err != nil {
			return
		}
	}

	err = ParsePDFMetadata(ctx, &m)
	if err != nil {
		return
	}

	// Removed for now, because identifiers are technically optional in webpub, and we can't
	// tell if this is the actual PDF ID or not if we use this custom hash as a fallback.
	/*if m.Metadata.Identifier == "" {
		// Create a UUIDv5 which is based on a SHA1 hash of the PDF's metadata.
		// This is done instead of a hash of the entire file like the mobile toolkits
		// because it minimizes the amount of reading of the PDF file, vitally important
		// when streaming a large PDF file from a remote source.

		// TODO reduce PDF reads
		v := ctx.HeaderVersion
		if ctx.RootVersion != nil {
			v = ctx.RootVersion
		}
		hashmaterial = append(hashmaterial, v.String(), strconv.Itoa(ctx.PageCount))
		hashmaterial = append(hashmaterial, ctx.Title, ctx.Author, ctx.Subject, ctx.Producer, ctx.Creator, ctx.CreationDate, ctx.ModDate)
		for k, v := range ctx.Properties {
			hashmaterial = append(hashmaterial, fmt.Sprintf("%s = %s", k, v))
		}

		kwl, err := pdfcpu.KeywordsList(ctx.XRefTable)
		if err == nil {
			hashmaterial = append(hashmaterial, kwl...)
		}

		aa, err := ctx.ListAttachments()
		if err == nil {
			for _, a := range aa {
				s := a.FileName
				if a.Desc != "" {
					s = fmt.Sprintf("%s (%s)", s, a.Desc)
				}
				hashmaterial = append(hashmaterial, s)
			}
		}

		for _, v := range ctx.XRefTable.Table {
			if v != nil {
				if v.Object != nil {
					// println(v.Object.PDFString())
				}

			}
		}

		m.Metadata.Identifier = uuid.NewSHA1(UUIDNameSpaceForPDF, []byte(strings.Join(hashmaterial, "|"))).String()
	}*/

	return
}

func ParseXMPMetadata(doc *xmp.Document, metadata *manifest.Metadata) error {
	if doc == nil {
		return nil
	}

	// TODO

	return nil
}

func ParsePDFMetadata(ctx *model.Context, m *manifest.Manifest) error {
	// Page count
	if ctx.PageCount > 0 && m.Metadata.NumberOfPages == nil {
		pc := uint(ctx.PageCount)
		m.Metadata.NumberOfPages = &pc
	}

	// Identifier
	if len(ctx.XRefTable.ID) > 0 {
		m.Metadata.Identifier = hex.EncodeToString([]byte(ctx.XRefTable.ID[0].String()))
	}

	// Title
	// TODO determine whether XMP or PDF title should take precendence. For now it's XMP
	if ctx.Title != "" && m.Metadata.LocalizedTitle.String() == "" {
		m.Metadata.LocalizedTitle = manifest.NewLocalizedStringFromString(ctx.Title)
	}

	// Author
	// Note: XMP can have multiple authors, PDF "Author" seems to only have the first one for some PDFs
	if ctx.Author != "" && len(m.Metadata.Authors) == 0 {
		m.Metadata.Authors = manifest.Contributors{{
			LocalizedName: manifest.NewLocalizedStringFromString(ctx.Author),
		}}
	}

	// Subject
	if ctx.Subject != "" && len(m.Metadata.Subjects) == 0 {
		subtitle := manifest.NewLocalizedStringFromString(ctx.Subject)
		m.Metadata.LocalizedSubtitle = &subtitle
	}

	// Keywords
	if ctx.Keywords != "" && len(m.Metadata.Subjects) == 0 {
		m.Metadata.Subjects = append(m.Metadata.Subjects, manifest.Subject{
			LocalizedName: manifest.NewLocalizedStringFromString(ctx.Keywords),
		})
	}

	if ctx.ModDate != "" && m.Metadata.Modified == nil {
		modDate := extensions.ParseDate(ctx.ModDate)
		if modDate != nil {
			m.Metadata.Modified = modDate
		}
	}
	if ctx.CreationDate != "" && m.Metadata.Published == nil {
		createDate := extensions.ParseDate(ctx.CreationDate)
		if createDate != nil {
			m.Metadata.Published = createDate
		}
	}

	// Bookmarks (TOC)
	if bookmarks, err := pdfcpu.Bookmarks(ctx); err == nil {
		rootLink := m.ReadingOrder.FirstWithMediaType(&mediatype.PDF)
		root := ""
		if rootLink != nil {
			root = rootLink.Href
		}
		var bf func(toc manifest.LinkList, bookmarks []pdfcpu.Bookmark)
		bf = func(toc manifest.LinkList, bookmarks []pdfcpu.Bookmark) {
			for _, b := range bookmarks {
				lnk := manifest.Link{
					Href:  fmt.Sprintf("%s#page=%d", root, b.PageFrom),
					Title: b.Title,
					Type:  mediatype.PDF.String(),
				}
				if len(b.Kids) > 0 {
					bf(lnk.Children, b.Kids)
				}
				m.TableOfContents = append(m.TableOfContents, lnk)
			}
		}
		bf(m.TableOfContents, bookmarks)
	}

	return nil
}
