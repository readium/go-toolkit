package epub

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/antchfx/xmlquery"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/util"
)

type Title struct {
	value      manifest.LocalizedString
	fileAs     *manifest.LocalizedString
	typ        string
	displaySeq *int
}

type EPUBLink struct {
	href       string
	rels       []string // set
	mediaType  string
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

func (m MetadataParser) Parse(document *xmlquery.Node, filePath string) *EPUBMetadata {
	// Init lang
	if l := document.SelectElement("/package[namespace-uri()='" + NamespaceOPF + "']"); l != nil {
		for _, attr := range l.Attr {
			if attr.Name.Local == "lang" {
				m.packageLanguage = attr.Value
			}
		}
	}
	// println(document.OutputXML(true))
	if l := document.SelectElement("//metadata/language[namespace-uri()='" + NamespaceDC + "']"); l != nil {
		m.metaLanguage = l.Data
	}

	// println("DOC", document.SelectElement("//metadata"))

	metadata := document.SelectElement("//metadata[namespace-uri()='" + NamespaceOPF + "']")
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
	lang := element.SelectAttr("lang")
	if lang != "" {
		return lang
	}
	if m.packageLanguage != "" {
		return m.packageLanguage
	}
	return m.metaLanguage
}

func (m MetadataParser) parseElements(metadataElement *xmlquery.Node, filePath string) ([]MetadataItem, []EPUBLink) {
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

func (m MetadataParser) parseLinkElement(element *xmlquery.Node, filePath string) *EPUBLink {
	if element == nil {
		return nil
	}
	href := element.SelectAttr("href")
	if href == "" {
		return nil
	}

	hr, err := util.NewHREF(href, filePath).String()
	if err != nil {
		return nil
	}

	link := &EPUBLink{
		href:      hr,
		mediaType: element.SelectAttr("media-type"),
		refines:   strings.TrimPrefix(element.SelectAttr("refines"), "#"),
	}

	relAttr := element.SelectAttr("rel")
	for _, v := range parseProperties(relAttr) {
		link.rels = append(link.rels, resolveProperty(v, m.prefixMap, VocabLink))
	}

	propAttrs := element.SelectAttr("properties")
	for _, v := range parseProperties(propAttrs) {
		link.properties = append(link.properties, resolveProperty(v, m.prefixMap, VocabLink))
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
		resolvedName := resolveProperty(name, m.prefixMap, NoVocab)
		return &MetadataItem{
			property: resolvedName,
			value:    content,
			lang:     m.language(element),
			id:       element.SelectAttr("id"),
		}
	} else {
		propName := strings.TrimSpace(element.SelectAttr("property"))
		if propName == "" {
			return nil
		}
		propValue := strings.TrimSpace(element.InnerText())
		if propValue == "" {
			return nil
		}
		resolvedScheme := strings.TrimSpace(element.SelectAttr("scheme"))
		if resolvedScheme != "" {
			resolvedScheme = resolveProperty(resolvedScheme, m.prefixMap, NoVocab)
		}
		return &MetadataItem{
			property: resolveProperty(propName, m.prefixMap, VocabMeta),
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

	propName := VocabularyDCTerms + element.Data
	switch element.Data {
	case "creator":
		fallthrough
	case "contributor":
		fallthrough
	case "publisher":
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

	fileAs := element.SelectAttr(NamespaceOPF + ":file-as")
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

	role := element.SelectAttr(NamespaceOPF + ":role")
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
	eventAttr := element.SelectAttr(NamespaceOPF + ":event")
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
		presentation:  expr.presentation,
		otherMetadata: expr.otherMetadata,
	}
}

type metadataAdapter struct {
	epubVersion float64
	items       map[string][]MetadataItem
}

func (m metadataAdapter) Duration() *float64 {
	return ParseClockValue(m.FirstValue(VocabularyMedia + "duration"))
}

func (m metadataAdapter) FirstValue(property string) string {
	p, ok := m.items[property]
	if !ok {
		return ""
	}
	if len(p) == 0 {
		return ""
	}
	return p[0].value
}

type LinkMetadataAdapter = metadataAdapter

type PubMetadataAdapter struct {
	metadataAdapter
	fallbackTitle      string
	uniqueIdentifierID string
	readingProgression manifest.ReadingProgression
	displayOptions     map[string]string
	_identifier        string

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
	_presentation    *manifest.Presentation
	_otherMetadata   map[string]interface{}
}

func (m PubMetadataAdapter) Metadata() manifest.Metadata {
	metadata := manifest.Metadata{
		Identifier:         m.Identifier(),
		ConformsTo:         manifest.Profiles{manifest.ProfileEPUB},
		Modified:           m.Modified(),
		Published:          m.Published(),
		Languages:          m.Languages(),
		LocalizedTitle:     m.LocalizedTitle(),
		LocalizedSortAs:    m.LocalizedSortAs(),
		LocalizedSubtitle:  m.LocalizedSubtitle(),
		Duration:           m.Duration(),
		Subjects:           m.Subjects(),
		Description:        m.Description(),
		ReadingProgression: m.ReadingProgression(),
		BelongsTo:          make(map[string][]manifest.Contributor),
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

func (m *PubMetadataAdapter) Identifier() string {
	if m._identifier != "" {
		return m._identifier
	}
	if m.uniqueIdentifierID == "" {
		return ""
	}
	identifiers, ok := m.items[VocabularyDCTerms+"identifier"]
	if !ok || len(identifiers) == 0 {
		return ""
	}
	for _, v := range identifiers {
		if v.property == m.uniqueIdentifierID {
			m._identifier = v.value
			return m._identifier
		}
	}
	m._identifier = identifiers[0].value
	return m._identifier
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
	return m.FirstValue("cover")
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

func (m *PubMetadataAdapter) seedBelongsToData() {
	if m._belongsToSeeded {
		return
	}

	type collectionHolder struct {
		typ        string
		collection manifest.Collection
	}

	var allCollections []collectionHolder
	for _, v := range m.items[VocabularyDCTerms+"belongs-to-collection"] {
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

func (m *PubMetadataAdapter) Presentation() manifest.Presentation {
	if m._presentation == nil {
		m._presentation = &manifest.Presentation{}

		flowProp := m.FirstValue(VocabularyRendition + "flow")
		spreadProp := m.FirstValue(VocabularyRendition + "spread")
		orientationProp := m.FirstValue(VocabularyRendition + "orientation")
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

		overflow := manifest.OverflowAuto
		continuous := manifest.PresentationDefaultContinuous
		switch flowProp {
		case "paginated":
			overflow = manifest.OverflowPaginated
		case "scrolled-continuous":
			overflow = manifest.OverflowScrolled
			continuous = true
		case "scrolled-doc":
			overflow = manifest.OverflowScrolled
		}
		m._presentation.Overflow = &overflow
		m._presentation.Continuous = &continuous

		layout := manifest.EPUBLayoutReflowable
		if layoutProp == "pre-paginated" {
			layout = manifest.EPUBLayoutFixed
		}
		m._presentation.Layout = &layout

		orientation := manifest.OrientationAuto
		switch orientationProp {
		case "landscape":
			orientation = manifest.OrientationLandscape
		case "portrait":
			orientation = manifest.OrientationPortrait
		}
		m._presentation.Orientation = &orientation

		spread := manifest.SpreadAuto
		switch spreadProp {
		case "none":
			spread = manifest.SpreadNone
		case "landscape":
			spread = manifest.SpreadLandscape
		case "portrait":
			fallthrough
		case "both":
			spread = manifest.SpreadBoth
		}
		m._presentation.Spread = &spread

	}
	return *m._presentation
}

func (m *PubMetadataAdapter) OtherMetadata() map[string]interface{} {
	if m._otherMetadata == nil {
		usedProperties := map[string]struct{}{
			VocabularyDCTerms + "identifier":    {},
			VocabularyDCTerms + "language":      {},
			VocabularyDCTerms + "title":         {},
			VocabularyDCTerms + "date":          {},
			VocabularyDCTerms + "modified":      {},
			VocabularyDCTerms + "description":   {},
			VocabularyDCTerms + "duration":      {},
			VocabularyDCTerms + "creator":       {},
			VocabularyDCTerms + "publisher":     {},
			VocabularyDCTerms + "contributor":   {},
			VocabularyMedia + "narrator":        {},
			VocabularyMedia + "duration":        {},
			VocabularyRendition + "flow":        {},
			VocabularyRendition + "spread":      {},
			VocabularyRendition + "orientation": {},
			VocabularyRendition + "layout":      {},
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
		m._otherMetadata["presentation"] = m.Presentation()
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
	presentation  manifest.Presentation
	otherMetadata map[string]interface{}
}

func (m MetadataItem) ToSubject() (*manifest.Subject, error) {
	if m.property != VocabularyDCTerms+"subject" {
		return nil, errors.New("wrong property for subject")
	}

	fileAsK, fileAsV := m.FileAs()
	fileAs := &manifest.LocalizedString{}
	fileAs.SetTranslation(fileAsK, fileAsV)

	return &manifest.Subject{
		LocalizedName:   m.LocalizedString(),
		LocalizedSortAs: fileAs,
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
	roles := []string{}
	if _, ok := knownRoles[role]; ok {
		roles = append(roles, role)
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

	return typ, &manifest.Contributor{
		LocalizedName:   names,
		LocalizedSortAs: localizedSortAs,
		Roles:           roles,
		Identifier:      m.Identifier(),
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

func (m MetadataItem) Identifier() string {
	return m.FirstValue(VocabularyDCTerms + "identifier")
}

func (m MetadataItem) Role() string {
	return m.FirstValue(VocabularyMeta + "role")
}

func (m MetadataItem) LocalizedString() manifest.LocalizedString {
	values := make(map[string]string)
	if m.lang != "" {
		values[m.lang] = m.value
	}
	if as := m.AlternateScript(); as != nil {
		for k, v := range as {
			values[k] = v
		}
	}
	return manifest.NewLocalizedStringFromStrings(values)
}

func (m MetadataItem) FirstValue(property string) string {
	child, ok := m.children[property]
	if !ok {
		return ""
	}
	if len(child) == 0 {
		return ""
	}
	return child[0].value
}
