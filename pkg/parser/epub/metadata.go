package epub

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/readium/xmlquery"
)

type Title struct {
	value      manifest.LocalizedString
	fileAs     *manifest.LocalizedString
	typ        string
	displaySeq *int
}

type EPUBLink struct {
	href       url.URL
	rels       []string // set
	mediaType  *mediatype.MediaType
	refines    string
	properties []string
}

type EPUBMetadata struct {
	global map[string][]MetadataItem
	refine map[string]map[string][]MetadataItem
	links  []EPUBLink
}

// Reference: https://github.com/readium/architecture/blob/master/streamer/parser/metadata.md
type MetadataParser struct {
	epubVersion float64
	prefixMap   map[string]string

	metaLanguage    string
	packageLanguage string
}

func NewMetadataParser(epubVersion float64, prefixMap map[string]string) MetadataParser {
	return MetadataParser{
		epubVersion: epubVersion,
		prefixMap:   prefixMap,
	}
}

func (m MetadataParser) Parse(document *xmlquery.Node, filePath url.URL) *EPUBMetadata {
	// Init lang
	if l := document.SelectElement("/" + NSSelect(NamespaceOPF, "package")); l != nil {
		for _, attr := range l.Attr {
			if attr.Name.Local == "lang" {
				m.packageLanguage = attr.Value
			}
		}
	}
	if l := document.SelectElement(
		"//" + NSSelect(NamespaceOPF, "metadata") + "/" + NSSelect(NamespaceDC, "language"),
	); l != nil {
		m.metaLanguage = strings.TrimSpace(l.InnerText())
	}

	metadata := document.SelectElement(
		"//" + NSSelect(NamespaceOPF, "metadata"),
	)
	if metadata == nil {
		return nil
	}

	metas, links := m.parseElements(metadata, filePath)
	allMetas := m.resolveMetaHierarchy(metas)
	var globalMetas []MetadataItem
	var refineMetas []MetadataItem
	for _, meta := range allMetas {
		if meta.refines == "" {
			globalMetas = append(globalMetas, meta)
		} else {
			refineMetas = append(refineMetas, meta)
		}
	}

	globalCollection := make(map[string][]MetadataItem)
	for _, meta := range globalMetas {
		globalCollection[meta.property] = append(globalCollection[meta.property], meta)
	}
	refineCollections := make(map[string]map[string][]MetadataItem)
	for _, meta := range refineMetas {
		if _, ok := refineCollections[meta.refines]; !ok {
			refineCollections[meta.refines] = make(map[string][]MetadataItem)
		}
		refineCollections[meta.refines][meta.property] = append(refineCollections[meta.refines][meta.property], meta)
	}

	return &EPUBMetadata{
		global: globalCollection,
		refine: refineCollections,
		links:  links,
	}
}

// Determines the BCP-47 language tag for the given element, using:
// 1. its xml:lang attribute
// 2. the package's xml:lang attribute
// 3. the primary language for the publication
func (m MetadataParser) language(element *xmlquery.Node) string {
	lang := ""
	for _, attr := range element.Attr {
		if attr.Name.Local == "lang" {
			lang = attr.Value
		}
	}

	if lang != "" {
		return lang
	}
	if m.packageLanguage != "" {
		return m.packageLanguage
	}
	return m.metaLanguage
}

func (m MetadataParser) parseElements(metadataElement *xmlquery.Node, filePath url.URL) ([]MetadataItem, []EPUBLink) {
	var metas []MetadataItem
	var links []EPUBLink

	for element := metadataElement.FirstChild; element != nil; element = element.NextSibling {
		if element.NamespaceURI == NamespaceDC {
			m := m.parseDcElement(element)
			if m != nil {
				metas = append(metas, *m)
			}
		} else if element.NamespaceURI == NamespaceOPF && element.Data == "meta" {
			m := m.parseMetaElement(element)
			if m != nil {
				metas = append(metas, *m)
			}
		} else if element.NamespaceURI == NamespaceOPF && element.Data == "link" {
			l := m.parseLinkElement(element, filePath)
			if l != nil {
				links = append(links, *l)
			}
		}
	}

	return metas, links
}

func (m MetadataParser) parseLinkElement(element *xmlquery.Node, filePath url.URL) *EPUBLink {
	if element == nil {
		return nil
	}
	href := element.SelectAttr("href")
	if href == "" {
		return nil
	}
	u, err := url.FromEPUBHref(href)
	if err != nil {
		return nil
	}
	hr := filePath.Resolve(u)

	link := &EPUBLink{
		href:      hr,
		mediaType: mediatype.MaybeNewOfString(element.SelectAttr("media-type")),
		refines:   strings.TrimPrefix(element.SelectAttr("refines"), "#"),
	}

	relAttr := element.SelectAttr("rel")
	for _, v := range parseProperties(relAttr) {
		link.rels = append(link.rels, resolveProperty(v, m.prefixMap, DefaultVocabLink))
	}

	propAttrs := element.SelectAttr("properties")
	for _, v := range parseProperties(propAttrs) {
		link.properties = append(link.properties, resolveProperty(v, m.prefixMap, DefaultVocabLink))
	}

	return link
}

