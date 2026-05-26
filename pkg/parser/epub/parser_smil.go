package epub

import (
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpSMILRoot      = mustCompileNS("/smil:smil | /smil2:smil")
	xpSMILBody      = mustCompileNS("smil:body | smil2:body")
	xpSMILParOrSeq  = mustCompileNS("smil:par | smil:seq | smil2:par | smil2:seq")
	xpSMILTextChild = mustCompileNS("smil:text | smil2:text")
	xpSMILAudio     = mustCompileNS("smil:audio | smil2:audio")
)

func ParseSMILDocument(document *xmlquery.Node, filePath url.URL) (*manifest.GuidedNavigationDocument, error) {
	smil := xmlquery.QuerySelector(document, xpSMILRoot)
	if smil == nil {
		return nil, errors.New("SMIL root element not found")
	}

	// Ignore the <head>, we don't need it with the current implementation

	body := xmlquery.QuerySelector(smil, xpSMILBody)
	if body == nil {
		return nil, errors.New("SMIL body not found")
	}

	seqs, err := ParseSMILSeq(body, filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing SMIL body")
	}
	return &manifest.GuidedNavigationDocument{
		Guided: seqs,
	}, nil
}

func ParseSMILSeq(seq *xmlquery.Node, filePath url.URL) ([]manifest.GuidedNavigationObject, error) {
	childElements := xmlquery.QuerySelectorAll(seq, xpSMILParOrSeq)
	if len(childElements) == 0 && seq.Data == "body" {
		return nil, errors.New("SMIL body is empty")
	}
	objects := make([]manifest.GuidedNavigationObject, 0, len(childElements))
	for _, el := range childElements {
		if el.Data == "par" {
			// <par>
			o, err := ParseSMILPar(el, filePath)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing SMIL par")
			}
			objects = append(objects, *o)
		} else {
			// <seq>
			o := &manifest.GuidedNavigationObject{
				TextRef: SelectNodeAttrNs(el, NamespaceOPS, "textref"),
			}
			if o.TextRef == "" {
				return nil, errors.New("SMIL seq has no textref")
			}
			u, err := url.URLFromString(o.TextRef)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing SMIL seq textref")
			}
			o.TextRef = filePath.Resolve(u).String()

			// epub:type
			pp := parseProperties(SelectNodeAttrNs(el, NamespaceOPS, "type"))
			if len(pp) > 0 {
				o.Role = make([]string, 0, len(pp))
				for _, prop := range pp {
					if prop == "" {
						continue
					}
					o.Role = append(o.Role, prop)
				}
			}

			// <seq> child elements
			children, err := ParseSMILSeq(el, filePath)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing SMIL seq children")
			}
			o.Children = children
			objects = append(objects, *o)
		}
	}
	return objects, nil
}

func ParseSMILPar(par *xmlquery.Node, filePath url.URL) (*manifest.GuidedNavigationObject, error) {
	text := xmlquery.QuerySelector(par, xpSMILTextChild)
	if text == nil {
		return nil, errors.New("SMIL par has no text element")
	}
	o := &manifest.GuidedNavigationObject{
		TextRef: text.SelectAttr("src"),
	}
	if o.TextRef == "" {
		return nil, errors.New("SMIL par text element has empty src attribute")
	}
	u, err := url.URLFromString(o.TextRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing SMIL par text element textref")
	}
	o.TextRef = filePath.Resolve(u).String()

	// Audio is optional
	if audio := xmlquery.QuerySelector(par, xpSMILAudio); audio != nil {
		o.AudioRef = audio.SelectAttr("src")
		if o.AudioRef == "" {
			return nil, errors.New("SMIL par audio element has empty src attribute")
		}
		begin := ParseClockValue(audio.SelectAttr("clipBegin"))
		end := ParseClockValue(audio.SelectAttr("clipEnd"))
		if begin != nil || end != nil {
			o.AudioRef += "#t="
		}
		if begin != nil {
			o.AudioRef += strconv.FormatFloat(*begin, 'f', -1, 64)
		}
		if end != nil {
			o.AudioRef += "," + strconv.FormatFloat(*end, 'f', -1, 64)
		}

		u, err := url.URLFromString(o.AudioRef)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing SMIL par audio element textref")
		}
		o.AudioRef = filePath.Resolve(u).String()
	}

	// epub:type
	pp := parseProperties(SelectNodeAttrNs(par, NamespaceOPS, "type"))
	if len(pp) > 0 {
		o.Role = make([]string, 0, len(pp))
		for _, prop := range pp {
			if prop == "" {
				continue
			}
			o.Role = append(o.Role, prop)
		}
	}

	return o, nil
}
