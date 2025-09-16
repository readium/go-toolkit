package guidednavigation

// Readium Guided Navigation Roles
// https://github.com/readium/guided-navigation/blob/main/schema/roles.schema.json
type GuidedNavigationRole string

const (
	RoleNone            GuidedNavigationRole = ""                // No role. Shouldn't be used
	RoleAbstract        GuidedNavigationRole = "abstract"        // A short summary of the principle ideas, concepts and conclusions of the work, or of a section or excerpt within it.
	RoleAcknowledgments GuidedNavigationRole = "acknowledgments" // A section or statement that acknowledges significant contributions by persons, organizations, governments and other entities to the realization of the work.
	RoleAfterword       GuidedNavigationRole = "afterword"       // A closing statement from the author or a person of importance, typically providing insight into how the content came to be written, its significance, or related events that have transpired since its timeline.
	RoleAppendix        GuidedNavigationRole = "appendix"        // A section of supplemental information located after the primary content that informs the content but is not central to it.
	RoleArticle         GuidedNavigationRole = "article"         // Represents a self-contained composition in a document, page, application, or site, which is intended to be independently distributable or reusable
	RoleAside           GuidedNavigationRole = "aside"           // Secondary or supplementary content.
	RoleAudio           GuidedNavigationRole = "audio"           // Embedded sound content in a document.
	RoleBacklink        GuidedNavigationRole = "backlink"        // A link that allows the user to return to a related location in the content (e.g., from a footnote to its reference or from a glossary definition to where a term is used).
	RoleBibliography    GuidedNavigationRole = "bibliography"    // A list of external references cited in the work, which may be to print or digital sources.
	RoleBiblioref       GuidedNavigationRole = "biblioref"       // A reference to a bibliography entry.
	RoleBlockquote      GuidedNavigationRole = "blockquote"      // Represents a section that is quoted from another source.
	RoleCaption         GuidedNavigationRole = "caption"         // A caption for an image or a table.
	RoleChapter         GuidedNavigationRole = "chapter"         // A major thematic section of content in a work.
	RoleCell            GuidedNavigationRole = "cell"            // A single cell of tabular data or content.
	RoleColumnHeader    GuidedNavigationRole = "columnheader"    // The header cell for a column, establishing a relationship between it and the other cells in the same column.
	RoleColophon        GuidedNavigationRole = "colophon"        // A short section of production notes particular to the edition (e.g., describing the typeface used), often located at the end of a work.
	RoleComplementary   GuidedNavigationRole = "complementary"   // A supporting section of the document, designed to be complementary to the main content at a similar level in the DOM hierarchy, but remains meaningful when separated from the main content.
	RoleConclusion      GuidedNavigationRole = "conclusion"      // A concluding section or statement that summarizes the work or wraps up the narrative.
	RoleCover           GuidedNavigationRole = "cover"           // An image that sets the mood or tone for the work and typically includes the title and author.
	RoleCredit          GuidedNavigationRole = "credit"          // An acknowledgment of the source of integrated content from third-party sources, such as photos. Typically identifies the creator, copyright and any restrictions on reuse.
	RoleCredits         GuidedNavigationRole = "credits"         // A collection of credits.
	RoleDedication      GuidedNavigationRole = "dedication"      // An inscription at the front of the work, typically addressed in tribute to one or more persons close to the author.
	RoleDefinition      GuidedNavigationRole = "definition"      // A definition of a term or concept.
	RoleDetails         GuidedNavigationRole = "details"         // A disclosure widget that can be expanded.
	RoleEndnotes        GuidedNavigationRole = "endnotes"        // A collection of notes at the end of a work or a section within it.
	RoleEpigraph        GuidedNavigationRole = "epigraph"        // A quotation set at the start of the work or a section that establishes the theme or sets the mood.
	RoleEpilogue        GuidedNavigationRole = "epilogue"        // A quotation set at the start of the work or a section that establishes the theme or sets the mood.
	RoleErrata          GuidedNavigationRole = "errata"          // A set of corrections discovered after initial publication of the work, sometimes referred to as corrigenda.
	RoleExample         GuidedNavigationRole = "example"         // An illustration of the usage of a defined term or phrase.
	RoleFigure          GuidedNavigationRole = "figure"          // An illustration, diagram, photo, code listing or similar, referenced from the text of a work, and typically annotated with a title, caption and/or credits.
	RoleFootnote        GuidedNavigationRole = "footnote"        // Ancillary information, such as a citation or commentary, that provides additional context to a referenced passage of text.
	RoleGlossary        GuidedNavigationRole = "glossary"        // A brief dictionary of new, uncommon, or specialized terms used in the content.
	RoleGlossref        GuidedNavigationRole = "glossref"        // A reference to a glossary definition.
	RoleHeader          GuidedNavigationRole = "header"          // Represents introductory content, typically a group of introductory or navigational aids.
	RoleHeading         GuidedNavigationRole = "heading"         // A heading for a section of the page.
	RoleImage           GuidedNavigationRole = "image"           // Represents an image.
	RoleIndex           GuidedNavigationRole = "index"           // A navigational aid that provides a detailed list of links to key subjects, names and other important topics covered in the work.
	RoleIntroduction    GuidedNavigationRole = "introduction"    // A preliminary section that typically introduces the scope or nature of the work.
	RoleLandmarks       GuidedNavigationRole = "landmarks"       // A short summary of the principle ideas, concepts and conclusions of the work, or of a section or excerpt within it.
	RoleList            GuidedNavigationRole = "list"            // A structure that contains an enumeration of related content items.
	RoleListItem        GuidedNavigationRole = "listItem"        // A single item in an enumeration.
	RoleLoa             GuidedNavigationRole = "loa"             // A listing of audio clips included in the work.
	RoleLoi             GuidedNavigationRole = "loi"             // A listing of illustrations included in the work.
	RoleLot             GuidedNavigationRole = "lot"             // A listing of tables included in the work.
	RoleLov             GuidedNavigationRole = "lov"             // A listing of video clips included in the work.
	RoleMain            GuidedNavigationRole = "main"            // Content that is directly related to or expands upon the central topic of the document.
	RoleMath            GuidedNavigationRole = "math"            // Content that represents a mathematical expression.
	RoleNavigation      GuidedNavigationRole = "navigation"      // Represents a section of a page that links to other pages or to parts within the page: a section with navigation links.
	RoleNoteref         GuidedNavigationRole = "noteref"         // A reference to a footnote or endnote, typically appearing as a superscripted number or symbol in the main body of text.
	RoleNotice          GuidedNavigationRole = "notice"          // Notifies the user of consequences that might arise from an action or event. Examples include warnings, cautions and dangers.
	RolePagebreak       GuidedNavigationRole = "pagebreak"       // A separator denoting the position before which a break occurs between two contiguous pages in a statically paginated version of the content.
	RolePagelist        GuidedNavigationRole = "pagelist"        // A navigational aid that provides a list of links to the pagebreaks in the content.
	RoleParagraph       GuidedNavigationRole = "paragraph"       // Represents a paragraph.
	RolePart            GuidedNavigationRole = "part"            // A major structural division in a work that contains a set of related sections dealing with a particular subject, narrative arc or similar encapsulated theme.
	RolePreface         GuidedNavigationRole = "preface"         // An introductory section that precedes the work, typically written by the author of the work.
	RolePreformatted    GuidedNavigationRole = "preformatted"    // Represents preformatted text which is to be presented exactly as written.
	RolePresentation    GuidedNavigationRole = "presentation"    // Represents an element being used only for presentation and therefore that does not have any accessibility semantics.
	RolePrologue        GuidedNavigationRole = "prologue"        // An introductory section that sets the background to a work, typically part of the narrative.
	RolePullquote       GuidedNavigationRole = "pullquote"       // A distinctively placed or highlighted quotation from the current content designed to draw attention to a topic or highlight a key point.
	RoleQna             GuidedNavigationRole = "qna"             // A section of content structured as a series of questions and answers, such as an interview or list of frequently asked questions.
	RoleRegion          GuidedNavigationRole = "region"          // Represents content that is relevant to a specific, author-specified purpose and sufficiently important that users will likely want to be able to navigate to the section easily and to have it listed in a summary of the page.
	RoleRow             GuidedNavigationRole = "row"             // A row of data or content in a tabular structure.
	RoleRowHeader       GuidedNavigationRole = "rowheader"       // The header cell for a row, establishing a relationship between it and the other cells in the same row.
	RoleSection         GuidedNavigationRole = "section"         // Represents a generic standalone section of a document, which doesn't have a more specific semantic element to represent it.
	RoleSeparator       GuidedNavigationRole = "separator"       // Indicates the element is a divider that separates and distinguishes sections of content or groups of menuitems.
	RoleSubtitle        GuidedNavigationRole = "subtitle"        // An explanatory or alternate title for the work, or a section or component within it.
	RoleSummary         GuidedNavigationRole = "summary"         // A summary of an element contained in details.
	RoleTable           GuidedNavigationRole = "table"           // A structure containing data or content laid out in tabular form.
	RoleTerm            GuidedNavigationRole = "term"            // A word or phrase with a corresponding definition.
	RoleTip             GuidedNavigationRole = "tip"             // Helpful information that clarifies some aspect of the content or assists in its comprehension.
	RoleToc             GuidedNavigationRole = "toc"             // A navigational aid that provides an ordered list of links to the major sectional headings in the content. A table of contents may cover an entire work, or only a smaller section of it.
	RoleVideo           GuidedNavigationRole = "video"           // Embedded videos, movies, or audio files with captions in a document.
)