func (m MetadataParser) parseMetaElement(element *xmlquery.Node) *MetadataItem {
	if element == nil {
		return nil
	}
	property := element.SelectAttr("property")
	if property == "" {
		name := strings.TrimSpace(element.SelectAttr("name"))
		if name == "" {
			return nil
		}
		content := strings.TrimSpace(element.SelectAttr("content"))
		if content == "" {
			return nil
		}
		resolvedName := resolveProperty(name, m.prefixMap, DefaultVocabMeta)
		return &MetadataItem{
			property: resolvedName,
			value:    content,
			lang:     m.language(element),
			id:       element.SelectAttr("id"),
		}
	} else {
		propName := strings.TrimSpace(property)
		if propName == "" {
			return nil
		}
		propValue := strings.TrimSpace(element.InnerText())
		if propValue == "" {
			return nil
		}
		resolvedScheme := strings.TrimSpace(element.SelectAttr("scheme"))
		if resolvedScheme != "" {
			resolvedScheme = resolveProperty(resolvedScheme, m.prefixMap, DefaultVocabMeta)
		}
		return &MetadataItem{
			property: resolveProperty(propName, m.prefixMap, DefaultVocabMeta),
			value:    propValue,
			lang:     m.language(element),
			refines:  strings.TrimPrefix(element.SelectAttr("refines"), "#"),
			scheme:   resolvedScheme,
			id:       element.SelectAttr("id"),
		}
	}
}

func (m MetadataParser) parseDcElement(element *xmlquery.Node) *MetadataItem {
	if element == nil {
		return nil
	}
	propValue := strings.TrimSpace(element.InnerText())
	if propValue == "" {
		return nil
	}

	data := strings.ToLower(element.Data)
	propName := VocabularyDCTerms + element.Data
	switch data {
	case "creator", "contributor", "publisher":
		c := m.contributorWithLegacyAttr(element, propName, propValue)
		return &c
	case "date":
		d := m.dateWithLegacyAttr(element, propName, propValue)
		return &d
	default:
		return &MetadataItem{
			property: propName,
			value:    propValue,
			lang:     m.language(element),
			id:       element.SelectAttr("id"),
		}
	}
}

func (m MetadataParser) contributorWithLegacyAttr(element *xmlquery.Node, name string, value string) MetadataItem {
	mi := MetadataItem{
		property: name,
		value:    value,
		lang:     m.language(element),
		id:       element.SelectAttr("id"),
		children: make(map[string][]MetadataItem),
	}

	fileAs := SelectNodeAttrNs(element, NamespaceOPF, "file-as")
	if fileAs != "" {
		mi.children[VocabularyMeta+"file-as"] = []MetadataItem{
			{
				property: VocabularyMeta + "file-as",
				value:    fileAs,
				lang:     m.language(element),
				id:       element.SelectAttr("id"),
			},
		}
	}

	role := SelectNodeAttrNs(element, NamespaceOPF, "role")
	if role != "" {
		mi.children[VocabularyMeta+"role"] = []MetadataItem{
			{
				property: VocabularyMeta + "role",
				value:    role,
				lang:     m.language(element),
				id:       element.SelectAttr("id"),
			},
		}
	}

	return mi
}

func (m MetadataParser) dateWithLegacyAttr(element *xmlquery.Node, name string, value string) MetadataItem {
	eventAttr := SelectNodeAttrNs(element, NamespaceOPF, "event")
	propName := name
	if eventAttr == "modification" {
		propName = VocabularyDCTerms + "modified"
	}
	return MetadataItem{
		property: propName,
		value:    value,
		lang:     m.language(element),
		id:       element.SelectAttr("id"),
	}
}

func (m MetadataParser) resolveMetaHierarchy(items []MetadataItem) []MetadataItem {
	metadataIds := make(map[string]struct{})
	for _, item := range items {
		if item.id != "" {
			metadataIds[item.id] = struct{}{}
		}
	}
	var rootExpr []MetadataItem
	for _, item := range items {
		if item.refines == "" {
			rootExpr = append(rootExpr, item)
		} else {
			if _, ok := metadataIds[item.refines]; !ok {
				rootExpr = append(rootExpr, item)
			}
		}
	}
	exprByRefines := make(map[string][]MetadataItem)
	for _, item := range items {
		exprByRefines[item.refines] = append(exprByRefines[item.refines], item)
	}

	ret := make([]MetadataItem, len(rootExpr))
	for i, item := range rootExpr {
		ret[i] = m.computeMetaItem(item, exprByRefines, map[string]struct{}{})
	}
	return ret
}

func (m MetadataParser) computeMetaItem(expr MetadataItem, metas map[string][]MetadataItem, chain map[string]struct{}) MetadataItem {
	updatedChain := chain
	var newChildren []MetadataItem
	if expr.id != "" {
		updatedChain[expr.id] = struct{}{}

		ms, ok := metas[expr.id]
		if ok {
			for _, meta := range ms {
				if _, ok := updatedChain[meta.id]; !ok {
					newChildren = append(newChildren, m.computeMetaItem(meta, metas, updatedChain))
				}
			}
		}
	}

	children := make(map[string][]MetadataItem)
	for k, v := range expr.children {
		children[k] = v
	}
	for _, child := range newChildren {
		children[child.property] = append(children[child.property], child)
	}

	return MetadataItem{
		property:      expr.property,
		value:         expr.value,
		lang:          expr.lang,
		scheme:        expr.scheme,
		refines:       expr.refines,
		id:            expr.id,
		children:      children,
		otherMetadata: expr.otherMetadata,
	}
}

