package manifest

// Applies [transform] to the HREF of every link of the manifest: the reading order,
// resources, links and table of contents, including their alternates, children, and
// the links of subcollections.
func (m *Manifest) TransformHREFs(transform func(HREF) HREF) {
	m.Links.TransformHREFs(transform)
	m.ReadingOrder.TransformHREFs(transform)
	m.Resources.TransformHREFs(transform)
	m.TableOfContents.TransformHREFs(transform)
	m.Subcollections.TransformHREFs(transform)
}

// Applies [transform] to the HREF of every link of the list, including their
// alternates and children.
func (ll LinkList) TransformHREFs(transform func(HREF) HREF) {
	for i := range ll {
		ll[i].TransformHREFs(transform)
	}
}

// Applies [transform] to the link's HREF, and to those of its alternates and children.
func (l *Link) TransformHREFs(transform func(HREF) HREF) {
	l.Href = transform(l.Href)
	l.Alternates.TransformHREFs(transform)
	l.Children.TransformHREFs(transform)
}

// Applies [transform] to the HREF of every link of every collection of the map,
// including subcollections.
func (pcm PublicationCollectionMap) TransformHREFs(transform func(HREF) HREF) {
	for _, collections := range pcm {
		for i := range collections {
			collections[i].Links.TransformHREFs(transform)
			collections[i].Subcollections.TransformHREFs(transform)
		}
	}
}
