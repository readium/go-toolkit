package manifest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTDMFromJSON(t *testing.T) {
	rawJSON := map[string]interface{}{
		"policy":      "https://provider.com/policies/policy.json",
		"reservation": "all",
	}
	tdm, err := TDMFromJSON(rawJSON)
	assert.NoError(t, err)

	assert.Equal(t, "https://provider.com/policies/policy.json", tdm.Policy)
	assert.Equal(t, TDMReservationAll, tdm.Reservation)

	rawJSON = map[string]interface{}{
		"reservation": "none",
	}
	tdm, err = TDMFromJSON(rawJSON)
	assert.NoError(t, err)

	assert.Equal(t, "", tdm.Policy)
	assert.Equal(t, TDMReservationNone, tdm.Reservation)
}

func TestTDMMarshalJSON(t *testing.T) {
	tdm := TDM{
		Policy:      "https://provider.com/policies/policy.json",
		Reservation: TDMReservationAll,
	}
	rawJSON, err := json.Marshal(tdm)
	assert.NoError(t, err)

	expectedJSON := `{"policy":"https://provider.com/policies/policy.json","reservation":"all"}`
	assert.JSONEq(t, expectedJSON, string(rawJSON))

	tdm = TDM{
		Reservation: TDMReservationNone,
	}
	rawJSON, err = json.Marshal(tdm)
	assert.NoError(t, err)

	expectedJSON = `{"reservation":"none"}`
	assert.JSONEq(t, expectedJSON, string(rawJSON))
}