type metadataAdapter struct {
	epubVersion float64
	items       map[string][]MetadataItem
	links       []EPUBLink
}

func (m metadataAdapter) Duration() *float64 {
	return ParseClockValue(m.FirstValue(VocabularyMedia + "duration"))
}

func (m metadataAdapter) First(property string) (item MetadataItem, ok bool) {
	items, ok := m.items[property]
	if !ok || len(items) == 0 {
		return
	}
	item = items[0]
	return
}

func (m metadataAdapter) FirstValue(property string) string {
	item, ok := m.First(property)
	if !ok {
		return ""
	}
	return item.value
}

func itemsValues(items []MetadataItem) []string {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = item.value
	}
	return values
}

func (m metadataAdapter) Values(property string) []string {
	var values []string
	if items, ok := m.items[property]; ok {
		values = itemsValues(items)
	}
	return values
}

func (m metadataAdapter) Links(rel string) []EPUBLink {
	links := []EPUBLink{}
	for _, link := range m.links {
		if extensions.Contains(link.rels, rel) {
			links = append(links, link)
		}
	}
	return links
}

func (m metadataAdapter) FirstLink(rel string) (EPUBLink, bool) {
	for _, link := range m.links {
		if extensions.Contains(link.rels, rel) {
			return link, true
		}
	}
	return EPUBLink{}, false
}

func (m metadataAdapter) FirstLinkRefining(rel string, refinedID string) (EPUBLink, bool) {
	for _, link := range m.links {
		if extensions.Contains(link.rels, rel) && link.refines == refinedID {
			return link, true
		}
	}
	return EPUBLink{}, false
}

type LinkMetadataAdapter = metadataAdapter

type PubMetadataAdapter struct {
	metadataAdapter
	fallbackTitle      string
	uniqueIdentifierID string
	readingProgression manifest.ReadingProgression
	displayOptions     map[string]string
	_identifier        string
	_altIdentifiers    []manifest.AltIdentifier

	// Title data
	_titlesSeeded      bool
	_localizedTitle    manifest.LocalizedString
	_localizedSubtitle *manifest.LocalizedString
	_localizedSortAs   *manifest.LocalizedString

	// BelongsTo data
	_belongsToSeeded      bool
	_belongsToSeries      []manifest.Collection
	_belongsToCollections []manifest.Collection

	_subjects        []manifest.Subject
	_allContributors map[string][]manifest.Contributor
	_layout          manifest.Layout
	_otherMetadata   map[string]interface{}
}

func (m PubMetadataAdapter) Metadata() manifest.Metadata {
	identifier, altIdentifiers := m.Identifiers()
	metadata := manifest.Metadata{
		Identifier:         identifier,
		AltIdentifiers:     altIdentifiers,
		ConformsTo:         manifest.Profiles{manifest.ProfileEPUB},
		Modified:           m.Modified(),
		Published:          m.Published(),
		Languages:          m.Languages(),
		LocalizedTitle:     m.LocalizedTitle(),
		LocalizedSortAs:    m.LocalizedSortAs(),
		LocalizedSubtitle:  m.LocalizedSubtitle(),
		Accessibility:      m.Accessibility(),
		TDM:                m.TDM(),
		Duration:           m.Duration(),
		Subjects:           m.Subjects(),
		Description:        m.Description(),
		ReadingProgression: m.ReadingProgression(),
		Layout:             m.Layout(),
		MediaOverlay:       m.MediaOverlay(),
		BelongsTo:          make(map[string]manifest.Contributors),
		OtherMetadata:      m.OtherMetadata(),

		Authors:      m.Contributors("aut"),
		Translators:  m.Contributors("trl"),
		Editors:      m.Contributors("edt"),
		Publishers:   m.Contributors("pbl"),
		Artists:      m.Contributors("art"),
		Illustrators: m.Contributors("ill"),
		Colorists:    m.Contributors("clr"),
		Narrators:    m.Contributors("nrt"),
		Contributors: m.Contributors(""),
	}

	cc := m.BelongsToCollections()
	if len(cc) > 0 {
		metadata.BelongsTo["collection"] = cc
	}
	cs := m.BelongsToSeries()
	if len(cs) > 0 {
		metadata.BelongsTo["series"] = cs
	}

	return metadata
}

func (m PubMetadataAdapter) Languages() []string {
	ix, ok := m.items[VocabularyDCTerms+"language"]
	if !ok {
		return nil
	}
	var languages []string
	for _, v := range ix {
		languages = append(languages, v.value)
	}
	return languages
}

