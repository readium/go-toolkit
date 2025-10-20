package converter

import (
	"slices"
	"strings"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Crawl up the tree and extract all namespaces.
// This is technically an incorrect implementation because it doesn't distinguish between XHTML and HTML.
// These namespaces are ignored in HTML documents, but we still support them.
func ExtractNamespaces(el *html.Node) (namespaces map[string]string) {
	namespaces = map[string]string{
		"xml": "http://www.w3.org/XML/1998/namespace",
	}

	f := func(n *html.Node) {
		for _, at := range el.Attr {
			if at.Key == "xmlns" {
				if _, ok := namespaces[""]; !ok {
					// Only the first xmlns gets set
					namespaces[""] = at.Val
				}
			} else if strings.HasPrefix(at.Key, "xmlns:") {
				namespace := strings.TrimPrefix(at.Key, "xmlns:")
				if _, ok := namespaces[namespace]; !ok {
					// Only the first unique xmlns:prefix gets set
					namespaces[namespace] = at.Val
				}
			}
		}
	}

	f(el)
	if el.Type != html.ElementNode || el.DataAtom != atom.Html {
		for el.Parent != nil {
			el = el.Parent
			f(el)
			if el.Type == html.ElementNode && el.DataAtom == atom.Html {
				break
			}
		}
	}

	return
}

var ariaRoles = map[string]guidednavigation.GuidedNavigationRole{
	"doc-abstract":        guidednavigation.RoleAbstract,
	"doc-acknowledgments": guidednavigation.RoleAcknowledgments,
	"doc-afterword":       guidednavigation.RoleAfterword,
	"doc-appendix":        guidednavigation.RoleAppendix,
	"doc-backlink":        guidednavigation.RoleBacklink,
	"doc-bibliography":    guidednavigation.RoleBibliography,
	"doc-biblioref":       guidednavigation.RoleBiblioref,
	"cell":                guidednavigation.RoleCell,
	"doc-chapter":         guidednavigation.RoleChapter,
	"doc-colophon":        guidednavigation.RoleColophon,
	"columnheader":        guidednavigation.RoleColumnHeader,
	"complementary":       guidednavigation.RoleComplementary,
	"doc-conclusion":      guidednavigation.RoleConclusion,
	"doc-cover":           guidednavigation.RoleCover,
	"doc-credit":          guidednavigation.RoleCredit,
	"doc-credits":         guidednavigation.RoleCredits,
	"doc-dedication":      guidednavigation.RoleDedication,
	"definition":          guidednavigation.RoleDefinition,
	"doc-endnotes":        guidednavigation.RoleEndnotes,
	"doc-epigraph":        guidednavigation.RoleEpigraph,
	"doc-epilogue":        guidednavigation.RoleEpilogue,
	"doc-errata":          guidednavigation.RoleErrata,
	"doc-example":         guidednavigation.RoleExample,
	"figure":              guidednavigation.RoleFigure,
	"doc-footnote":        guidednavigation.RoleFootnote,
	"doc-glossary":        guidednavigation.RoleGlossary,
	"doc-glossref":        guidednavigation.RoleGlossref,
	"heading":             guidednavigation.RoleHeading,
	"img":                 guidednavigation.RoleImage,
	"doc-index":           guidednavigation.RoleIndex,
	"doc-introduction":    guidednavigation.RoleIntroduction,
	"list":                guidednavigation.RoleList,
	"listitem":            guidednavigation.RoleListItem,
	"main":                guidednavigation.RoleMain,
	"math":                guidednavigation.RoleMath,
	"navigation":          guidednavigation.RoleNavigation,
	"doc-noteref":         guidednavigation.RoleNoteref,
	"doc-notice":          guidednavigation.RoleNotice,
	"doc-pagebreak":       guidednavigation.RolePagebreak,
	"doc-pagelist":        guidednavigation.RolePagelist,
	"doc-part":            guidednavigation.RolePart,
	"doc-preface":         guidednavigation.RolePreface,
	"doc-prologue":        guidednavigation.RolePrologue,
	"doc-pullquote":       guidednavigation.RolePullquote,
	"presentation":        guidednavigation.RolePresentation,
	"none":                guidednavigation.RolePresentation,
	"qna":                 guidednavigation.RoleQna,
	"region":              guidednavigation.RoleRegion,
	"row":                 guidednavigation.RoleRow,
	"rowheader":           guidednavigation.RoleRowHeader,
	"separator":           guidednavigation.RoleSeparator,
	"doc-subtitle":        guidednavigation.RoleSubtitle,
	"table":               guidednavigation.RoleTable,
	"term":                guidednavigation.RoleTerm,
	"doc-tip":             guidednavigation.RoleTip,
	"doc-toc":             guidednavigation.RoleToc,
}

var epubTypeRoles = map[string]guidednavigation.GuidedNavigationRole{
	"abstract":        guidednavigation.RoleAbstract,
	"acknowledgments": guidednavigation.RoleAcknowledgments,
	"afterword":       guidednavigation.RoleAfterword,
	"appendix":        guidednavigation.RoleAppendix,
	"aside":           guidednavigation.RoleAside,
	"backlink":        guidednavigation.RoleBacklink,
	"bibliography":    guidednavigation.RoleBibliography,
	"biblioref":       guidednavigation.RoleBiblioref,
	"table-cell":      guidednavigation.RoleCell,
	"chapter":         guidednavigation.RoleChapter,
	"colophon":        guidednavigation.RoleColophon,
	"conclusion":      guidednavigation.RoleConclusion,
	"cover":           guidednavigation.RoleCover,
	"credit":          guidednavigation.RoleCredit,
	"credits":         guidednavigation.RoleCredits,
	"dedication":      guidednavigation.RoleDedication,
	"glossdef":        guidednavigation.RoleDefinition,
	"endnotes":        guidednavigation.RoleEndnotes,
	"epigraph":        guidednavigation.RoleEpigraph,
	"epilogue":        guidednavigation.RoleEpilogue,
	"errata":          guidednavigation.RoleErrata,
	"example":         guidednavigation.RoleExample,
	"figure":          guidednavigation.RoleFigure,
	"footnote":        guidednavigation.RoleFootnote,
	"glossary":        guidednavigation.RoleGlossary,
	"glossref":        guidednavigation.RoleGlossref,
	"index":           guidednavigation.RoleIndex,
	"introduction":    guidednavigation.RoleIntroduction,
	"landmarks":       guidednavigation.RoleLandmarks,
	"list":            guidednavigation.RoleList,
	"list-item":       guidednavigation.RoleListItem,
	"loa":             guidednavigation.RoleLoa,
	"loi":             guidednavigation.RoleLoi,
	"lot":             guidednavigation.RoleLot,
	"lov":             guidednavigation.RoleLov,
	"noteref":         guidednavigation.RoleNoteref,
	"notice":          guidednavigation.RoleNotice,
	"pagebreak":       guidednavigation.RolePagebreak,
	"pagelist":        guidednavigation.RolePagelist,
	"part":            guidednavigation.RolePart,
	"preface":         guidednavigation.RolePreface,
	"prologue":        guidednavigation.RolePrologue,
	"pullquote":       guidednavigation.RolePullquote,
	"qna":             guidednavigation.RoleQna,
	"table-row":       guidednavigation.RoleRow,
	"subtitle":        guidednavigation.RoleSubtitle,
	"table":           guidednavigation.RoleTable,
	"glossterm":       guidednavigation.RoleTerm,
	"tip":             guidednavigation.RoleTip,
	"toc":             guidednavigation.RoleToc,
}

var simpleElementTypeRoles = map[atom.Atom]guidednavigation.GuidedNavigationRole{
	atom.Article:    guidednavigation.RoleArticle,
	atom.Aside:      guidednavigation.RoleAside,
	atom.Audio:      guidednavigation.RoleAudio,
	atom.Blockquote: guidednavigation.RoleBlockquote,
	atom.Caption:    guidednavigation.RoleCaption,
	atom.Figcaption: guidednavigation.RoleCaption,
	atom.Td:         guidednavigation.RoleCell,
	atom.Dd:         guidednavigation.RoleDefinition,
	atom.Details:    guidednavigation.RoleDetails,
	atom.Figure:     guidednavigation.RoleFigure,
	atom.Header:     guidednavigation.RoleHeader,
	atom.H1:         guidednavigation.RoleHeading,
	atom.H2:         guidednavigation.RoleHeading,
	atom.H3:         guidednavigation.RoleHeading,
	atom.H4:         guidednavigation.RoleHeading,
	atom.H5:         guidednavigation.RoleHeading,
	atom.H6:         guidednavigation.RoleHeading,
	atom.Img:        guidednavigation.RoleImage,
	atom.Ul:         guidednavigation.RoleList,
	atom.Ol:         guidednavigation.RoleList,
	atom.Li:         guidednavigation.RoleListItem,
	atom.Main:       guidednavigation.RoleMain,
	atom.Math:       guidednavigation.RoleMath,
	atom.Nav:        guidednavigation.RoleNavigation,
	atom.P:          guidednavigation.RoleParagraph,
	atom.Pre:        guidednavigation.RolePreformatted,
	atom.Tr:         guidednavigation.RoleRow,
	atom.Section:    guidednavigation.RoleSection,
	atom.Hr:         guidednavigation.RoleSeparator,
	atom.Summary:    guidednavigation.RoleSummary,
	atom.Table:      guidednavigation.RoleTable,
	atom.Dfn:        guidednavigation.RoleTerm,
	atom.Dt:         guidednavigation.RoleTerm,
	atom.Video:      guidednavigation.RoleVideo,
}

func ExtractNodeRoles(el *html.Node) (roles []guidednavigation.GuidedNavigationRole, level uint8) {
	add := func(role guidednavigation.GuidedNavigationRole) {
		if !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}

	// Based on attributes
	var namespaces map[string]string
	var alreadyHasRole bool
	for _, at := range el.Attr {

		// Remove namespace prefix if it exists
		frags := strings.SplitN(at.Key, ":", 2)
		key := frags[len(frags)-1]

		if len(frags) == 1 {
			// ARIA role
			if key == "role" {
				if role, ok := ariaRoles[at.Val]; ok {
					alreadyHasRole = true
					add(role)
				}
			}
		} else {
			// First we check for an attribute key we're interested in, because extracting namespaces is expensive
			if key != "type" {
				continue
			}

			// Maybe the attribute has a namespace...?
			if at.Namespace == "" {
				if namespaces == nil {
					// Save namespaces so they're only extracted once per element
					namespaces = ExtractNamespaces(el)
				}
				if namespace, ok := namespaces[frags[0]]; ok {
					// Set the namespace if we found it
					at.Namespace = namespace
				}
			}

			if at.Namespace == "http://www.idpf.org/2007/ops" && key == "type" {
				if role, ok := epubTypeRoles[at.Val]; ok {
					add(role)
				}
			}
		}
	}
	if alreadyHasRole {
		// Aria role overrides logic based on the element type
		return
	}

	// Based on element type
	switch el.DataAtom {
	case atom.Th:
		scope := getAttr(el, "scope")
		switch scope {
		case "col":
			add(guidednavigation.RoleColumnHeader)
		case "row":
			add(guidednavigation.RoleRowHeader)
		}
	default:
		if role, ok := simpleElementTypeRoles[el.DataAtom]; ok {
			add(role)

			switch role {
			case guidednavigation.RoleHeading:
				switch el.DataAtom {
				case atom.H1:
					level = 1
				case atom.H2:
					level = 2
				case atom.H3:
					level = 3
				case atom.H4:
					level = 4
				case atom.H5:
					level = 5
				case atom.H6:
					level = 6
				}
			}
		}
	}

	/*case atom.Blockquote, atom.Q:
	quote := element.Quote{}
	for _, at := range el.Attr {
		if at.Key == "cite" {
			quote.ReferenceURL, _ = nurl.Parse(at.Val)
		}
		if at.Key == "title" {
			quote.ReferenceTitle = at.Val
		}
	}
	bestRole = quote*/ // TODO

	return
}

func ConvertEPUBRole(role string) guidednavigation.GuidedNavigationRole {
	if gRole, ok := epubTypeRoles[role]; ok {
		return gRole
	}
	return ""
}
