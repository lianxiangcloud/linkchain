package ringct

import (
	"encoding/hex"
	"fmt"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/crypto"
	"testing"
)

var (
	benchK1 = sToKey("9cef8a2152d4109aeb4268fcb6cab98834c628c878271a086609e2716cea5c0a")
	benchK2 = sToKey("a570b7ba4137be56437cd7f3e1b79e2783e7e9f2f5313e7b20f705e14dbf4808")
	benchK3 = sToKey("a570b7ba4137be56437cd7f3e1b79e2783e7e9f2f5313e7b20f705e14dbf4808")
	scalar1 = types.EcScalar{225, 110, 138, 107, 13, 197, 103, 87, 227, 39, 246, 209, 177, 246, 188, 224, 71, 194, 37, 126, 41, 37, 24, 118, 173, 217, 155, 137, 86, 198, 191, 32}
	scalar2 = types.EcScalar{22, 221, 175, 246, 221, 238, 44, 238, 240, 191, 7, 253, 181, 179, 131, 38, 197, 207, 195, 121, 166, 151, 159, 41, 85, 136, 212, 159, 151, 124, 55, 40}

	benchAmount types.Lk_amount = 0xfffffff123
)

func TestScalarmultKey(t *testing.T) {
	p := types.Key{225, 110, 138, 107, 13, 197, 103, 87, 227, 39, 246, 209, 177, 246, 188, 224, 71, 194, 37, 126, 41, 37, 24, 118, 173, 217, 155, 137, 86, 198, 191, 32}
	a := types.Key{22, 221, 175, 246, 221, 238, 44, 238, 240, 191, 7, 253, 181, 179, 131, 38, 197, 207, 195, 121, 166, 151, 159, 41, 85, 136, 212, 159, 151, 124, 55, 40}
	key, err := ScalarmultKey(p, a)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0x31d8a45b3089d53fdc99374b16327adf35abd30d6a9e1917d0993585cd0925e2")
	if !key.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, key)
	}
}

func TestScalarmultBase(t *testing.T) {
	a := types.Key{22, 221, 175, 246, 221, 238, 44, 238, 240, 191, 7, 253, 181, 179, 131, 38, 197, 207, 195, 121, 166, 151, 159, 41, 85, 136, 212, 159, 151, 124, 55, 40}
	key := ScalarmultBase(a)
	expect := sToKey("0xe7bd2745841be66dbb6a21cea6450d08d922da78f0697429021f94f43af94aeb")
	if !key.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, key)
	}
}

func TestSkpkGen(t *testing.T) {
	_, pk := SkpkGen()
	publickKey:=crypto.KeyToPublicKey(pk)
	if !crypto.CheckKey(&publickKey){
		t.Fatalf("publickKey is invalid %v", pk)
	}
	//fmt.Printf("skey=%v pkey=%v\n", hexEncode(sk[:]), hexEncode(pk[:]))
}