func (m *PubMetadataAdapter) Identifiers() (string, []manifest.AltIdentifier) {
	if m._identifier != "" || len(m._altIdentifiers) > 0 {
		return m._identifier, m._altIdentifiers
	}
	identifiers, ok := m.items[VocabularyDCTerms+"identifier"]
	if !ok || len(identifiers) == 0 {
		return "", nil
	}

	for _, v := range identifiers {
		if v.id == m.uniqueIdentifierID {
			m._identifier = v.value
		} else {
			m._altIdentifiers = append(m._altIdentifiers, manifest.AltIdentifier{
				Value:  v.value,
				Scheme: v.scheme,
			})
		}
	}
	if m._identifier == "" {
		// Fall back to first identifier if no unique identifier was found
		m._identifier = identifiers[0].value
	}
	return m._identifier, m._altIdentifiers
}

func (m PubMetadataAdapter) Published() *time.Time {
	return extensions.ParseDate(m.FirstValue(VocabularyDCTerms + "date"))
}

func (m PubMetadataAdapter) Modified() *time.Time {
	return extensions.ParseDate(m.FirstValue(VocabularyDCTerms + "modified"))
}

func (m PubMetadataAdapter) Description() string {
	return m.FirstValue(VocabularyDCTerms + "description")
}

func (m PubMetadataAdapter) Cover() string {
	return m.FirstValue(VocabularyMeta + "cover")
}

func (m *PubMetadataAdapter) seedTitleData() {
	if m._titlesSeeded {
		return
	}
	var titles []Title
	for _, v := range m.items[VocabularyDCTerms+"title"] {
		if v, err := v.ToTitle(); err == nil {
			titles = append(titles, *v)
		}
	}

	// Title
	var mainTitle *Title
	if len(titles) > 0 {
		for _, t := range titles {
			if t.typ == "main" {
				mainTitle = &t
				break
			}
		}
		if mainTitle == nil {
			mainTitle = &titles[0]
		}
	}
	if mainTitle != nil {
		m._localizedTitle = (*mainTitle).value
	}
	if m._localizedTitle.String() == "" {
		m._localizedTitle = manifest.NewLocalizedStringFromString(m.fallbackTitle)
	}

	// Subtitle
	var tt []Title
	for _, title := range titles {
		if title.typ != "subtitle" {
			continue
		}
		tt = append(tt, title)
	}
	if len(tt) > 0 {
		sort.Slice(tt, func(i, j int) bool {
			return nilIntOrZero(tt[i].displaySeq) < nilIntOrZero(tt[j].displaySeq)
		})
		m._localizedSubtitle = &tt[0].value
	}

	// SortAs
	if mainTitle != nil && mainTitle.fileAs != nil {
		m._localizedSortAs = mainTitle.fileAs
	} else {
		s := m.FirstValue("calibre:title_sort")
		if s != "" {
			lss := manifest.NewLocalizedStringFromString(s)
			m._localizedSortAs = &lss
		}
	}

	m._titlesSeeded = true
}

func (m PubMetadataAdapter) LocalizedTitle() manifest.LocalizedString {
	m.seedTitleData()
	return m._localizedTitle
}

func (m PubMetadataAdapter) LocalizedSubtitle() *manifest.LocalizedString {
	m.seedTitleData()
	return m._localizedSubtitle
}

func (m PubMetadataAdapter) LocalizedSortAs() *manifest.LocalizedString {
	m.seedTitleData()
	return m._localizedSortAs
}

func (m PubMetadataAdapter) Accessibility() *manifest.A11y {
	a11y := manifest.NewA11y()
	ct, refinedA11y := m.a11yConformsTo()
	a11y.ConformsTo = ct

	certifierItem, _ := m.First(VocabularyA11Y + "certifiedBy")
	a11y.Certification = m.a11yCertification(certifierItem)

	a11y.Summary = m.FirstValue(VocabularySchema + "accessibilitySummary")

	modeValues := m.Values(VocabularySchema + "accessMode")
	a11y.AccessModes = valuesToAccessModes(modeValues)

	sufficientValues := m.Values(VocabularySchema + "accessModeSufficient")
	a11y.AccessModesSufficient = valuesToAccessModesSufficient(sufficientValues)

	featureValues := m.Values(VocabularySchema + "accessibilityFeature")
	a11y.Features = valuesToA11yFeatures(featureValues)

	hazardValues := m.Values(VocabularySchema + "accessibilityHazard")
	a11y.Hazards = valuesToA11yHazards(hazardValues)

	exemptionValues := m.Values(VocabularyA11Y + "exemption")
	a11y.Exemptions = valuesToA11yExemptions(exemptionValues)

	if a11y.IsEmpty() {
		return nil
	}

	// ConformsTo refinements merge with the main a11y data
	if !refinedA11y.IsEmpty() {
		a11y.Merge(&refinedA11y)
	}

	return &a11y
}

// https://www.w3.org/community/reports/tdmrep/CG-FINAL-tdmrep-20240510/#sec-epub3
func (m PubMetadataAdapter) TDM() *manifest.TDM {
	tdm := &manifest.TDM{}
	tdm.Policy = m.FirstValue(VocabularyTDM + "policy")

	reservation := m.FirstValue(VocabularyTDM + "reservation")
	switch reservation {
	case "1":
		tdm.Reservation = manifest.TDMReservationAll
	case "0":
		tdm.Reservation = manifest.TDMReservationNone
	}

	if tdm.IsEmpty() {
		return nil
	}
	return tdm
}

