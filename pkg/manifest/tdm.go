package manifest

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// TDMRep (Text & Data Mining Reservation Protocol)
//
// https://www.w3.org/community/reports/tdmrep/CG-FINAL-tdmrep-20240510/
type TDM struct {
	Policy      string         `json:"policy,omitempty"`
	Reservation TDMReservation `json:"reservation,omitempty"`
}

func (t *TDM) IsEmpty() bool {
	return t.Policy == "" && t.Reservation == ""
}

type TDMReservation string

const (
	TDMReservationAll  TDMReservation = "all"
	TDMReservationNone TDMReservation = "none"
)

func (t TDMReservation) String() string {
	return string(t)
}

func TDMFromJSON(rawJSON map[string]any) (*TDM, error) {
	if rawJSON == nil {
		return nil, nil
	}

	t := &TDM{}

	if policy, ok := rawJSON["policy"].(string); ok {
		t.Policy = policy
	}

	if reservation, ok := rawJSON["reservation"].(string); ok {
		t.Reservation = TDMReservation(reservation)
	}

	if t.IsEmpty() {
		return nil, nil
	}

	return t, nil
}

func (t *TDM) UnmarshalJSON(data []byte) error {
	var d any
	err := json.Unmarshal(data, &d)
	if err != nil {
		return err
	}

	mp, ok := d.(map[string]any)
	if !ok {
		return errors.New("tdm object not a map with string keys")
	}

	ft, err := TDMFromJSON(mp)
	if err != nil {
		return err
	}
	*t = *ft
	return nil
}
