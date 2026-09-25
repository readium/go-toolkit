// Package obfuscation implements the repeating-key XOR used by EPUB fonts.
package obfuscation

import "crypto/subtle"

// XOR transforms data in place using a repeating key, starting at the given
// non-negative offset in the original resource. An empty key leaves data intact.
func XOR(data, key []byte, offset int64) {
	if len(data) == 0 || len(key) == 0 {
		return
	}

	// Both EPUB font algorithms fit in this stack buffer (1024 or 1040 bytes).
	// Expand the key with bulk copies so the XOR can use the standard library's
	// optimized implementation without a modulo operation for every byte.
	var expanded [1040]byte
	phase := int(offset % int64(len(key)))
	for len(data) > 0 {
		n := min(len(data), len(expanded))
		mask := expanded[:n]
		filled := copy(mask, key[phase:])
		filled += copy(mask[filled:], key[:phase])
		for filled < n {
			filled += copy(mask[filled:], mask[:filled])
		}
		subtle.XORBytes(data[:n], data[:n], mask)
		data = data[n:]
		if len(data) > 0 {
			phase = (phase + n) % len(key)
		}
	}
}
