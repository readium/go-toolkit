package lcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/pkg/errors"
)

// UserKey is the key unlocking the licenses of a user: it decrypts their
// content key. It is derived from the user passphrase, in a way which
// depends on the encryption profile of the license.
type UserKey []byte

// UserKeyFromPassphrase derives the user key from the user passphrase, for
// the given encryption profile.
//
// Only the basic profile, where the user key is the SHA-256 hash of the
// passphrase, is supported: production profiles return
// [ErrUnsupportedProfile]. For those, the user key must be obtained through
// authorized means, and given to [UserKeyFromHex]. Other profiles return
// [ErrUnknownProfile].
func UserKeyFromPassphrase(passphrase string, profile Profile) (UserKey, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if profile.IsProduction() {
		return nil, errors.Wrapf(ErrUnsupportedProfile, "the user key of the production profile %s can't be derived from a passphrase", profile)
	}
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:], nil
}

// UserKeyFromHex decodes a hex-encoded user key. For the basic profile, this
// is the hex-encoded SHA-256 hash of the passphrase, which providers send to
// the License Server.
func UserKeyFromHex(s string) (UserKey, error) {
	k, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.Wrap(err, "lcp: invalid hex user key")
	}
	key := UserKey(k)
	if err := key.Validate(); err != nil {
		return nil, err
	}
	return key, nil
}

// Validate returns [ErrInvalidKey] when the key is not [KeySize] bytes long.
func (k UserKey) Validate() error {
	return validateKey(k)
}

// CheckUserKey returns nil when the user key unlocks the license, and
// [ErrWrongUserKey] otherwise. The key check of the license is the license
// identifier, encrypted with the user key. Licenses of an unknown profile,
// whose algorithms aren't known either, return [ErrUnknownProfile].
func (l *License) CheckUserKey(uk UserKey) error {
	if l == nil {
		return errNilLicense
	}
	if err := uk.Validate(); err != nil {
		return err
	}
	if err := l.Encryption.Profile.Validate(); err != nil {
		return err
	}
	if l.Encryption.ContentKey.Algorithm != AlgorithmAES256CBC {
		return errors.Wrapf(ErrInvalidLicense, "unsupported content key algorithm %q", l.Encryption.ContentKey.Algorithm)
	}
	var id bytes.Buffer
	if _, err := Decrypt(uk, bytes.NewReader(l.Encryption.UserKey.KeyCheck), &id); err != nil {
		if errors.Is(err, ErrInvalidPadding) {
			return ErrWrongUserKey
		}
		return errors.Wrap(ErrInvalidLicense, "invalid key check: "+err.Error())
	}
	if id.String() != l.ID {
		return ErrWrongUserKey
	}
	return nil
}

// ContentKey unlocks the license with the user key, and returns the content
// key of the publication. It returns [ErrWrongUserKey] when the user key
// doesn't match the license.
func (l *License) ContentKey(uk UserKey) (ContentKey, error) {
	if err := l.CheckUserKey(uk); err != nil {
		return nil, err
	}
	var key bytes.Buffer
	if _, err := Decrypt(uk, bytes.NewReader(l.Encryption.ContentKey.EncryptedValue), &key); err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, "invalid encrypted content key: "+err.Error())
	}
	ck := ContentKey(key.Bytes())
	if err := ck.Validate(); err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, "invalid content key length")
	}
	return ck, nil
}

// ContentKeyFromPassphrase unlocks the license with the user passphrase, and
// returns the content key of the publication. It only supports licenses of
// the basic profile: see [UserKeyFromPassphrase].
func (l *License) ContentKeyFromPassphrase(passphrase string) (ContentKey, error) {
	if l == nil {
		return nil, errNilLicense
	}
	// The profile first: the algorithms of other profiles may differ.
	uk, err := UserKeyFromPassphrase(passphrase, l.Encryption.Profile)
	if err != nil {
		return nil, err
	}
	if alg := l.Encryption.UserKey.Algorithm; alg != "" && alg != AlgorithmSHA256 {
		return nil, errors.Wrapf(ErrUnsupportedProfile, "unsupported user key algorithm %q", alg)
	}
	return l.ContentKey(uk)
}

// DecryptUserFields returns the user fields listed as encrypted in the
// license (e.g. "email", "name", or provider-specific fields), decrypted
// with the user key.
func (l *License) DecryptUserFields(uk UserKey) (map[string]string, error) {
	if err := l.CheckUserKey(uk); err != nil {
		return nil, err
	}
	if len(l.User.Encrypted) == 0 {
		return map[string]string{}, nil
	}

	// Read the fields from the signed document, which may hold fields
	// unknown to LicenseUser.
	canonical, err := l.canonicalJSON()
	if err != nil {
		return nil, err
	}
	var doc struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return nil, errors.Wrap(ErrInvalidLicense, err.Error())
	}
	user := doc.User

	fields := make(map[string]string, len(l.User.Encrypted))
	for _, name := range l.User.Encrypted {
		if _, decrypted := fields[name]; decrypted {
			continue
		}
		value, ok := user[name].(string)
		if !ok {
			continue
		}
		ciphertext, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidLicense, "user field %q is not base64: %s", name, err)
		}
		var plain bytes.Buffer
		if _, err := Decrypt(uk, bytes.NewReader(ciphertext), &plain); err != nil {
			return nil, errors.Wrapf(ErrInvalidLicense, "failed decrypting user field %q: %s", name, err)
		}
		fields[name] = plain.String()
	}
	return fields, nil
}
