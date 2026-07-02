package converter

import (
	"slices"
	"strconv"
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
		for _, at := range n.Attr {
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

var headingRoles = [6]guidednavigation.GuidedNavigationRole{
	guidednavigation.RoleHeading1,
	guidednavigation.RoleHeading2,
	guidednavigation.RoleHeading3,
	guidednavigation.RoleHeading4,
	guidednavigation.RoleHeading5,
	guidednavigation.RoleHeading6,
}

var ariaRoles = map[string]guidednavigation.GuidedNavigationRole{
	"doc-abstract":        guidednavigation.RoleAbstract,
	"doc-acknowledgments": guidednavigation.RoleAcknowledgments,
	"doc-afterword":       guidednavigation.RoleAfterword,
	"doc-appendix":        guidednavigation.RoleAppendix,
	"article":             guidednavigation.RoleArticle,
	"doc-backlink":        guidednavigation.RoleBacklink,
	"doc-bibliography":    guidednavigation.RoleBibliography,
	"doc-biblioref":       guidednavigation.RoleBiblioref,
	"blockquote":          guidednavigation.RoleBlockquote,
	"caption":             guidednavigation.RoleCaption,
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
	"doc-endnote":         guidednavigation.RoleFootnote, // Deprecated in DPUB-ARIA 1.1
	"doc-endnotes":        guidednavigation.RoleEndnotes,
	"doc-epigraph":        guidednavigation.RoleEpigraph,
	"doc-epilogue":        guidednavigation.RoleEpilogue,
	"doc-errata":          guidednavigation.RoleErrata,
	"doc-example":         guidednavigation.RoleExample,
	"figure":              guidednavigation.RoleFigure,
	"doc-footnote":        guidednavigation.RoleFootnote,
	"doc-foreword":        guidednavigation.RoleForeword,
	"doc-glossary":        guidednavigation.RoleGlossary,
	"doc-glossref":        guidednavigation.RoleGlossref,
	"img":                 guidednavigation.RoleImage,
	"image":               guidednavigation.RoleImage, // ARIA 1.3 synonym of img
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
	"paragraph":           guidednavigation.RoleParagraph,
	"doc-part":            guidednavigation.RolePart,
	"doc-preface":         guidednavigation.RolePreface,
	"doc-prologue":        guidednavigation.RolePrologue,
	"doc-pullquote":       guidednavigation.RolePullquote,
	"presentation":        guidednavigation.RolePresentation,
	"none":                guidednavigation.RolePresentation,
	"doc-qna":             guidednavigation.RoleQna,
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
	"endnote":         guidednavigation.RoleFootnote,
	"endnotes":        guidednavigation.RoleEndnotes,
	"rearnote":        guidednavigation.RoleFootnote, // Deprecated alias of endnote
	"rearnotes":       guidednavigation.RoleEndnotes, // Deprecated alias of endnotes
	"epigraph":        guidednavigation.RoleEpigraph,
	"epilogue":        guidednavigation.RoleEpilogue,
	"errata":          guidednavigation.RoleErrata,
	"example":         guidednavigation.RoleExample,
	"figure":          guidednavigation.RoleFigure,
	"footnote":        guidednavigation.RoleFootnote,
	"foreword":        guidednavigation.RoleForeword,
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
	"page-list":       guidednavigation.RolePagelist,
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
	atom.H1:         guidednavigation.RoleHeading1,
	atom.H2:         guidednavigation.RoleHeading2,
	atom.H3:         guidednavigation.RoleHeading3,
	atom.H4:         guidednavigation.RoleHeading4,
	atom.H5:         guidednavigation.RoleHeading5,
	atom.H6:         guidednavigation.RoleHeading6,
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
	atom.Svg:        guidednavigation.RoleImage,
}

// ExtractNodeRoles determines the Guided Navigation roles of an element, combining
// the roles derived from the element type itself with the ones from its ARIA `role`
// and `epub:type` attributes, e.g. <section epub:type="chapter"> -> [section, chapter].
// An ARIA role of "presentation"/"none" strips the element of its native semantics.
func ExtractNodeRoles(el *html.Node) (roles []guidednavigation.GuidedNavigationRole) {
	add := func(role guidednavigation.GuidedNavigationRole) {
		if !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}

	// Based on attributes. Both `role` and `epub:type` can hold a list of values.
	var namespaces map[string]string
	var attrRoles []guidednavigation.GuidedNavigationRole
	presentational := false
	for _, at := range el.Attr {
		// Remove namespace prefix if it exists
		frags := strings.SplitN(at.Key, ":", 2)
		key := frags[len(frags)-1]

		if len(frags) == 1 {
			// ARIA role
			if key == "role" {
				for _, val := range strings.Fields(at.Val) {
					if val == "presentation" || val == "none" {
						presentational = true
					}
					if val == "heading" {
						// The heading level comes from aria-level. It defaults to 2:
						// https://www.w3.org/TR/wai-aria/#heading
						level := 2
						if l, err := strconv.Atoi(getAttr(el, "aria-level")); err == nil && l >= 1 {
							level = min(l, 6)
						}
						attrRoles = append(attrRoles, headingRoles[level-1])
					} else if role, ok := ariaRoles[val]; ok {
						attrRoles = append(attrRoles, role)
					}
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
				for _, val := range strings.Fields(at.Val) {
					if role, ok := epubTypeRoles[val]; ok {
						attrRoles = append(attrRoles, role)
					}
				}
			}
		}
	}

	if presentational {
		// The element only retains the presentation role
		return []guidednavigation.GuidedNavigationRole{guidednavigation.RolePresentation}
	}

	// Based on element type. The element's own role comes first, followed by the
	// more specific attribute-based roles, matching the ordering of the examples
	// in the Guided Navigation specification.
	switch el.DataAtom {
	case atom.Body:
		add(guidednavigation.RoleBody)
	case atom.Th:
		switch getAttr(el, "scope") {
		case "col":
			add(guidednavigation.RoleColumnHeader)
		case "row":
			add(guidednavigation.RoleRowHeader)
		default:
			// Without an explicit scope the direction is unknown, but it's still a cell
			add(guidednavigation.RoleCell)
		}
	default:
		if role, ok := simpleElementTypeRoles[el.DataAtom]; ok {
			add(role)
		}
	}

	for _, role := range attrRoles {
		add(role)
	}

	return
}

func ConvertEPUBRole(role string) guidednavigation.GuidedNavigationRole {
	if gRole, ok := epubTypeRoles[role]; ok {
		return gRole
	}
	return ""
}