func (m PubMetadataAdapter) a11yConformsTo() ([]manifest.A11yProfile, manifest.A11y) {
	profiles := manifest.A11yProfileList{}
	a11y := manifest.NewA11y()

	if items, ok := m.items[VocabularyDCTerms+"conformsTo"]; ok {
		for _, item := range items {
			if profile := a11yProfile(item.value); profile != "" {
				profiles = append(profiles, profile)
				for k, v := range item.children {
					switch k {
					case VocabularyA11Y + "certifiedBy":
						a11y.Certification = m.a11yCertification(v[0])
					case VocabularySchema + "accessibilitySummary":
						a11y.Summary = v[0].value
					case VocabularySchema + "accessMode":
						a11y.AccessModes = valuesToAccessModes(itemsValues(v))
					case VocabularySchema + "accessModeSufficient":
						a11y.AccessModesSufficient = valuesToAccessModesSufficient(itemsValues(v))
					case VocabularySchema + "accessibilityFeature":
						a11y.Features = valuesToA11yFeatures(itemsValues(v))
					case VocabularySchema + "accessibilityHazard":
						a11y.Hazards = valuesToA11yHazards(itemsValues(v))
					case VocabularyA11Y + "exemption":
						a11y.Exemptions = valuesToA11yExemptions(itemsValues(v))
					}
				}
			}
		}
	}

	for _, link := range m.Links(VocabularyDCTerms + "conformsTo") {
		if profile := a11yProfile(link.href.String()); profile != "" {
			profiles = append(profiles, profile)
		}
	}

	profiles.Sort()
	return profiles, a11y
}

func a11yProfile(value string) manifest.A11yProfile {
	switch value {
	case "http://idpf.org/epub/a11y/accessibility-20170105.html#wcag-a",
		"http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a",
		"https://idpf.org/epub/a11y/accessibility-20170105.html#wcag-a",
		"https://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-a":
		return manifest.EPUBA11y10WCAG20A

	case "http://idpf.org/epub/a11y/accessibility-20170105.html#wcag-aa",
		"http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aa",
		"https://idpf.org/epub/a11y/accessibility-20170105.html#wcag-aa",
		"https://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aa":
		return manifest.EPUBA11y10WCAG20AA

	case "http://idpf.org/epub/a11y/accessibility-20170105.html#wcag-aaa",
		"http://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aaa",
		"https://idpf.org/epub/a11y/accessibility-20170105.html#wcag-aaa",
		"https://www.idpf.org/epub/a11y/accessibility-20170105.html#wcag-aaa":
		return manifest.EPUBA11y10WCAG20AAA
	case "EPUB Accessibility 1.1 - WCAG 2.0 Level A":
		return manifest.EPUBA11y11WCAG20A
	case "EPUB Accessibility 1.1 - WCAG 2.0 Level AA":
		return manifest.EPUBA11y11WCAG20AA
	case "EPUB Accessibility 1.1 - WCAG 2.0 Level AAA":
		return manifest.EPUBA11y11WCAG20AAA
	case "EPUB Accessibility 1.1 - WCAG 2.1 Level A":
		return manifest.EPUBA11y11WCAG21A
	case "EPUB Accessibility 1.1 - WCAG 2.1 Level AA":
		return manifest.EPUBA11y11WCAG21AA
	case "EPUB Accessibility 1.1 - WCAG 2.1 Level AAA":
		return manifest.EPUBA11y11WCAG21AAA
	case "EPUB Accessibility 1.1 - WCAG 2.2 Level A":
		return manifest.EPUBA11y11WCAG22A
	case "EPUB Accessibility 1.1 - WCAG 2.2 Level AA":
		return manifest.EPUBA11y11WCAG22AA
	case "EPUB Accessibility 1.1 - WCAG 2.2 Level AAA":
		return manifest.EPUBA11y11WCAG22AAA
	default:
		return ""
	}
}

func (m PubMetadataAdapter) a11yCertification(certifierItem MetadataItem) *manifest.A11yCertification {
	c := manifest.A11yCertification{
		CertifiedBy: certifierItem.value,
	}

	if certifierItem.id != "" {
		if items, ok := certifierItem.children[VocabularyA11Y+"certifierCredential"]; ok && len(items) > 0 {
			c.Credential = items[0].value
		}
		if link, ok := m.FirstLinkRefining(VocabularyA11Y+"certifierReport", certifierItem.id); ok {
			c.Report = link.href.String()
		}
	} else {
		c.Credential = m.FirstValue(VocabularyA11Y + "certifierCredential")
		c.Report = m.FirstValue(VocabularyA11Y + "certifierReport")
		if c.Report == "" {
			if link, ok := m.FirstLink(VocabularyA11Y + "certifierReport"); ok {
				c.Report = link.href.String()
			}
		}
	}

	if c.IsEmpty() {
		return nil
	}
	return &c
}

func valuesToAccessModes(values []string) []manifest.A11yAccessMode {
	am := make([]manifest.A11yAccessMode, len(values))
	for i, v := range values {
		am[i] = manifest.A11yAccessMode(v)
	}
	return am
}

