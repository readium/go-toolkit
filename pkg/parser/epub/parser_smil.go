package epub

import (
	"strconv"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/guidednavigation/converter"
	"github.com/readium/go-toolkit/pkg/util/url"
)

var (
	xpSMILRoot      = mustCompileNS("/smil:smil | /smil2:smil")
	xpSMILBody      = mustCompileNS("smil:body | smil2:body")
	xpSMILParOrSeq  = mustCompileNS("smil:par | smil:seq | smil2:par | smil2:seq")
	xpSMILTextChild = mustCompileNS("smil:text | smil2:text")
	xpSMILAudio     = mustCompileNS("smil:audio | smil2:audio")
)

func ParseSMILDocument(document *xmlquery.Node, filePath url.URL) (*guidednavigation.GuidedNavigationDocument, error) {
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
	return &guidednavigation.GuidedNavigationDocument{
		Guided: seqs,
	}, nil
}

func ParseSMILSeq(seq *xmlquery.Node, filePath url.URL) ([]guidednavigation.GuidedNavigationObject, error) {
	childElements := xmlquery.QuerySelectorAll(seq, xpSMILParOrSeq)
	if len(childElements) == 0 && seq.Data == "body" {
		return nil, errors.New("SMIL body is empty")
	}
	objects := make([]guidednavigation.GuidedNavigationObject, 0, len(childElements))
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
			textrefAttr := SelectNodeAttrNs(el, NamespaceOPS, "textref")
			if textrefAttr == "" {
				return nil, errors.New("SMIL seq has no textref")
			}
			u, err := url.URLFromString(textrefAttr)
			if err != nil {
				return nil, errors.Wrap(err, "failed parsing SMIL seq textref")
			}
			o := &guidednavigation.GuidedNavigationObject{
				TextRef: filePath.Resolve(u),
			}

			// epub:type
			pp := parseProperties(SelectNodeAttrNs(el, NamespaceOPS, "type"))
			if len(pp) > 0 {
				o.Role = make([]guidednavigation.GuidedNavigationRole, 0, len(pp))
				for _, prop := range pp {
					p := converter.ConvertEPUBRole(prop)
					if p == "" {
						continue
					}
					o.Role = append(o.Role, p)
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

func ParseSMILPar(par *xmlquery.Node, filePath url.URL) (*guidednavigation.GuidedNavigationObject, error) {
	text := xmlquery.QuerySelector(par, xpSMILTextChild)
	if text == nil {
		return nil, errors.New("SMIL par has no text element")
	}
	srcAttr := text.SelectAttr("src")
	if srcAttr == "" {
		return nil, errors.New("SMIL par text element has empty src attribute")
	}
	u, err := url.URLFromString(srcAttr)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing SMIL par text element textref")
	}
	o := &guidednavigation.GuidedNavigationObject{
		TextRef: filePath.Resolve(u),
	}

	// Audio is optional
	if audio := xmlquery.QuerySelector(par, xpSMILAudio); audio != nil {
		audioAttr := audio.SelectAttr("src")
		if audioAttr == "" {
			return nil, errors.New("SMIL par audio element has empty src attribute")
		}
		begin := ParseClockValue(audio.SelectAttr("clipBegin"))
		end := ParseClockValue(audio.SelectAttr("clipEnd"))
		if begin != nil || end != nil {
			audioAttr += "#t="
		}
		if begin != nil {
			audioAttr += strconv.FormatFloat(*begin, 'f', -1, 64)
		}
		if end != nil {
			audioAttr += "," + strconv.FormatFloat(*end, 'f', -1, 64)
		}

		u, err := url.URLFromString(audioAttr)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing SMIL par audio element textref")
		}
		o.AudioRef = filePath.Resolve(u)
	}

	// epub:type
	pp := parseProperties(SelectNodeAttrNs(par, NamespaceOPS, "type"))
	if len(pp) > 0 {
		o.Role = make([]guidednavigation.GuidedNavigationRole, 0, len(pp))
		for _, prop := range pp {
			p := converter.ConvertEPUBRole(prop)
			if p == "" {
				continue
			}
			o.Role = append(o.Role, p)
		}
	}

	return o, nil
}
