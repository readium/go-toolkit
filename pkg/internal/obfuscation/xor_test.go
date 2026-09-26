package obfuscation

import (
	"bytes"
	"fmt"
	"testing"
)

func TestXOR(t *testing.T) {
	for _, keyLen := range []int{0, 1, 16, 20, 1057} {
		key := make([]byte, keyLen)
		for i := range key {
			key[i] = byte(i*13 + 7)
		}
		for _, offset := range []int64{0, 1, 13, 15, 16, 19, 20, 1023, 1040} {
			for _, size := range []int{0, 1, 15, 16, 17, 63, 64, 65, 1024, 1040, 1041, 4096} {
				t.Run(fmt.Sprintf("key%d/offset%d/size%d", keyLen, offset, size), func(t *testing.T) {
					original := make([]byte, size)
					for i := range original {
						original[i] = byte(i*3 + 1)
					}
					want := bytes.Clone(original)
					if keyLen > 0 {
						for i := range want {
							want[i] ^= key[(offset+int64(i))%int64(keyLen)]
						}
					}
					got := bytes.Clone(original)
					XOR(got, key, offset)
					if !bytes.Equal(got, want) {
						t.Fatal("result differs from repeating-key XOR")
					}
					XOR(got, key, offset)
					if !bytes.Equal(got, original) {
						t.Fatal("second XOR did not restore the original data")
					}
				})
			}
		}
	}
}

// xorModulo preserves the original byte-at-a-time font transform as a benchmark
// baseline. Both implementations receive data already limited to the font prefix.
func xorModulo(data, key []byte, offset int64) {
	if len(key) == 0 {
		return
	}
	keyLen := int64(len(key))
	for i := int64(0); i < int64(len(data)); i++ {
		data[i] ^= key[(offset+i)%keyLen]
	}
}

// BenchmarkXOR includes key expansion in the optimized measurement, but excludes
// fixture setup, key derivation, and I/O from both implementations.
func BenchmarkXOR(b *testing.B) {
	for _, keyLen := range []int{16, 20} {
		size := 1024
		if keyLen == 20 {
			size = 1040
		}
		key := make([]byte, keyLen)
		for i := range key {
			key[i] = byte(i*13 + 7)
		}
		for _, span := range []struct {
			name   string
			offset int64
			size   int
		}{
			{"Full", 0, size},
			{"Unaligned", 13, size - 13},
			{"Short", 13, 16},
			{"Tail", int64(size - 1), 1},
		} {
			for _, impl := range []struct {
				name string
				xor  func([]byte, []byte, int64)
			}{
				{"Modulo", xorModulo},
				{"XORBytes", XOR},
			} {
				b.Run(fmt.Sprintf("key%d/%s/%s", keyLen, span.name, impl.name), func(b *testing.B) {
					data := bytes.Repeat([]byte{0x5a}, span.size)
					b.SetBytes(int64(len(data)))
					b.ReportAllocs()
					for b.Loop() {
						impl.xor(data, key, span.offset)
					}
					// Each iteration toggles the same input. Check the final
					// bytes after B.Loop has stopped the timer.
					want := bytes.Repeat([]byte{0x5a}, span.size)
					if b.N%2 != 0 {
						xorModulo(want, key, span.offset)
					}
					if !bytes.Equal(data, want) {
						b.Fatal("incorrect benchmark result")
					}
				})
			}
		}
	}
}