func valuesToAccessModesSufficient(values []string) [][]manifest.A11yPrimaryAccessMode {
	ams := make([][]manifest.A11yPrimaryAccessMode, 0, len(values))
	for _, v := range values {
		c := a11yAccessModesSufficient(v)
		if len(c) > 0 {
			ams = append(ams, c)
		}
	}
	return ams
}

func a11yAccessModesSufficient(value string) []manifest.A11yPrimaryAccessMode {
	values := strings.Split(value, ",")
	ams := make([]manifest.A11yPrimaryAccessMode, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			ams = append(ams, manifest.A11yPrimaryAccessMode(v))
		}
	}
	return ams
}

func valuesToA11yFeatures(values []string) []manifest.A11yFeature {
	features := make([]manifest.A11yFeature, len(values))
	for i, v := range values {
		features[i] = manifest.A11yFeature(v)
	}
	return features
}

func valuesToA11yHazards(values []string) []manifest.A11yHazard {
	hazards := make([]manifest.A11yHazard, len(values))
	for i, v := range values {
		hazards[i] = manifest.A11yHazard(v)
	}
	return hazards
}

func valuesToA11yExemptions(values []string) []manifest.A11yExemption {
	exemptions := make([]manifest.A11yExemption, len(values))
	for i, v := range values {
		exemptions[i] = manifest.A11yExemption(v)
	}
	return exemptions
}

func (m *PubMetadataAdapter) seedBelongsToData() {
	if m._belongsToSeeded {
		return
	}

	type collectionHolder struct {
		typ        string
		collection manifest.Collection
	}

	var allCollections []collectionHolder
	for _, v := range m.items[VocabularyMeta+"belongs-to-collection"] {
		if typ, col, err := v.ToCollection(); err == nil {
			allCollections = append(allCollections, collectionHolder{typ: typ, collection: *col})
		}
	}

	var seriesMeta []collectionHolder
	var collectionsMeta []collectionHolder
	for _, v := range allCollections {
		if v.typ == "series" {
			seriesMeta = append(seriesMeta, v)
		} else {
			collectionsMeta = append(collectionsMeta, v)
		}
	}

	for _, v := range collectionsMeta {
		m._belongsToCollections = append(m._belongsToCollections, v.collection)
	}

	if len(seriesMeta) > 0 {
		m._belongsToSeries = make([]manifest.Collection, len(seriesMeta))
		for i, v := range seriesMeta {
			m._belongsToSeries[i] = v.collection
		}
	} else {
		cs, ok := m.items["calibre:series"]
		if ok && len(cs) > 0 {
			calibreSeries := cs[0]
			m._belongsToSeries = append(m._belongsToSeries, manifest.Collection{
				LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{
					calibreSeries.lang: calibreSeries.value,
				}),
				Position: floatOrNil(m.FirstValue("calibre:series_index")),
			})
		}
	}

	m._belongsToSeeded = true
}

func (m PubMetadataAdapter) BelongsToCollections() []manifest.Collection {
	m.seedBelongsToData()
	return m._belongsToCollections
}

func (m PubMetadataAdapter) BelongsToSeries() []manifest.Collection {
	m.seedBelongsToData()
	return m._belongsToSeries
}

func (m PubMetadataAdapter) splitSubject(subject manifest.Subject) []manifest.Subject {
	var lang string
	var names []string
	for k, v := range subject.LocalizedName.Translations {
		lang = k

		Split := func(r rune) bool {
			return r == ',' || r == ';'
		}
		for _, n := range strings.FieldsFunc(v, Split) {
			n = strings.TrimSpace(n)
			if n != "" {
				names = append(names, n)
			}
		}

		break
	}

	subjects := make([]manifest.Subject, len(names))
	for i, n := range names {
		subjects[i] = manifest.Subject{
			LocalizedName: manifest.NewLocalizedStringFromStrings(map[string]string{lang: n}),
		}
	}
	return subjects
}

func (m *PubMetadataAdapter) Subjects() []manifest.Subject {
	if len(m._subjects) > 0 {
		return m._subjects
	}
	subjectItems, ok := m.items[VocabularyDCTerms+"subject"]
	if !ok || len(subjectItems) == 0 {
		return nil
	}

	var parsedSubjects []manifest.Subject
	for _, v := range subjectItems {
		if v, err := v.ToSubject(); err == nil {
			parsedSubjects = append(parsedSubjects, *v)
		}
	}

	hasToSplit := false
	if len(parsedSubjects) == 1 {
		ln := parsedSubjects[0]
		if ln.LocalizedName.Length() == 1 && ln.Code == "" && ln.Scheme == "" && ln.SortAs() == "" {
			hasToSplit = true
		}
	}

	if hasToSplit {
		m._subjects = m.splitSubject(parsedSubjects[0])
	} else {
		m._subjects = parsedSubjects
	}

	return m._subjects
}

