package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringsJSON(t *testing.T) {
	for input, expected := range map[string]Strings{
		`"a"`:        {"a"},
		`["a"]`:      {"a"},
		`["a", "b"]`: {"a", "b"},
		`[]`:         {},
	} {
		var s Strings
		require.NoError(t, json.Unmarshal([]byte(input), &s), input)
		assert.Equal(t, expected, s, input)
	}

	var s Strings
	assert.Error(t, json.Unmarshal([]byte(`42`), &s))

	data, err := json.Marshal(Strings{"a"})
	require.NoError(t, err)
	assert.JSONEq(t, `"a"`, string(data))
	data, err = json.Marshal(Strings{"a", "b"})
	require.NoError(t, err)
	assert.JSONEq(t, `["a", "b"]`, string(data))
}