func TestZeroCommit(t *testing.T) {
	ret, err := ZeroCommit(1)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0x1738eb7a677c6149228a2beaa21bea9e3370802d72a3eec790119580e02bd522")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}

func TestScalarmult8(t *testing.T) {
	p := Z
	p[0] = 0x01
	ret, err := Scalarmult8(p)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0x0100000000000000000000000000000000000000000000000000000000000000")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}
func TestScAdd(t *testing.T) {
	a := types.EcScalar{225, 110, 138, 107, 13, 197, 103, 87, 227, 39, 246, 209, 177, 246, 188, 224, 71, 194, 37, 126, 41, 37, 24, 118, 173, 217, 155, 137, 86, 198, 191, 32}
	b := types.EcScalar{22, 221, 175, 246, 221, 238, 44, 238, 240, 191, 7, 253, 181, 179, 131, 38, 197, 207, 195, 121, 166, 151, 159, 41, 85, 136, 212, 159, 151, 124, 55, 40}
	ret := ScAdd(a, b)
	expect := sToKey("0x43fc62ee81274be57a741f43edc2c4b30c92e9f7cfbcb79f02627029ee42f708")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}

func TestScSub(t *testing.T) {
	a := types.EcScalar{225, 110, 138, 107, 13, 197, 103, 87, 227, 39, 246, 209, 177, 246, 188, 224, 71, 194, 37, 126, 41, 37, 24, 118, 173, 217, 155, 137, 86, 198, 191, 32}
	b := types.EcScalar{22, 221, 175, 246, 221, 238, 44, 238, 240, 191, 7, 253, 181, 179, 131, 38, 197, 207, 195, 121, 166, 151, 159, 41, 85, 136, 212, 159, 151, 124, 55, 40}
	ret := ScSub(a, b)
	expect := sToKey("0xb865d0d149394dc1c804e677da3c18cf82f26104838d784c5851c7e9be498808")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}
func TestSkGen(t *testing.T) {
	ret := SkGen()
	fmt.Printf("TestSkGen=%v\n", hexEncode(ret[:]))
}
func TestGenC(t *testing.T) {
	a := types.Key{225, 110, 138, 107, 13, 197, 103, 87, 227, 39, 246, 209, 177, 246, 188, 224, 71, 194, 37, 126, 41, 37, 24, 118, 173, 217, 155, 137, 86, 198, 191, 32}
	ret, err := GenC(a, 1)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0xeb3145ab8945398f1584053d2e058e0fa8453cdba0475f05fe8554452b140cae")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}

func TestAddKeys(t *testing.T) {
	ret, err := AddKeys(benchK1, benchK2)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0x49b9937a9210c62b7e0f65f75de0815efb0e34a4ff0e75e3ef34ac2320419129")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}
func TestAddKeyV(t *testing.T) {
	kv := types.KeyV{}
	for i := 0; i < 4; i++ {
		kv = append(kv, benchK1)
		kv = append(kv, benchK2)
		kv = append(kv, benchK3)
	}
	ret, err := AddKeyV(kv)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0x7f55b51644abaf55d04c04e91b05e05eb80ae4b64892812bf0ad6ebf47c1b7f7")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}
func TestAddKeys2(t *testing.T) {
	ret, err := AddKeys2(benchK1, benchK2, benchK3)
	if err != nil {
		t.Fatalf(err.Error())
	}
	expect := sToKey("0x72a31e34d6002b8d8b0bb63b21df83e52a939bbf74591a809283c48639affc40")
	if !ret.IsEqual(&expect) {
		t.Fatalf("want %v ,got %v", expect, ret)
	}
}

func BenchmarkScalarmultBase(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScalarmultBase(benchK1)
	}
}
func BenchmarkScalarmultKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScalarmultKey(benchK1, benchK2)
	}
}
func BenchmarkSkpkGen(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SkpkGen()
	}
}
func BenchmarkScalarmultH(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScalarmultH(benchK1)
	}
}
func BenchmarkZeroCommit(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ZeroCommit(benchAmount)
	}
}
func BenchmarkScalarmult8(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Scalarmult8(benchK1)
	}
}
func BenchmarkScAdd(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScAdd(scalar1, scalar2)
	}
}
func BenchmarkScSub(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScSub(scalar1, scalar2)
	}
}
func BenchmarkSkGen(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SkGen()
	}
}

func BenchmarkGenC(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenC(benchK1, benchAmount)
	}
}

func BenchmarkAddKeys(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AddKeys(benchK1, benchK2)
	}
}
func BenchmarkAddKeyV(b *testing.B) {
	kv := types.KeyV{}
	for i := 0; i < 4; i++ {
		kv = append(kv, benchK1)
		kv = append(kv, benchK2)
		kv = append(kv, benchK3)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := AddKeyV(kv)
		if err != nil {
			b.Fatalf(err.Error())
		}
	}
}

func BenchmarkAddKeys2(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AddKeys2(benchK1, benchK2, benchK3)
	}
}

func hexEncode(input []byte) string {
	enc := make([]byte, len(input)*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], input)
	return string(enc)
}
