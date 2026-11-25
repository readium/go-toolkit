package lcp

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/pkg/errors"
)

// KeySize is the size in bytes of the keys used by LCP (AES-256).
const KeySize = 32

// ContentKey is the symmetric key encrypting the resources of a publication.
//
// It is generated once per publication, stored by the License Server, and
// delivered to reading systems encrypted with the user key, in the
// publication's license.
type ContentKey []byte

// GenerateContentKey returns a new random content key.
func GenerateContentKey() (ContentKey, error) {
	k := make(ContentKey, KeySize)
	if _, err := rand.Read(k); err != nil {
		return nil, errors.Wrap(err, "lcp: failed generating content key")
	}
	return k, nil
}

// ContentKeyFromBase64 decodes a base64-encoded (standard encoding) content
// key, as stored by the LCP License Server.
func ContentKeyFromBase64(s string) (ContentKey, error) {
	k, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: invalid base64 content key")
	}
	key := ContentKey(k)
	if err := key.Validate(); err != nil {
		return nil, err
	}
	return key, nil
}

// Base64 returns the base64-encoded (standard encoding) key.
func (k ContentKey) Base64() string {
	return base64.StdEncoding.EncodeToString(k)
}

// Validate returns [ErrInvalidKey] when the key is not [KeySize] bytes long.
func (k ContentKey) Validate() error {
	return validateKey(k)
}

func validateKey(key []byte) error {
	if len(key) != KeySize {
		return ErrInvalidKey
	}
	return nil
}
