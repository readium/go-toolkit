package streamer

import (
	"github.com/readium/go-toolkit/pkg/internal/extensions"
	"github.com/readium/go-toolkit/pkg/manifest"
	"github.com/readium/go-toolkit/pkg/mediatype"
)

func inferA11yMetadataFromManifest(mf manifest.Manifest) *manifest.A11y {
	var a11y manifest.A11y
	if mf.Metadata.Accessibility != nil {
		a11y = *mf.Metadata.Accessibility
	} else {
		a11y = manifest.NewA11y()
	}

	accessModes := a11y.AccessModes
	accessModesSufficient := a11y.AccessModesSufficient
	features := a11y.Features

	addFeature := func(f manifest.A11yFeature) {
		if !extensions.Contains(features, f) {
			features = append(features, f)
		}
	}

	allResources := append(mf.ReadingOrder, mf.Resources...)

	if len(accessModes) == 0 {
		for _, link := range allResources {
			if link.MediaType().IsAudio() || link.MediaType().IsVideo() {
				accessModes = append(accessModes, manifest.A11yAccessModeAuditory)
				break
			}
		}

		for _, link := range allResources {
			if link.MediaType().IsBitmap() || link.MediaType().IsVideo() {
				accessModes = append(accessModes, manifest.A11yAccessModeVisual)
				break
			}
		}
	}

	if len(accessModesSufficient) == 0 {
		setTextual := false

		for _, profile := range a11y.ConformsTo {
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
			accessModesSufficient = append(
				accessModesSufficient,
				[]manifest.A11yPrimaryAccessMode{manifest.A11yPrimaryAccessModeTextual},
			)
		}

		if allResources.AllAreAudio() {
			accessModesSufficient = append(
				accessModesSufficient,
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
			if extensions.Contains(a11y.ConformsTo, manifest.EPUBA11y10WCAG20AA) ||
				extensions.Contains(a11y.ConformsTo, manifest.EPUBA11y10WCAG20AAA) {
				addFeature(manifest.A11yFeatureDisplayTransformability)
			}
		}
	}

	a11y.AccessModes = accessModes
	a11y.AccessModesSufficient = accessModesSufficient
	a11y.Features = features
	if a11y.IsEmpty() {
		return nil
	}
	return &a11y
}
