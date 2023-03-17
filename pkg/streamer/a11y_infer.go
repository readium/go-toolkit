package streamer

import (
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

func inferA11yMetadataFromManifest(mf manifest.Manifest) *manifest.A11y {
	inferredA11y := manifest.NewA11y()

	var manifestA11y manifest.A11y
	if mf.Metadata.Accessibility != nil {
		manifestA11y = *mf.Metadata.Accessibility
	} else {
		manifestA11y = manifest.NewA11y()
	}

	addFeature := func(f manifest.A11yFeature) {
		if !extensions.Contains(inferredA11y.Features, f) && !extensions.Contains(manifestA11y.Features, f) {
			inferredA11y.Features = append(inferredA11y.Features, f)
		}
	}

	allResources := append(mf.ReadingOrder, mf.Resources...)

	if len(manifestA11y.AccessModes) == 0 {
		for _, link := range allResources {
			if link.MediaType().IsAudio() || link.MediaType().IsVideo() {
				inferredA11y.AccessModes = append(inferredA11y.AccessModes, manifest.A11yAccessModeAuditory)
				break
			}
		}

		for _, link := range allResources {
			if link.MediaType().IsBitmap() || link.MediaType().IsVideo() {
				inferredA11y.AccessModes = append(inferredA11y.AccessModes, manifest.A11yAccessModeVisual)
				break
			}
		}
	}

	if len(manifestA11y.AccessModesSufficient) == 0 {
		setTextual := false

		for _, profile := range manifestA11y.ConformsTo {
			if profile == manifest.EPUBA11y10WCAG20A ||
				profile == manifest.EPUBA11y10WCAG20AA ||
				profile == manifest.EPUBA11y10WCAG20AAA {
				setTextual = true
				break
			}
		}

		if !setTextual {
			setTextual = true
			for _, link := range allResources {
				mt := link.MediaType()
				if mt.IsAudio() ||
					mt.IsVideo() ||
					(mt.IsBitmap() && !extensions.Contains(link.Rels, "cover")) ||
					mt.Matches(&mediatype.PDF) {

					setTextual = false
					break
				}
			}
		}

		if setTextual {
			inferredA11y.AccessModesSufficient = append(
				inferredA11y.AccessModesSufficient,
				[]manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual},
			)
		}

		if allResources.AllAreAudio() {
			inferredA11y.AccessModesSufficient = append(
				inferredA11y.AccessModesSufficient,
				[]manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeAuditory},
			)
		}
	}

	if mf.TableOfContents != nil && len(mf.TableOfContents) > 0 {
		addFeature(manifest.A11yFeatureTableOfContents)
	}

	if mf.ConformsTo(manifest.ProfileEPUB) {
		if _, hasPageList := mf.Subcollections["pageList"]; hasPageList {
			addFeature(manifest.A11yFeaturePrintPageNumbers)
		}

		for _, link := range allResources {
			if extensions.Contains(link.Properties.Contains(), "mathml") {
				addFeature(manifest.A11yFeatureMathML)
				break
			}
		}

		if mf.Metadata.Presentation != nil && *mf.Metadata.Presentation.Layout == manifest.EPUBLayoutReflowable {
			if extensions.Contains(manifestA11y.ConformsTo, manifest.EPUBA11y10WCAG20AA) ||
				extensions.Contains(manifestA11y.ConformsTo, manifest.EPUBA11y10WCAG20AAA) {
				addFeature(manifest.A11yFeatureDisplayTransformability)
			}
		}
	}

	if inferredA11y.IsEmpty() {
		return nil
	}
	return &inferredA11y
}
