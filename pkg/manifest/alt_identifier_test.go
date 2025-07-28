package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAltIdentifierUnmarshalString(t *testing.T) {
	ai, err := AltIdentifierFromJSON("https://example.com/alt-id")
	require.NoError(t, err)
	require.Equal(t, &AltIdentifier{
		Value: "https://example.com/alt-id",
	}, ai, "parsed JSON string should be equal to string")
}

func TestAltIdentifierUnmarshalMinimalJSON(t *testing.T) {
	var ai AltIdentifier
	err := ai.UnmarshalJSON([]byte(`{"value":"https://example.com/alt-id"}`))
	require.NoError(t, err)

	require.Equal(t, &AltIdentifier{
		Value: "https://example.com/alt-id",
	}, &ai, "parsed JSON object should be equal to AltIdentifier object")
}

func TestAltIdentifierUnmarshalFullJSON(t *testing.T) {
	var ai AltIdentifier
	err := ai.UnmarshalJSON([]byte(`{
		"value": "https://example.com/alt-id",
		"scheme": "http://example.com/scheme"
	}`))
	require.NoError(t, err)

	require.Equal(t, &AltIdentifier{
		Value:  "https://example.com/alt-id",
		Scheme: "http://example.com/scheme",
	}, &ai, "parsed JSON object should be equal to AltIdentifier object")
}

func TestAltIdentifierUnmarshalNilJSON(t *testing.T) {
	ai, err := AltIdentifierFromJSON(nil)
	require.NoError(t, err)
	require.Nil(t, ai, "should return nil for nil JSON input")
}

func TestAltIdentifierRequiresValue(t *testing.T) {
	var ai AltIdentifier
	require.Error(t, json.Unmarshal([]byte(`{"scheme":"http://example.com/scheme"}`), &ai), "value is required for AltIdentifier objects")
}

func TestAltIdentifierFromJSONArray(t *testing.T) {
	var ais []AltIdentifier
	err := json.Unmarshal([]byte(`["id1", {"value":"id2", "scheme":"http://example.com/scheme"}]`), &ais)
	require.NoError(t, err)
	require.Len(t, ais, 2)
	require.Equal(t, []AltIdentifier{
		{Value: "id1"},
		{Value: "id2", Scheme: "http://example.com/scheme"},
	}, ais, "parsed JSON array should match expected AltIdentifier objects")
}

func TestAltIdentifierUnmarshalNilJSONArray(t *testing.T) {
	ais, err := AltIdentifierFromJSONArray(nil)
	require.NoError(t, err)
	require.Empty(t, ais)
}

func TestAltIdentifierUnmarshalJSONArrayString(t *testing.T) {
	ais, err := AltIdentifierFromJSONArray("https://example.com/alt-id")
	require.NoError(t, err)
	require.Len(t, ais, 1)
	require.Equal(t, []AltIdentifier{{Value: "https://example.com/alt-id"}}, ais, "parsed JSON string should be converted to single AltIdentifier")
}

func TestAltIdentifierMinimalJSON(t *testing.T) {
	bin, err := json.Marshal(AltIdentifier{Value: "https://example.com/alt-id"})
	require.NoError(t, err)
	require.JSONEq(t, string(bin), `"https://example.com/alt-id"`)
}

func TestAltIdentifierFullJSON(t *testing.T) {
	bin, err := json.Marshal(AltIdentifier{
		Value:  "https://example.com/alt-id",
		Scheme: "http://example.com/scheme",
	})
	require.NoError(t, err)
	require.JSONEq(t, string(bin), `{"value":"https://example.com/alt-id", "scheme":"http://example.com/scheme"}`)
}

func TestAltIdentifierJSONArray(t *testing.T) {
	bin, err := json.Marshal([]AltIdentifier{
		{Value: "https://example.com/alt-id1"},
		{Value: "https://example.com/alt-id2", Scheme: "http://example.com/scheme"},
	})
	require.NoError(t, err)
	require.JSONEq(t, string(bin), `[
		"https://example.com/alt-id1",
		{"value":"https://example.com/alt-id2", "scheme":"http://example.com/scheme"}
	]`)
}
