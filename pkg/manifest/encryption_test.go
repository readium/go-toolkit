package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptionUnmarshalMinimalJSON(t *testing.T) {
	var m Encryption
	assert.NoError(t, json.Unmarshal([]byte(`{"algorithm": "http://algo"}`), &m))
	assert.Equal(
		t,
		Encryption{
			Algorithm: "http://algo",
		},
		m,
		"unmarshalled JSON object should be equal to Encryption object",
	)
}

func TestEncryptionUnmarshalFullJSON(t *testing.T) {
	var m Encryption
	assert.NoError(t, json.Unmarshal([]byte(`{
		"algorithm": "http://algo",
		"compression": "gzip",
		"originalLength": 42099,
		"profile": "http://profile",
		"scheme": "http://scheme"
	}`), &m))
	assert.Equal(t, Encryption{
		Algorithm:      "http://algo",
		Compression:    "gzip",
		OriginalLength: 42099,
		Profile:        "http://profile",
		Scheme:         "http://scheme",
	}, m, "unmarshalled JSON object should be equal to Encryption object")
}

func TestEncryptionUnmarshalNullJSON(t *testing.T) {
	enc, err := EncryptionFromJSON(nil)
	assert.NoError(t, err)
	assert.Nil(t, enc)
}

func TestEncryptionRequiresAlgorithm(t *testing.T) {
	var m Encryption
	assert.Error(t, json.Unmarshal([]byte(`{"compression": "gzip"}`), &m), "algorithm is required for Encryption objects")
}

func TestEncryptionMarshalMinimalJSON(t *testing.T) {
	m := Encryption{
		Algorithm: "http://algo",
	}
	data, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte(`{"algorithm":"http://algo"}`), "unmarshalled JSON object should be equal to Encryption object")
}

func TestEncryptionMarshalFullJSON(t *testing.T) {
	m := Encryption{
		Algorithm:      "http://algo",
		Compression:    "gzip",
		OriginalLength: 42099,
		Profile:        "http://profile",
		Scheme:         "http://scheme",
	}
	data, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(
		t,
		data,
		[]byte(`{"scheme":"http://scheme","profile":"http://profile","algorithm":"http://algo","compression":"gzip","originalLength":42099}`),
		"unmarshalled JSON object should be equal to Encryption object",
	)
}
