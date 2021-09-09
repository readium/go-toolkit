package lcp

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/readium/go-toolkit/pkg/pub"
)

// HasGoodKey check if the Publication contains hashed pass phrase or pass
// phrase and these information are good
func HasGoodKey(publication *pub.Manifest) bool {

	hashPassphrase := publication.GetBytesFromInternal("lcp_hash_passphrase")
	if len(hashPassphrase) > 0 {
		ret := CheckHashPassphrase(publication, hashPassphrase)
		if ret == true {
			return true
		}
	}

	passphrase := publication.GetStringFromInternal("lcp_passphrase")
	if passphrase != "" {
		ret2 := CheckPassphrase(publication, passphrase)
		if ret2 == true {
			return true
		}
	}

	return false
}

// CheckPassphrase create and store the hash and call the check for hashed pass phrse
func CheckPassphrase(publication *pub.Manifest, passphrase string) bool {
	hashPasswd := sha256.Sum256([]byte(passphrase))
	publication.AddToInternal("lcp_hash_passphrase", hashPasswd[:])
	return CheckHashPassphrase(publication, hashPasswd[:])
}

// CheckHashPassphrase check if the hash is good by decrypting the key check
// and compare it to the license id
func CheckHashPassphrase(publication *pub.Manifest, hashPassphrase []byte) bool {
	var cipher bytes.Buffer

	keyCheck := publication.GetBytesFromInternal("lcp_user_key_check")
	lcpID := publication.GetStringFromInternal("lcp_id")

	keyCheckReader := bytes.NewBuffer(keyCheck)
	err := decryptAESCBC(hashPassphrase, keyCheckReader, &cipher)
	if err != nil {
		fmt.Println(err)
		return false
	}

	if cipher.String() == lcpID {
		return true
	}

	return false
}
