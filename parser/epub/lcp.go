package epub

import "time"

// LCP struct for the lcp license
type LCP struct {
	ID         string     `json:"id"`
	Issued     *time.Time `json:"issued"`
	Provider   string     `json:"provider"`
	Updated    *time.Time `json:"updated,omitempty"`
	Encryption struct {
		Profile    string `json:"profile"`
		ContentKey struct {
			EncryptedValue []byte `json:"encrypted_value"`
			Algorithm      string `json:"algorithm"`
		} `json:"content_key"`
		UserKey struct {
			TextHint  string `json:"text_hint"`
			Algorithm string `json:"algorithm"`
			KeyCheck  []byte `json:"key_check"`
		} `json:"user_key"`
	} `json:"encryption"`
	Links []struct {
		Rel       string `json:"rel"`
		Href      string `json:"href"`
		Title     string `json:"title"`
		Type      string `json:"type"`
		Templated string `json:"templated"`
		Profile   string `json:"profile"`
		Length    int    `json:"length"`
		Hash      []byte `json:"hash"`
	} `json:"links"`
	Rights struct {
		Print int        `json:"print"`
		Copy  int        `json:"copy"`
		Start *time.Time `json:"start"`
		End   *time.Time `json:"end"`
	}
	User struct {
		ID        string   `json:"id"`
		Email     string   `json:"email"`
		Name      string   `json:"name"`
		Encrypted []string `json:"encrypted"`
	}
	Signature struct {
		Algorithm   string `json:"algorithm"`
		Certificate []byte `json:"certificate"`
		Value       []byte `json:"value"`
	}
}
