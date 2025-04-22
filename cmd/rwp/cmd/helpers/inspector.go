package helpers

import (
	"io/fs"

	"github.com/pkg/errors"
	"github.com/readium/go-toolkit/pkg/analyzer"
	"github.com/readium/go-toolkit/pkg/manifest"
)

type ImageInspector struct {
	Filesystem fs.FS
	Algorithms []manifest.HashAlgorithm
	err        error
}

func (n *ImageInspector) Error() error {
	return n.err
}

// TransformHREF implements ManifestTransformer
func (n *ImageInspector) TransformHREF(href manifest.HREF) manifest.HREF {
	// Identity
	return href
}

// TransformLink implements ManifestTransformer
func (n *ImageInspector) TransformLink(link manifest.Link) manifest.Link {
	if n.err != nil || link.MediaType == nil || !link.MediaType.IsBitmap() {
		return link
	}

	newLink, err := analyzer.Image(n.Filesystem, link, n.Algorithms)
	if err != nil {
		n.err = errors.Wrap(err, "failed inspecting image "+link.Href.String())
		return link
	}
	return *newLink
}

// TransformManifest implements ManifestTransformer
func (n *ImageInspector) TransformManifest(manifest manifest.Manifest) manifest.Manifest {
	// Identity
	return manifest
}

// TransformMetadata implements ManifestTransformer
func (n *ImageInspector) TransformMetadata(metadata manifest.Metadata) manifest.Metadata {
	// Identity
	return metadata
}
