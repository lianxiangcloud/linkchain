package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
)

func TestLenTo2Byte(t *testing.T) {
	data := make([]byte, 10)
	for i := (1 << 16) - 1; i >= 0; i-- {
		LenTo2Byte(i, data[LENOFFSET:VOFFSET])
		low := int(data[2])
		hight := int(data[3]) << 8
		if i != (low | hight) {
			t.Fatalf("LenTo2Byte fail: wanted=%v, got=%v %v %v", i, hex.EncodeToString(data[:]), low, hight)
			return
		}
		//	fmt.Printf("len=%d byte=%v\n",i,hex.EncodeToString(data[:]))
	}
}

func TestKeyVDEncode(t *testing.T) {
	key1 := Key{}
	key2 := Key{}

	for i := 0; i < 32; i++ {
		key1[i] = byte(i)
	}
	for i := 0; i < 32; i++ {
		key2[i] = byte((1 << 8) - 1 - i)
	}
	kv := KeyV{
		key1, key2,
	}

	data := make([]byte, kv.TlvSize());
	if _, err := kv.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}
	//	fmt.Printf("encode=%v\n", data)
	decodekv := KeyV{}
	if err := decodekv.TlvDecode(data); err != nil {
		t.Fatal(err.Error())
	}
	if len(decodekv) != 2 {
		t.Fatalf("TlvDecode fail expected size=2 got=%v", len(decodekv))
	}
	//	fmt.Printf("decodekv=%v\n",decodekv)
	for i := 0; i < 32; i++ {
		if decodekv[0][i] != byte(i) {
			t.Fatalf("expected key1=%v got=%v", key1, decodekv[0])
		}
	}
	for i := 0; i < 32; i++ {
		if decodekv[1][i] != byte((1<<8)-1-i) {
			t.Fatalf("expected key2=%v got=%v", key2, decodekv[1])
		}
	}
}
func getMockKey(st int) (*Key) {
	ret := &Key{}
	for i := 0; i < 32; i++ {
		ret[i] = byte(st + i)
	}
	return ret
}
func TestKeyMDEncode(t *testing.T) {

	km := KeyM{}
	km = append(km, KeyV{})
	km = append(km, KeyV{})
	km = append(km, KeyV{})
	km[0] = append(km[0], *getMockKey(32 * 0))
	km[0] = append(km[0], *getMockKey(32 * 1))
	km[0] = append(km[0], *getMockKey(32 * 2))

	km[1] = append(km[1], *getMockKey(32 * 3))
	km[1] = append(km[1], *getMockKey(32 * 4))

	km[2] = append(km[2], *getMockKey(32 * 5))
	km[2] = append(km[2], *getMockKey(32 * 6))
	km[2] = append(km[2], *getMockKey(32 * 7))

	data := make([]byte, km.TlvSize());
	if _, err := km.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}
	//	fmt.Printf("encode=%v\n", data)
	decodekM := KeyM{}
	if err := decodekM.TlvDecode(data); err != nil {
		t.Fatal(err.Error())
	}
	if len(decodekM) != 3 {
		t.Fatalf("TestKeyMDEncode fail expected size=3 got=%v", len(decodekM))
	}
	flag := byte(0)
	for i := 0; i < len(km); i++ {
		for j := 0; j < len(km[i]); j++ {
			for kk := 0; kk < len(km[i][j]); kk++ {
				if decodekM[i][j][kk] != flag {
					t.Fatalf("TestKeyMDEncode fail expected flag=%v got=%v", flag, km[i][j])
				}
				flag++
			}
		}
	}

}
func TestKey64DEncode(t *testing.T) {
	k64 := Key64{}
	for i := 0; i < 64; i++ {
		k64[i] = *getMockKey(i)
	}
	data := make([]byte, k64.TlvSize());
	if _, err := k64.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}
	//	fmt.Printf("encode=%v\n", data)
	decode := Key64{}
	if err := decode.TlvDecode(data); err != nil {
		t.Fatal(err.Error())
	}
	for i := 0; i < 64; i++ {
		for j := 0; j < 32; j++ {
			if decode[i][j] != byte(i+j) {
				t.Fatalf("TestKey64DEncode fail expected byte=%v got=%v", i+j, decode[i])
			}
		}
	}
}

