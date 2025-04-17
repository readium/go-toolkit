package manifest

import "github.com/pkg/errors"

type HashAlgorithm string

const (
	HashAlgorithmBlake2b  HashAlgorithm = "blake2b"
	HashAlgorithmBlake2s  HashAlgorithm = "blake2s"
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

type HashList []HashValue

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