func (m *PubMetadataAdapter) seedContributors() {
	contributors := append(append(append(
		m.items[VocabularyDCTerms+"creator"],
		m.items[VocabularyDCTerms+"contributor"]...,
	), m.items[VocabularyDCTerms+"publisher"]...),
		m.items[VocabularyMedia+"narrator"]...)

	m._allContributors = make(map[string][]manifest.Contributor)
	for _, contributor := range contributors {
		typ, c, err := contributor.ToContributor()
		if err != nil {
			continue
		}
		m._allContributors[typ] = append(m._allContributors[typ], *c)
	}
}

func (m *PubMetadataAdapter) Contributors(role string) []manifest.Contributor {
	if m._allContributors == nil {
		m.seedContributors()
	}
	return m._allContributors[role]
}

func (m *PubMetadataAdapter) ReadingProgression() manifest.ReadingProgression {
	return m.readingProgression
}

func (m *PubMetadataAdapter) Layout() manifest.Layout {
	if m._layout == manifest.LayoutNone {
		var layoutProp string
		if m.epubVersion < 3.0 {
			if do, ok := m.displayOptions["fixed-layout"]; ok && do == "true" {
				layoutProp = "pre-paginated"
			} else {
				layoutProp = "reflowable"
			}
		} else {
			layoutProp = m.FirstValue(VocabularyRendition + "layout")
		}

		m._layout = manifest.LayoutReflowable
		if layoutProp == "pre-paginated" {
			m._layout = manifest.LayoutFixed
		} else if layoutProp == "scrolled" {
			m._layout = manifest.LayoutScrolled
		}
	}
	return m._layout
}

func (m *PubMetadataAdapter) MediaOverlay() *manifest.MediaOverlay {
	activeClass := m.FirstValue(VocabularyMedia + "active-class")
	playbackActiveClass := m.FirstValue(VocabularyMedia + "playback-active-class")

	if activeClass == "" && playbackActiveClass == "" {
		return nil
	}
	return &manifest.MediaOverlay{
		ActiveClass:         activeClass,
		PlaybackActiveClass: playbackActiveClass,
	}
}

func (m *PubMetadataAdapter) OtherMetadata() map[string]interface{} {
	if m._otherMetadata == nil {
		usedProperties := map[string]struct{}{
			VocabularyMeta + "cover": {}, // EPUB 2 cover meta

			VocabularyDCTerms + "identifier":          {},
			VocabularyDCTerms + "language":            {},
			VocabularyDCTerms + "title":               {},
			VocabularyDCTerms + "date":                {},
			VocabularyDCTerms + "modified":            {},
			VocabularyDCTerms + "description":         {},
			VocabularyDCTerms + "duration":            {},
			VocabularyDCTerms + "creator":             {},
			VocabularyDCTerms + "publisher":           {},
			VocabularyDCTerms + "contributor":         {},
			VocabularyMedia + "narrator":              {},
			VocabularyMedia + "duration":              {},
			VocabularyMedia + "active-class":          {},
			VocabularyMedia + "playback-active-class": {},
			VocabularyRendition + "layout":            {},

			VocabularyDCTerms + "conformsto":          {},
			VocabularyDCTerms + "conformsTo":          {},
			VocabularyA11Y + "certifiedBy":            {},
			VocabularyA11Y + "certifierCredential":    {},
			VocabularyA11Y + "certifierReport":        {},
			VocabularySchema + "accessibilitySummary": {},
			VocabularySchema + "accessMode":           {},
			VocabularySchema + "accessModeSufficient": {},
			VocabularySchema + "accessibilityFeature": {},
			VocabularySchema + "accessibilityHazard":  {},
		}

		m._otherMetadata = make(map[string]interface{})
		for k, v := range m.items {
			if _, ok := usedProperties[k]; ok {
				continue
			}
			values := make([]interface{}, len(v))
			for i, val := range v {
				values[i] = val.ToMap()
			}
			if len(values) == 1 {
				m._otherMetadata[k] = values[0]
			} else {
				m._otherMetadata[k] = values
			}
		}
	}
	return m._otherMetadata
}

// MetadataItem
type MetadataItem struct {
	property      string
	value         string
	lang          string
	scheme        string
	refines       string
	id            string
	children      map[string][]MetadataItem
	otherMetadata map[string]interface{}
}

func (m MetadataItem) ToSubject() (*manifest.Subject, error) {
	if m.property != VocabularyDCTerms+"subject" {
		return nil, errors.New("wrong property for subject")
	}

	fileAsK, fileAsV := m.FileAs()
	var localizedSortAs *manifest.LocalizedString
	if fileAsK != "" && fileAsV != "" {
		localizedSortAs = &manifest.LocalizedString{}
		localizedSortAs.SetTranslation(fileAsK, fileAsV)
	}

	return &manifest.Subject{
		LocalizedName:   m.LocalizedString(),
		LocalizedSortAs: localizedSortAs,
		Scheme:          m.Authority(),
		Code:            m.Term(),
	}, nil
}

func (m MetadataItem) ToTitle() (*Title, error) {
	if m.property != VocabularyDCTerms+"title" {
		return nil, errors.New("wrong property for title")
	}

	fileAsK, fileAsV := m.FileAs()
	var localizedSortAs *manifest.LocalizedString
	if fileAsK != "" && fileAsV != "" {
		localizedSortAs = &manifest.LocalizedString{}
		localizedSortAs.SetTranslation(fileAsK, fileAsV)
	}

	return &Title{
		value:      m.LocalizedString(),
		fileAs:     localizedSortAs,
		typ:        m.TitleType(),
		displaySeq: m.DisplaySeq(),
	}, nil
}

