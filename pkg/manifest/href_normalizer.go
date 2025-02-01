package manifest

import "github.com/readium/go-toolkit/pkg/util/url"

// Returns a copy of the receiver after normalizing its HREFs to the link with `rel="self"`.
func (m Manifest) NormalizeHREFsToSelf() Manifest {
	self := m.LinkWithRel("self")
	if self == nil {
		return m
	}

	return m.NormalizeHREFsToBase(self.URL(nil, nil))
}

// Returns a copy of the receiver after normalizing its HREFs to the given [baseUrl].
func (m Manifest) NormalizeHREFsToBase(baseURL url.URL) Manifest {
	if baseURL == nil {
		return m
	}

	return m.Copy(NewHREFNormalizer(baseURL))
}

// Returns a copy of the receiver after normalizing its HREFs to the given [baseUrl].
func (l Link) NormalizeHREFsToBase(baseURL url.URL) Link {
	if baseURL == nil {
		return l
	}

	return l.Copy(NewHREFNormalizer(baseURL))
}

type HREFNormalizer struct {
	baseURL url.URL
}

func NewHREFNormalizer(baseURL url.URL) HREFNormalizer {
	return HREFNormalizer{baseURL: baseURL}
}

// TransformHREF implements ManifestTransformer
func (n HREFNormalizer) TransformHREF(href HREF) HREF {
	return href.ResolveTo(n.baseURL)
}

// TransformLink implements ManifestTransformer
func (n HREFNormalizer) TransformLink(link Link) Link {
	// Identity
	return link
}

// TransformManifest implements ManifestTransformer
func (n HREFNormalizer) TransformManifest(manifest Manifest) Manifest {
	// Identity
	return manifest
}

// TransformMetadata implements ManifestTransformer
func (n HREFNormalizer) TransformMetadata(metadata Metadata) Metadata {
	// Identity
	return metadata
}