func TestCtkeyDEncode(t *testing.T) {
	ctkey := &Ctkey{
		Key{191, 106, 61, 16, 27, 205, 154, 179, 20, 74, 153, 245, 60, 147, 226, 119, 91, 87, 96, 247, 21, 79, 251, 241, 188, 61, 206, 122, 110, 222, 72, 99,}, Key{224, 99, 57, 131, 42, 137, 71, 62, 118, 246, 63, 141, 83, 195, 40, 70, 234, 7, 145, 208, 182, 171, 44, 28, 231, 35, 153, 213, 163, 50, 207, 226},
	}
	data := make([]byte, ctkey.TlvSize())
	if _, err := ctkey.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}
	to := &Ctkey{}
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !ctkey.IsEqual(to) {
		t.Fatalf("TestCtkeyDEncode fail expected byte=%v got=%v", ctkey, to)
	}
}

func TestCtkeyVDEncode(t *testing.T) {
	src := mixringForTest[0]
	//src:= CtkeyV{Ctkey{Dest:Key{1,2,}},Ctkey{Dest:Key{3,4,}}}
	data := make([]byte, src.TlvSize())
//	fmt.Printf("datalen=%v\n", len(data))
	if _, err := src.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}

	to := &CtkeyV{}
	//fmt.Printf("data=%v\n",data)
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !src.IsEqual(to) {
		t.Fatalf("TestCtkeyVDEncode fail expected byte=%v got=%v", src, to)
	}
}
func TestCtkeyMDEncode(t *testing.T) {
	src := mixringForTest
	data := make([]byte, src.TlvSize())
	//fmt.Printf("datalen=%v\n", len(data))
	if _, err := src.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}

	to := &CtkeyM{}
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !src.IsEqual(to) {
		t.Fatalf("TestCtkeyMDEncode fail expected byte=%v got=%v", src, to)
	}
}
func TestMultisigKLRkiDEncode(t *testing.T) {
	src := &MultisigKLRki{
		K:  sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1600"),
		Ki: sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1601"),
		L:  sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1602"),
		R:  sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1603"),
	}
	data := make([]byte, src.TlvSize())
//	fmt.Printf("datalen=%v\n", len(data))
	if _, err := src.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}

	to := &MultisigKLRki{}
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !IsEqualMultisigKLRkiForTest(src, to) {
		t.Fatalf("TestMultisigKLRkiDEncode fail expected byte=%v got=%v", src, to)
	}
}
func TestMultisigOutDEncode(t *testing.T) {
	src := &MultisigOut{
		C: KeyV{
			sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1600"),
			sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1601"),
			sToKey("0x13bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1602"),
			sToKey("0x23bca9af218a7b338f545ce41207a569bc2782a287579c540bc5512fa1df1602"),
		},
	}
	data := make([]byte, src.TlvSize())
	//fmt.Printf("datalen=%v\n", len(data))
	if _, err := src.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}

	to := &MultisigOut{}
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !src.C.IsEqual(&(to.C)) {
		t.Fatalf("TestMultisigOutDEncode fail expected byte=%v got=%v", src, to)
	}
}



func TestRctConfigDEncode(t *testing.T) {
	src := &RctConfig{
		BpVersion:1,
		RangeProofType:RCTTypeBulletproof2,
	}
	data := make([]byte, src.TlvSize())
//	fmt.Printf("datalen=%v\n", len(data))
	if _, err := src.TlvEncode(data); err != nil {
		t.Fatal(err.Error())
	}
//	fmt.Printf("encode finish=%v\n", data)
	to := &RctConfig{}
	err := to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if src.BpVersion != to.BpVersion || src.RangeProofType != src.RangeProofType {
		t.Fatalf("TestRctConfigDEncode fail expected byte=%v got=%v", src, to)
	}
}

