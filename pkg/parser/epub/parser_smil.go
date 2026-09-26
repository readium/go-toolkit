package epub

import (
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/readium/go-toolkit/pkg/guidednavigation/converter"
	"github.com/readium/go-toolkit/pkg/util/url"
)

// The previous XPath unions select the SMIL namespace before SMIL 2, even
// when a SMIL 2 sibling appears earlier in the document. Keep that precedence
// while avoiding the XPath union's costly node hashing and deduplication.
func firstSMILChild(parent *xmlquery.Node, name string) *xmlquery.Node {
	if child := firstNavigationChild(parent, NamespaceSMIL, name); child != nil {
		return child
	}
	return firstNavigationChild(parent, NamespaceSMIL2, name)
}

func ParseSMILDocument(document *xmlquery.Node, filePath url.URL) (*guidednavigation.GuidedNavigationDocument, error) {
	smil := firstSMILChild(document, "smil")
	if smil == nil {
		return nil, errors.New("SMIL root element not found")
	}

	// Ignore the <head>, we don't need it with the current implementation

	body := firstSMILChild(smil, "body")
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
	childCount := 0
	for child := seq.FirstChild; child != nil; child = child.NextSibling {
		if child.Data == "par" || child.Data == "seq" {
			if matchesNavigationElement(child, NamespaceSMIL, child.Data) ||
				matchesNavigationElement(child, NamespaceSMIL2, child.Data) {
				childCount++
			}
		}
	}
	if childCount == 0 && seq.Data == "body" {
		return nil, errors.New("SMIL body is empty")
	}
	objects := make([]guidednavigation.GuidedNavigationObject, 0, childCount)
	// Match the original union's branch order: SMIL par, SMIL seq, SMIL 2
	// par, then SMIL 2 seq. Within each branch, retain sibling order.
	for _, namespace := range [...]string{NamespaceSMIL, NamespaceSMIL2} {
		for _, name := range [...]string{"par", "seq"} {
			for child := seq.FirstChild; child != nil; child = child.NextSibling {
				if !matchesNavigationElement(child, namespace, name) {
					continue
				}
				o, err := parseSMILSequenceChild(child, filePath)
				if err != nil {
					return nil, err
				}
				objects = append(objects, *o)
			}
		}
	}
	return objects, nil
}

func parseSMILSequenceChild(el *xmlquery.Node, filePath url.URL) (*guidednavigation.GuidedNavigationObject, error) {
	if el.Data == "par" {
		o, err := ParseSMILPar(el, filePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed parsing SMIL par")
		}
		return o, nil
	}

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

	children, err := ParseSMILSeq(el, filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed parsing SMIL seq children")
	}
	o.Children = children
	return o, nil
}

func secondsToDuration(seconds *float64) *time.Duration {
	if seconds == nil {
		return nil
	}
	d := time.Duration(*seconds * float64(time.Second))
	return &d
}

func ParseSMILPar(par *xmlquery.Node, filePath url.URL) (*guidednavigation.GuidedNavigationObject, error) {
	text := firstSMILChild(par, "text")
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
	if audio := firstSMILChild(par, "audio"); audio != nil {
		audioAttr := audio.SelectAttr("src")
		if audioAttr == "" {
			return nil, errors.New("SMIL par audio element has empty src attribute")
		}
		clip := guidednavigation.Clip{
			Begin: secondsToDuration(ParseClockValue(audio.SelectAttr("clipBegin"))),
			End:   secondsToDuration(ParseClockValue(audio.SelectAttr("clipEnd"))),
		}
		if fragment := clip.MediaFragment(); fragment != "" {
			audioAttr += "#" + fragment
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
