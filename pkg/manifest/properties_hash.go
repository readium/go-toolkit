package manifest

import (
	"crypto/subtle"

	"github.com/pkg/errors"
)

type HashAlgorithm string

// The following hashes keys are reserved for future use, but not necessarily supported by the toolkit.
// If you are using a hash algorithm not listed here, it's better to use a URI, such as `https://blurha.sh`.
// If there's an algorithm you think should be recognized, let us know.
const (
	HashAlgorithmBlake2b  HashAlgorithm = "blake2b"
	HashAlgorithmBlake2s  HashAlgorithm = "blake2s"
	HashAlgorithmBlake3   HashAlgorithm = "blake3"
	HashAlgorithmSHA512   HashAlgorithm = "sha512"
	HashAlgorithmSHA256   HashAlgorithm = "sha256"
	HashAlgorithmSHA1     HashAlgorithm = "sha1"
	HashAlgorithmMD5      HashAlgorithm = "md5"
	HashAlgorithmXXH3     HashAlgorithm = "xxh3"
	HashAlgorithmCRC32    HashAlgorithm = "crc32"
	HashAlgorithmPhashDCT HashAlgorithm = "phash-dct"
)

type HashValue struct {
	Algorithm HashAlgorithm `json:"algorithm"`
	Value     string        `json:"value"`
}

func (h HashValue) String() string {
	return string(h.Algorithm) + ":" + h.Value
}

func (h HashValue) Equal(other HashValue) bool {
	if h.Algorithm != other.Algorithm {
		return false
	}

	// Cast the strings to []byte because we don't have a standard encoding to decode from for the values
	// We should probably decide on one, such as base64 std encoding
	return subtle.ConstantTimeCompare([]byte(h.Value), []byte(other.Value)) == 1
}

type HashList []HashValue

func (h HashList) Find(algorithm HashAlgorithm) (HashValue, bool) {
	for _, hash := range h {
		if hash.Algorithm == algorithm {
			return hash, true
		}
	}
	return HashValue{}, false
}

func (h HashList) Value(algorithm HashAlgorithm) (string, bool) {
	for _, hash := range h {
		if hash.Algorithm == algorithm {
			return hash.Value, true
		}
	}
	return "", false
}

func (h *HashList) Deduplicate() {
	seen := make(map[HashAlgorithm]struct{})
	var unique HashList
	for _, hash := range *h {
		if _, ok := seen[hash.Algorithm]; !ok {
			seen[hash.Algorithm] = struct{}{}
			unique = append(unique, hash)
		}
	}
	*h = unique
}

func HashListFromJSONArray(rawJsonArray []interface{}) (HashList, error) {
	var hashes HashList
	for _, item := range rawJsonArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("invalid hash item: %v", item)
		}
		hashValue := HashValue{
			Algorithm: itemMap["algorithm"].(HashAlgorithm),
			Value:     itemMap["value"].(string),
		}
		hashes = append(hashes, hashValue)
	}
	return hashes, nil
}

func (h HashList) ToJSONArray() []interface{} {
	jsonArray := make([]interface{}, 0, len(h))
	for _, hash := range h {
		jsonArray = append(jsonArray, map[string]interface{}{
			"algorithm": hash.Algorithm,
			"value":     hash.Value,
		})
	}
	return jsonArray
}