var contributorProperties = map[string]struct{}{
	VocabularyDCTerms + "creator":            {},
	VocabularyDCTerms + "contributor":        {},
	VocabularyDCTerms + "publisher":          {},
	VocabularyMedia + "narrator":             {},
	VocabularyMeta + "belongs-to-collection": {},
}
var knownRoles = map[string]struct{}{"aut": {}, "trl": {}, "edt": {}, "pbl": {}, "art": {}, "ill": {}, "clr": {}, "nrt": {}}

func (m MetadataItem) ToContributor() (string, *manifest.Contributor, error) {
	if _, ok := contributorProperties[m.property]; !ok {
		return "", nil, errors.New("wrong property for contributor")
	}

	names := m.LocalizedString()

	fileAsK, fileAsV := m.FileAs()
	var localizedSortAs *manifest.LocalizedString
	if fileAsV != "" {
		localizedSortAs = &manifest.LocalizedString{}
		localizedSortAs.SetTranslation(fileAsK, fileAsV)
	}

	role := m.Role()
	var roles []string
	if role != "" {
		if _, ok := knownRoles[role]; !ok {
			roles = append(roles, role)
		}
	}

	typ := ""
	switch m.property {
	case VocabularyMeta + "belongs-to-collection":
		typ = m.CollectionType()
	case VocabularyDCTerms + "creator":
		typ = "aut"
	case VocabularyDCTerms + "publisher":
		typ = "pbl"
	case VocabularyMedia + "narrator":
		typ = "nrt"
	default:
		if _, ok := knownRoles[role]; ok {
			typ = role
		}
	}

	allIdentifiers := m.Identifiers()
	var identifier string
	var altIdentifiers []manifest.AltIdentifier
	if len(allIdentifiers) > 0 {
		identifier = allIdentifiers[0]
		if len(allIdentifiers) > 1 {
			altIdentifiers = make([]manifest.AltIdentifier, len(allIdentifiers)-1)
			for i, id := range allIdentifiers[1:] {
				altIdentifiers[i] = manifest.AltIdentifier{
					Value:  id,
					Scheme: m.scheme,
				}
			}
		}
	}

	return typ, &manifest.Contributor{
		LocalizedName:   names,
		LocalizedSortAs: localizedSortAs,
		Roles:           roles,
		Identifier:      identifier,
		AltIdentifier:   altIdentifiers,
		Position:        m.GroupPosition(),
	}, nil
}

func (m MetadataItem) ToCollection() (string, *manifest.Contributor, error) {
	return m.ToContributor()
}

func (m MetadataItem) ToMap() interface{} {
	if len(m.children) == 0 {
		return m.value
	} else {
		cm := make(map[string]interface{})
		for _, child := range m.children {
			for _, item := range child {
				cm[item.property] = item.ToMap()
			}
		}
		cm["@value"] = m.value
		return cm
	}
}

func (m MetadataItem) FileAs() (string, string) {
	child, ok := m.children[VocabularyMeta+"file-as"]
	if !ok {
		return "", ""
	}
	if len(child) == 0 {
		return "", ""
	}
	return child[0].lang, child[0].value
}

func (m MetadataItem) TitleType() string {
	return m.FirstValue(VocabularyMeta + "title-type")
}

func (m MetadataItem) DisplaySeq() *int {
	return intOrNil(m.FirstValue(VocabularyMeta + "display-seq"))
}

func (m MetadataItem) Authority() string {
	return m.FirstValue(VocabularyMeta + "authority")
}

func (m MetadataItem) Term() string {
	return m.FirstValue(VocabularyMeta + "term")
}

func (m MetadataItem) AlternateScript() map[string]string {
	child, ok := m.children[VocabularyMeta+"alternate-script"]
	if !ok {
		return nil
	}

	fm := make(map[string]string)
	for _, c := range child {
		fm[c.lang] = c.value
	}
	return fm
}

func (m MetadataItem) CollectionType() string {
	return m.FirstValue(VocabularyMeta + "collection-type")
}

func (m MetadataItem) GroupPosition() *float64 {
	return floatOrNil(m.FirstValue(VocabularyMeta + "group-position"))
}

func (m MetadataItem) Identifiers() []string {
	identifiers, ok := m.children[VocabularyDCTerms+"identifier"]
	if !ok || len(identifiers) == 0 {
		return nil
	}

	return itemsValues(identifiers)
}

func (m MetadataItem) Role() string {
	return m.FirstValue(VocabularyMeta + "role")
}

func (m MetadataItem) LocalizedString() manifest.LocalizedString {
	values := make(map[string]string)
	values[m.lang] = m.value
	if as := m.AlternateScript(); as != nil {
		for k, v := range as {
			values[k] = v
		}
	}
	return manifest.NewLocalizedStringFromStrings(values)
}

func (m MetadataItem) FirstValue(property string) string {
	child, ok := m.children[property]
	if !ok || len(child) == 0 {
		return ""
	}
	return child[0].value
}
