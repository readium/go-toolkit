package guidednavigation

import (
	"encoding/json"
	"testing"

	"github.com/readium/go-toolkit/pkg/util/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextMarshalPlainOnly(t *testing.T) {
	b, err := json.Marshal(GuidedNavigationText{Plain: "Hello"})
	require.NoError(t, err)
	assert.JSONEq(t, `"Hello"`, string(b))
}

func TestTextMarshalFull(t *testing.T) {
	b, err := json.Marshal(GuidedNavigationText{
		Plain:    "Hello",
		SSML:     "<emphasis>Hello</emphasis>",
		Language: "en",
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"plain": "Hello", "ssml": "<emphasis>Hello</emphasis>", "language": "en"}`, string(b))
}

func TestTextUnmarshalString(t *testing.T) {
	var text GuidedNavigationText
	require.NoError(t, json.Unmarshal([]byte(`"Hello"`), &text))
	assert.Equal(t, GuidedNavigationText{Plain: "Hello"}, text)
}

func TestTextUnmarshalObject(t *testing.T) {
	var text GuidedNavigationText
	require.NoError(t, json.Unmarshal([]byte(`{"plain": "Hi", "language": "en"}`), &text))
	assert.Equal(t, GuidedNavigationText{Plain: "Hi", Language: "en"}, text)
}

func TestDescriptionMarshalPlainString(t *testing.T) {
	b, err := json.Marshal(NewTextDescription("A cool image"))
	require.NoError(t, err)
	assert.JSONEq(t, `"A cool image"`, string(b))
}

func TestDescriptionMarshalObject(t *testing.T) {
	b, err := json.Marshal(GuidedNavigationDescription{
		AudioRef: url.MustURLFromString("description.mp3"),
		Text:     GuidedNavigationText{Plain: "A described panel"},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"audioref": "description.mp3", "text": "A described panel"}`, string(b))
}

func TestDescriptionUnmarshalString(t *testing.T) {
	var d GuidedNavigationDescription
	require.NoError(t, json.Unmarshal([]byte(`"A cool image"`), &d))
	assert.Equal(t, NewTextDescription("A cool image"), d)
}

func TestDescriptionUnmarshalObject(t *testing.T) {
	var d GuidedNavigationDescription
	require.NoError(t, json.Unmarshal([]byte(`{"audioref": "audio/page1.mp3", "text": "Pepper walks away."}`), &d))
	assert.Equal(t, "audio/page1.mp3", d.AudioRef.String())
	assert.Equal(t, "Pepper walks away.", d.Text.Plain)
}

func TestObjectMarshal(t *testing.T) {
	obj := GuidedNavigationObject{
		ID:       "image1",
		ImgRef:   url.MustURLFromString("image.jpg"),
		VideoRef: url.MustURLFromString("movie.mp4"),
		Role:     []GuidedNavigationRole{RoleImage},
		Children: []GuidedNavigationObject{
			{Text: GuidedNavigationText{Plain: "Child"}},
		},
		Description: NewTextDescription("Description"),
	}
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"id": "image1",
		"imgref": "image.jpg",
		"videoref": "movie.mp4",
		"role": ["image"],
		"children": [{"text": "Child"}],
		"description": "Description"
	}`, string(b))
}

func TestObjectPredicates(t *testing.T) {
	assert.True(t, GuidedNavigationObject{}.Empty())
	assert.True(t, GuidedNavigationObject{Role: []GuidedNavigationRole{RoleSeparator}}.Empty(),
		"an object with only roles carries no content")
	assert.False(t, GuidedNavigationObject{VideoRef: url.MustURLFromString("v.mp4")}.Empty())
	assert.False(t, GuidedNavigationObject{Description: NewTextDescription("d")}.Empty())

	wrapper := GuidedNavigationObject{Children: []GuidedNavigationObject{{Text: GuidedNavigationText{Plain: "t"}}}}
	assert.True(t, wrapper.ChildrenOnly())
	wrapper.ID = "id1"
	assert.False(t, wrapper.ChildrenOnly(), "an object with an id is referenced from SSML and can't be dissolved")

	assert.True(t, GuidedNavigationObject{Text: GuidedNavigationText{Plain: "t"}}.TextOnly())
	assert.False(t, GuidedNavigationObject{Text: GuidedNavigationText{Plain: "t"}, ID: "x"}.TextOnly())
}