func TestRctSigDEncode(t *testing.T) {
	src := defualtTestRctsig()
	src.PseudoOuts=KeyV{}
	data := make([]byte, src.TlvSize())
	//fmt.Printf("datalen=%v\n", len(data))
	_, err := src.TlvEncode(data);
	//fmt.Printf("encodeLen=%v\n", encodeLen)
	if err != nil {
		t.Fatal(err.Error())
	}
//	fmt.Printf("encode=%v\n", hex.EncodeToString(data))
	to := &RctSig{}
	err = to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !IsEqualRctSigForTest(src,to) {
		t.Fatalf("TestRctConfigDEncode fail")
	}
}
func BenchmarkRctSigEncode(b *testing.B){
	src := defualtTestRctsig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := make([]byte, src.TlvSize())
		if _, err := src.TlvEncode(data); err != nil {
			b.Fatalf("BenchmarkRctSigEncode fail %v",err.Error());
		}
	}
}
func BenchmarkRctSigJsonEncode(b *testing.B){
	src := defualtTestRctsig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ ,err := json.Marshal(src)
		if err != nil {
			b.Fatalf("BenchmarkRctSigJsonEncode fail %v",err.Error());
		}
	}
}
func BenchmarkRctSigDecode(b *testing.B){
	src := defualtTestRctsig()
	data := make([]byte, src.TlvSize())
	if _, err := src.TlvEncode(data); err != nil {
		b.Fatalf("BenchmarkRctSigDecode fail %v",err.Error());
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		to := &RctSig{}
		err := to.TlvDecode(data)
		if err != nil {
			b.Fatalf("BenchmarkRctSigDecode fail %v",err.Error());
		}
	}
}
func BenchmarkRctSigJsonDecode(b *testing.B){
	src := defualtTestRctsig()
	data ,err := json.Marshal(src)
	if  err != nil {
		b.Fatalf("BenchmarkRctSigDecode fail %v",err.Error());
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		to := &RctSig{}
		err := json.Unmarshal(data,to)
		if err != nil {
			b.Fatalf("BenchmarkRctSigJsonDecode fail %v",err.Error());
		}
	}
}
func IsEqualRctSigBaseForTest(a, b *RctSigBase)bool  {
	return false
}

func IsEqualRctSigForTest(a, b *RctSig)bool  {
	ajson ,err := json.Marshal(a)
	if err != nil{
		fmt.Printf("%v\n",err)
		return false
	}
	bjson,err := json.Marshal(b)
	if err != nil{
		fmt.Printf("%v\n",err)
		return false
	}
	if bytes.Equal(ajson,bjson){
	//	fmt.Printf("ajson=%v\n",string(ajson))
	//	fmt.Printf("bjson=%v\n",string(ajson))
		return true
	}
	fmt.Printf("ajson=%v\n",string(ajson))
	fmt.Printf("bjson=%v\n",string(bjson))
	return false
}


func IsEqualJsonForTest(a, b interface{})bool  {
	ajson ,err := json.Marshal(a)
	if err != nil{
		fmt.Printf("%v\n",err)
		return false
	}
	bjson,err := json.Marshal(b)
	if err != nil{
		fmt.Printf("%v\n",err)
		return false
	}
	if bytes.Equal(ajson,bjson){
			//fmt.Printf("ajson=%v\n",string(ajson))
			//fmt.Printf("bjson=%v\n",string(bjson))
		return true
	}
	//fmt.Printf("ajson=%v\n",string(ajson))
	//fmt.Printf("bjson=%v\n",string(bjson))
	return false
}

func IsEqualMultisigKLRkiForTest(a, b *MultisigKLRki) bool {
	if (!a.K.IsEqual(&(b.K))) || (!a.Ki.IsEqual(&(b.Ki))) || (!a.L.IsEqual(&(b.L))) || (!a.R.IsEqual(&(b.R))) {
		return false
	}
	return true
}
