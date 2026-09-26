package converter

import (
	"testing"

	"github.com/readium/go-toolkit/pkg/guidednavigation"
	"github.com/stretchr/testify/assert"
)

func TestNavigationObjectConvertRetainsChildrenAndOrder(t *testing.T) {
	text := func(id string) guidednavigation.GuidedNavigationObject {
		return guidednavigation.GuidedNavigationObject{
			ID:   id,
			Text: guidednavigation.GuidedNavigationText{Plain: id},
		}
	}
	existing, direct, first, second := text("existing"), text("direct"), text("first"), text("second")
	root := navigationObject{
		object: guidednavigation.GuidedNavigationObject{
			Children: []guidednavigation.GuidedNavigationObject{existing},
		},
		children: []*navigationObject{
			{}, // Empty nodes do not produce children.
			{object: direct},
			{children: []*navigationObject{ // Transparent wrappers flatten in place.
				{object: first},
				{object: second},
			}},
		},
	}

	result := root.convert()
	assert.Equal(t, []guidednavigation.GuidedNavigationObject{existing, direct, first, second}, result.Children)
	assert.Equal(t, []guidednavigation.GuidedNavigationObject{existing}, root.object.Children)
}

func TestNavigationObjectConvertEmptyChildrenNilness(t *testing.T) {
	for _, children := range [][]guidednavigation.GuidedNavigationObject{nil, {}} {
		root := navigationObject{
			object:   guidednavigation.GuidedNavigationObject{Children: children},
			children: []*navigationObject{{}, {children: []*navigationObject{{}}}},
		}
		assert.Equal(t, children, root.convert().Children)
	}
}
