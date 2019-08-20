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

func TestSlowHash(t *testing.T) {
	data := make([]byte, 100)
	for i := 0; i < 100; i++ {
		data[i] = byte(i)
	}

	wanted := "efc274dee601318882cccbe4c02103162d9fc8281dc608be0da513fa3916504e"
	hash := SlowHash(data, 0, 100)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), wanted) {
		t.Fatalf("SlowHash fail: wanted=%s, got=%s", wanted, hex.EncodeToString(hash[:]))
	}
}
func randomByte(size int)[]byte  {
	ret := make([]byte,size)
	for i:=0;i < size;i++{
		ret[i]= (byte)(rand.Intn(256))
	}
	return ret
}
func BenchmarkFastHashSizeOf32(b *testing.B){
	message := randomByte(32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FastHash(message)
	}
}
func BenchmarkFastHashSizeOf16384(b *testing.B){
	message := randomByte(16384)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FastHash(message)
	}
}
func  BenchmarkSlowHashVariant0(b *testing.B) {
	message,_ := hex.DecodeString("63617665617420656d70746f763617665617420656ffffffd70746fffffff72263617665617420656d70746f7201020304")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SlowHash(message,0,0)
	}

}
func  BenchmarkSlowHashVariant1(b *testing.B) {
	message,_ := hex.DecodeString("63617665617420656d70746f763617665617420656ffffffd70746fffffff72263617665617420656d70746f7201020304")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SlowHash(message,1,0)
	}

}
func  BenchmarkSlowHashVariant2(b *testing.B) {
	message,_ := hex.DecodeString("63617665617420656d70746f763617665617420656ffffffd70746fffffff72263617665617420656d70746f7201020304")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SlowHash(message,2,0)
	}

}
func  BenchmarkSlowHashVariant4(b *testing.B) {
	message,_ := hex.DecodeString("63617665617420656d70746f763617665617420656ffffffd70746fffffff72263617665617420656d70746f7201020304")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SlowHash(message,4,0)
	}
}
/*
func TestSlowHashPreHashed(t *testing.T) {
	data := make([]byte, 100)
	for i := 0; i < 100; i++ {
		data[i] = byte(i)
	}

	wanted := "e14dfa9ee3f498c371b84466444e36007a984485d49d84d0ab2c65d9b461df7d"
	hash := SlowHashPreHashed(data, 0, 100)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), wanted) {
		t.Fatalf("SlowHashPreHashed fail: wanted=%s, got=%s", wanted, hex.EncodeToString(hash[:]))
	}
}
*/

func TestSlowHashState(t *testing.T) {
	data := make([]byte, 100)
	for i := 0; i < 100; i++ {
		data[i] = byte(i)
	}

	state := SlowHashState{}
	state.Allocate()
	defer state.Free()

	wanted := "efc274dee601318882cccbe4c02103162d9fc8281dc608be0da513fa3916504e"
	hash := state.Hash(data, 0, 100)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), wanted) {
		t.Fatalf("SlowHashState fail: wanted=%s, got=%s", wanted, hex.EncodeToString(hash[:]))
	}

	wanted = "745ed2edd8476a55caa6d287228f103dc68986b938c1c911eea07a12f0667fae"
	hash = state.PreHashed(data, 0, 100)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), wanted) {
		t.Fatalf("SlowHashState fail: wanted=%s, got=%s", wanted, hex.EncodeToString(hash[:]))
	}

}



// func TestStoreTToBinary(t *testing.T) {
// 	StoreTToBinary()
// }

//func TestStoreTToBinaryGetBlocks(t *testing.T) {
//	req := types.RPCGetBlocksFastRequest{}
//	req.StartHeight = 1000
//	req.Prune = false
//	req.NoMinerTx = false
//
//	req.BlockIDs = append(req.BlockIDs, string(types.BytesToHash([]byte("b69a75afb6d7798c88ee3ecedd12ccd38395cdba1e2889bb75704996f14b3091"))[:]))
//	req.BlockIDs = append(req.BlockIDs, string(types.BytesToHash([]byte("f28646b8ffd004fe405db1f304f3174c8bda9f1b8cbd1f87edd0c3ee1fc59cdb"))[:]))
//	var reqParam string
//	StoreTToBinaryGetBlocks(&req, &reqParam)
//
//	fmt.Println("ret---->", reqParam, "<--------")
//}
