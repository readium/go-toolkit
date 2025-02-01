package manifest

// Transforms a manifest's components.
type ManifestTransformer interface {
	TransformManifest(manifest Manifest) Manifest
	TransformMetadata(metadata Metadata) Metadata
	TransformLink(link Link) Link
	TransformHREF(href HREF) HREF
}

// Creates a copy of the receiver [Manifest], applying the given [transformer] to each component.
func (m Manifest) Copy(transformer ManifestTransformer) Manifest {
	m.Metadata = m.Metadata.Copy(transformer)
	m.Links = m.Links.Copy(transformer)
	m.ReadingOrder = m.ReadingOrder.Copy(transformer)
	m.Resources = m.Resources.Copy(transformer)
	m.TableOfContents = m.TableOfContents.Copy(transformer)
	m.Subcollections = m.Subcollections.Copy(transformer)
	return transformer.TransformManifest(m)
}

func (m Metadata) Copy(transformer ManifestTransformer) Metadata {
	for i, subject := range m.Subjects {
		m.Subjects[i] = subject.Copy(transformer)
	}
	m.Authors = m.Authors.Copy(transformer)
	m.Translators = m.Translators.Copy(transformer)
	m.Editors = m.Editors.Copy(transformer)
	m.Artists = m.Artists.Copy(transformer)
	m.Illustrators = m.Illustrators.Copy(transformer)
	m.Letterers = m.Letterers.Copy(transformer)
	m.Pencilers = m.Pencilers.Copy(transformer)
	m.Colorists = m.Colorists.Copy(transformer)
	m.Inkers = m.Inkers.Copy(transformer)
	m.Narrators = m.Narrators.Copy(transformer)
	m.Contributors = m.Contributors.Copy(transformer)
	m.Publishers = m.Publishers.Copy(transformer)
	m.Imprints = m.Imprints.Copy(transformer)
	for k, v := range m.BelongsTo {
		m.BelongsTo[k] = v.Copy(transformer)
	}
	return transformer.TransformMetadata(m)
}

func (p PublicationCollection) Copy(transformer ManifestTransformer) PublicationCollection {
	p.Links = p.Links.Copy(transformer)
	p.Subcollections = p.Subcollections.Copy(transformer)
	return p
}

func (p PublicationCollectionMap) Copy(transformer ManifestTransformer) PublicationCollectionMap {
	for k, v := range p {
		for i, c := range v {
			p[k][i] = c.Copy(transformer)
		}
	}
	return p
}

func (c Contributors) Copy(transformer ManifestTransformer) Contributors {
	for i, contributor := range c {
		c[i] = contributor.Copy(transformer)
	}
	return c
}

func (c Contributor) Copy(transformer ManifestTransformer) Contributor {
	c.Links = c.Links.Copy(transformer)
	return c
}

func (s Subject) Copy(transformer ManifestTransformer) Subject {
	s.Links = s.Links.Copy(transformer)
	return s
}

func (ll LinkList) Copy(transformer ManifestTransformer) LinkList {
	for i, link := range ll {
		ll[i] = link.Copy(transformer)
	}
	return ll
}

func (l Link) Copy(transformer ManifestTransformer) Link {
	l.Href = l.Href.Copy(transformer)
	l.Alternates = l.Alternates.Copy(transformer)
	l.Children = l.Children.Copy(transformer)
	return transformer.TransformLink(l)
}

func (h HREF) Copy(transformer ManifestTransformer) HREF {
	return transformer.TransformHREF(h)
}
