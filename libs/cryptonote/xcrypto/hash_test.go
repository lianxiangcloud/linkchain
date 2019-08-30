package xcrypto

import (
	"encoding/hex"
	"math/rand"
	"strings"
	"testing"
)

func TestFastHash(t *testing.T) {
	for i := 0; i < len(hashpair); i++ {
		data, _ := hex.DecodeString(hashpair[i][1])
		hash := FastHash(data)
		if !strings.EqualFold(hex.EncodeToString(hash[:]), hashpair[i][0]) {
			t.Fatalf("FastHash fail:message=%s wanted=%s, got=%s", hashpair[i][1], hashpair[i][0], hex.EncodeToString(hash[:]))
			return
		}
	}
}

func randomByte(size int) []byte {
	ret := make([]byte, size)
	for i := 0; i < size; i++ {
		ret[i] = (byte)(rand.Intn(256))
	}
	return ret
}
func BenchmarkFastHashSizeOf32(b *testing.B) {
	message := randomByte(32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FastHash(message)
	}
}
func BenchmarkFastHashSizeOf16384(b *testing.B) {
	message := randomByte(16384)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FastHash(message)
	}
}
