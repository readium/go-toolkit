package streamer

import (
	"context"
	"slices"

	"github.com/readium/go-toolkit/pkg/analyzer"
	"github.com/readium/go-toolkit/pkg/fetcher"
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
	"github.com/readium/go-toolkit/pkg/pub"
)

func inferA11yMetadataInPublicationManifest(ctx context.Context, pub *pub.Publication, ignorableImages manifest.HashList) (*manifest.A11y, error) {
	mf := pub.Manifest
	inferredA11y := manifest.NewA11y()

	var manifestA11y manifest.A11y
	if mf.Metadata.Accessibility != nil {
		manifestA11y = *mf.Metadata.Accessibility
	} else {
		manifestA11y = manifest.NewA11y()
	}

	conformsToWCAGA := false
	conformsToWCAGAA := false
	for _, profile := range manifestA11y.ConformsTo {
		if profile == manifest.EPUBA11y10WCAG20A {
			conformsToWCAGA = true
		}
		if profile == manifest.EPUBA11y10WCAG20AA || profile == manifest.EPUBA11y10WCAG20AAA {
			conformsToWCAGA = true
			conformsToWCAGAA = true
		}
	}

	addFeature := func(f manifest.A11yFeature) {
		if !extensions.Contains(inferredA11y.Features, f) && !extensions.Contains(manifestA11y.Features, f) {
			inferredA11y.Features = append(inferredA11y.Features, f)
		}
	}

	allResources := append(mf.ReadingOrder, mf.Resources...)

	// Inferred textual if the publication is partially or fully accessible
	// (WCAG A or above).
	isTextual := conformsToWCAGA

	// ... or if a reflowable EPUB does not contain any image, audio or
	// video resource (inspect "resources" and "readingOrder" in RWPM), or
	// if the only image available can be identified as a cover.
	if !isTextual &&
		mf.ConformsTo(manifest.ProfileEPUB) &&
		mf.Metadata.EffectiveLayout() == manifest.LayoutReflowable {
		isTextual = true

		var hashAlgorithms []manifest.HashAlgorithm
		for _, hash := range ignorableImages {
			if !slices.Contains(hashAlgorithms, hash.Algorithm) {
				hashAlgorithms = append(hashAlgorithms, hash.Algorithm)
			}
		}
		ffs := fetcher.ToFS(ctx, pub.Fetcher)

		for _, link := range allResources {
			mt := link.MediaType
			if mt.IsAudio() ||
				mt.IsVideo() ||
				mt.Matches(&mediatype.PDF) {
				isTextual = false
				break
			} else if mt.IsBitmap() && !extensions.Contains(link.Rels, "cover") {
				if len(ignorableImages) > 0 {
					// We may want to consider doing this in parallel in a future version,
					// as it could be reading each resource in sequence from a remote source.
					link, err := analyzer.InspectImage(ffs, link, hashAlgorithms)
					if err != nil {
						return nil, err
					}
					hashes := link.Properties.Hash()
					canIgnore := false
					for _, ignorable := range ignorableImages {
						if v, ok := hashes.Find(ignorable.Algorithm); ok {
							if v.Equal(ignorable) {
								canIgnore = true
								break
							}
						}
					}

					// If image is ignored, isTextual remains true
					if canIgnore {
						continue
					}
				}
				isTextual = false
				break
			}
		}
	}

	if len(manifestA11y.AccessModes) == 0 {
		if isTextual {
			inferredA11y.AccessModes = append(inferredA11y.AccessModes, manifest.A11yAccessModeTextual)
		}

		// Inferred auditory if the publication contains a reference to an
		// audio or video resource (inspect "resources" and "readingOrder" in
		// RWPM).
		for _, link := range allResources {
			if link.MediaType.IsAudio() || link.MediaType.IsVideo() {
				inferredA11y.AccessModes = append(inferredA11y.AccessModes, manifest.A11yAccessModeAuditory)
				break
			}
		}

		// Inferred visual if the publications contain a reference to an image
		// or a video resource (inspect "resources" and "readingOrder" in
		// RWPM).
		for _, link := range allResources {
			if link.MediaType.IsBitmap() || link.MediaType.IsVideo() {
				inferredA11y.AccessModes = append(inferredA11y.AccessModes, manifest.A11yAccessModeVisual)
				break
			}
		}
	}

	if len(manifestA11y.AccessModesSufficient) == 0 {
		var accessMode manifest.A11yPrimaryAccessMode
		if isTextual {
			accessMode = manifest.A11yPrimaryAccessModeTextual
		}

		if accessMode == "" {
			if allResources.AllAreAudio() {
				// Inferred auditory if all references in the "readingOrder" are
				// identified as audio resources.
				accessMode = manifest.A11yPrimaryAccessModeAuditory

			} else if allResources.AllAreVisual() {
				// Inferred visual if all references in the "readingOrder" are
				// identified as images or video resources.
				accessMode = manifest.A11yPrimaryAccessModeVisual
			}
		}

		if accessMode != "" {
			inferredA11y.AccessModesSufficient = append(
				inferredA11y.AccessModesSufficient,
				[]manifest.A11yPrimaryAccessMode{accessMode},
			)
		}
	}

	if len(mf.TableOfContents) > 0 {
		addFeature(manifest.A11yFeatureTableOfContents)
	}

	if mf.ConformsTo(manifest.ProfileEPUB) {
		if _, hasPageList := mf.Subcollections["pageList"]; hasPageList {
			addFeature(manifest.A11yFeaturePageNavigation)
		}

		for _, link := range allResources {
			if extensions.Contains(link.Properties.Contains(), "mathml") {
				addFeature(manifest.A11yFeatureMathML)
				break
			}
		}

		for _, link := range mf.Resources {
			if link.MediaType.Matches(&mediatype.SMIL) {
				addFeature(manifest.A11yFeatureSynchronizedAudioText)
				break
			}
		}

		if mf.Metadata.EffectiveLayout() == manifest.LayoutReflowable && conformsToWCAGAA {
			addFeature(manifest.A11yFeatureDisplayTransformability)
		}
	}

	if inferredA11y.IsEmpty() {
		return nil, nil
	}
	return &inferredA11y, nil
}
