package xcrypto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

func TestTlvKeyVTest(t *testing.T) {
	err := TlvKeyVTest(2)
	if err != nil {
		t.Fatal(err)
	}
}
func BenchmarkTlvKeyVTest(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := TlvKeyVTest(100)
		if err != nil {
			b.Fatal(err)
		}
	}
}
func TestTlvRctV(t *testing.T) {
	rctsign := DefualtTestRctsig()
	rctsign.PseudoOuts = types.KeyV{}
	err := TlvRctSign(rctsign)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestTlvRctSignCgo(t *testing.T) {
	from := DefualtTestRctsig()
	to, err := TlvRctsigForTest(from)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !IsEqualRctSigForTest(from, to) {
		t.Fatalf("TlvRctSignCgoTest fail")
	}
}
func TestTlvGetSubaddressCgo(t *testing.T) {
	kk := types.SecretKey{}
	kk[0] = 0x0f
	spendSK, spendPK := GenerateKeys(kk)
	viewSK, viewPK := GenerateKeys(spendSK)
	acc := types.AccountKey{
		Addr: types.AccountAddress{
			SpendPublicKey: spendPK,
			ViewPublicKey:  viewPK,
		},
		SpendSKey: spendSK,
		ViewSKey:  viewSK,
		SubIdx:    uint64(10),
	}
	addr, err := TlvGetSubaddress(&acc, 1)
	if err != nil {
		t.Fatal(err.Error())
	}
	fmt.Printf("%v %v\n", addr.ViewPublicKey, addr.SpendPublicKey)
}

func BenchmarkTlvRctSignDefaultCgo(b *testing.B) {
	from := DefualtTestRctsig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := TlvRctsigForTest(from)
		if err != nil {
			b.Fatal(err.Error())
		}
	}
}
func TestRctSigDEncode(t *testing.T) {
	nums := 5
	src := &types.RctSig{
		RctSigBase: types.RctSigBase{
			Type:       0x10,
			Message:    makeRctKey(nums),
			MixRing:    makeRctCtKeyM(nums),
			PseudoOuts: makeRctKeyV(nums),
			EcdhInfo:   makeRctEcdhTupleV(nums),
			OutPk:      makeRctCtKeyV(nums),
			TxnFee:     0x1000,
		},
		P: types.RctSigPrunable{
			RangeSigs:    makeRctRangeSigV(nums),
			Bulletproofs: makeRctBulletproofV(nums),
			MGs:          makeRctMgSigV(nums),
			PseudoOuts:   makeRctKeyV(nums),
		},
	}
	data := make([]byte, src.TlvSize())
	//	fmt.Printf("datalen=%v\n", len(data))
	_, err := src.TlvEncode(data);
	//	fmt.Printf("encodeLen=%v\n", encodeLen)
	if err != nil {
		t.Fatal(err.Error())
	}
	//	fmt.Printf("encode=%v\n", hex.EncodeToString(data))
	to := &types.RctSig{}
	err = to.TlvDecode(data)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !IsEqualRctSigForTest(src, to) {
		t.Fatalf("TestRctConfigDEncode fail")
	}
}
func BenchmarkTlvRctSignCgo(b *testing.B) {
	nums := 5
	in := &types.RctSig{
		RctSigBase: types.RctSigBase{
			Type:       0x10,
			Message:    makeRctKey(nums),
			MixRing:    makeRctCtKeyM(nums),
			PseudoOuts: makeRctKeyV(nums),
			EcdhInfo:   makeRctEcdhTupleV(nums),
			OutPk:      makeRctCtKeyV(nums),
			TxnFee:     0x1000,
		},
		P: types.RctSigPrunable{
			RangeSigs:    makeRctRangeSigV(nums),
			Bulletproofs: makeRctBulletproofV(nums),
			MGs:          makeRctMgSigV(nums),
			PseudoOuts:   makeRctKeyV(nums),
		},
	}
	fmt.Printf("size=%v\n", in.TlvSize())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := TlvRctsigForTest(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}
func IsEqualRctSigForTest(a, b *types.RctSig) bool {
	ajson, err := json.Marshal(a)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}
	bjson, err := json.Marshal(b)
	if err != nil {
		fmt.Printf("%v\n", err)
		return false
	}
	if bytes.Equal(ajson, bjson) {
		//	fmt.Printf("ajson=%v\n",string(ajson))
		//	fmt.Printf("bjson=%v\n",string(ajson))
		return true
	}
	//fmt.Printf("ajson=%v\n", string(ajson))
	//fmt.Printf("bjson=%v\n", string(bjson))
	return false
}
