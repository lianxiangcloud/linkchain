package xcrypto

import (
	"bytes"
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

func TestCCRctPublicKeyV(t *testing.T) {
	nums := 4
	pkIns := make([]types.PublicKey, nums)

	r := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < nums; i++ {
		r.Read(pkIns[i][:])
	}
	for i := 0; i < nums; i++ {
		t.Logf("in[%d] = %s", i, hex.EncodeToString(pkIns[i][:]))
	}

	pkOuts := ccRctPublicKeyV(pkIns)
	for i := 0; i < nums; i++ {
		if !bytes.Equal(pkIns[i][:], pkOuts[i][:]) {
			t.Fatalf("ccRctPublicKeyV fail: index=%d, in=%s, out=%s", i, hex.EncodeToString(pkIns[i][:]), hex.EncodeToString(pkOuts[i][:]))
		} else {
			t.Logf("ccRctPublicKeyV ok: index=%d, in=%s, out=%s", i, hex.EncodeToString(pkIns[i][:]), hex.EncodeToString(pkOuts[i][:]))
		}
	}
}

func TestCCSignature(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	sigInt := types.Signature{}
	r.Read(sigInt.C[:])
	r.Read(sigInt.R[:])

	sigOut := ccSignature(sigInt)
	if !bytes.Equal(sigInt.C[:], sigOut.C[:]) {
		t.Fatalf("ccSignature C fail: in=%s, out=%s", hex.EncodeToString(sigInt.C[:]), hex.EncodeToString(sigInt.C[:]))
	} else {
		t.Logf("ccSignature C ok: in=%s, out=%s", hex.EncodeToString(sigInt.C[:]), hex.EncodeToString(sigInt.C[:]))
	}

	if !bytes.Equal(sigInt.R[:], sigOut.R[:]) {
		t.Fatalf("ccSignature R fail: in=%s, out=%s", hex.EncodeToString(sigInt.R[:]), hex.EncodeToString(sigInt.R[:]))
	} else {
		t.Logf("ccSignature R ok: in=%s, out=%s", hex.EncodeToString(sigInt.R[:]), hex.EncodeToString(sigInt.R[:]))
	}
}

// func TestGenRct(t *testing.T) {
// 	// @Todo:
// 	ctx := GenRctContext{
// 		KLRki:     &types.MultisigKLRki{},
// 		Msout:     &types.MultisigOut{},
// 		RctConfig: &types.RctConfig{},
// 	}

// 	GenRct(&ctx)
// }

// just for test
func makeRctKey(nums int) types.Key {
	var in types.Key
	r := rand.New(rand.NewSource(time.Now().Unix()))
	r.Read(in[:])
	return in
}

func makeRctKeyV(nums int) types.KeyV {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	var in types.KeyV
	for i := 0; i < nums; i++ {
		var tmp types.Key
		r.Read(tmp[:])
		in = append(in, tmp)
	}
	return in
}

func makeRctKeyM(nums int) types.KeyM {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	var in types.KeyM
	for i := 0; i < nums; i++ {
		var tmpv types.KeyV
		for j := 0; j < nums; j++ {
			var tmp types.Key
			r.Read(tmp[:])
			tmpv = append(tmpv, tmp)
		}
		in = append(in, tmpv)
	}
	return in
}

func makeRctCtKeyV(nums int) types.CtkeyV {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	var in types.CtkeyV
	for i := 0; i < nums; i++ {
		var tmp types.Ctkey
		r.Read(tmp.Dest[:])
		r.Read(tmp.Mask[:])
		in = append(in, tmp)
	}
	return in
}

func makeRctCtKeyM(nums int) types.CtkeyM {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	var in types.CtkeyM
	for i := 0; i < nums; i++ {
		var tmpv types.CtkeyV
		for j := 0; j < nums; j++ {
			var tmp types.Ctkey
			r.Read(tmp.Dest[:])
			r.Read(tmp.Mask[:])
			tmpv = append(tmpv, tmp)
		}
		in = append(in, tmpv)
	}
	return in
}

func makeRctEcdhTupleV(nums int) []types.EcdhTuple {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var in []types.EcdhTuple
	for i := 0; i < nums; i++ {
		var tmp types.EcdhTuple
		r.Read(tmp.Mask[:])
		r.Read(tmp.Amount[:])
		// r.Read(tmp.SenderPK[:])
		in = append(in, tmp)
	}
	return in
}

func makeRctKey64(nums int) types.Key64 {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	var in types.Key64
	for i := 0; i < 64; i++ {
		r.Read(in[i][:])
	}
	return in
}

func makeRctRangeSigV(nums int) []types.RangeSig {
	var in []types.RangeSig
	for i := 0; i < nums; i++ {
		var tmp types.RangeSig
		tmp.Asig.Ee = makeRctKey(nums)
		tmp.Asig.S0 = makeRctKey64(nums)
		tmp.Asig.S1 = makeRctKey64(nums)
		tmp.Ci = makeRctKey64(nums)
		in = append(in, tmp)
	}
	return in
}

func makeRctBulletproofV(nums int) []types.Bulletproof {
	var in []types.Bulletproof
	for i := 0; i < nums; i++ {
		var tmp types.Bulletproof
		tmp.L = makeRctKeyV(nums)
		tmp.A = makeRctKey(nums)
		tmp.S = makeRctKey(nums)
		tmp.T1 = makeRctKey(nums)
		tmp.T2 = makeRctKey(nums)
		tmp.Taux = makeRctKey(nums)
		tmp.Mu = makeRctKey(nums)
		tmp.L = makeRctKeyV(nums)
		tmp.R = makeRctKeyV(nums)

		tmp.Aa = makeRctKey(nums)
		tmp.B = makeRctKey(nums)
		tmp.T = makeRctKey(nums)

		in = append(in, tmp)
	}
	return in
}

func makeRctMgSigV(nums int) []types.MgSig {
	var in []types.MgSig
	for i := 0; i < nums; i++ {
		var tmp types.MgSig
		tmp.Cc = makeRctKey(nums)
		tmp.II = makeRctKeyV(nums)
		tmp.Ss = makeRctKeyM(nums)
		in = append(in, tmp)
	}
	return in
}

func checkRctKey(t *testing.T, in, out types.Key) {
	if !bytes.Equal(in[:], out[:]) {
		t.Fatalf("checkRctKey fail: in=%s, out=%s", hex.EncodeToString(in[:]), hex.EncodeToString(out[:]))
	} else {
		t.Logf("checkRctKey ok: in=%s, out=%s", hex.EncodeToString(in[:]), hex.EncodeToString(out[:]))
	}
}

func checkRctKeyV(t *testing.T, in, out types.KeyV) {
	for i := 0; i < len(in); i++ {
		if !bytes.Equal(in[i][:], out[i][:]) {
			t.Fatalf("ccRctKeyV fail: index=%d, in=%s, out=%s", i, hex.EncodeToString(in[i][:]), hex.EncodeToString(out[i][:]))
		} else {
			t.Logf("ccRctKeyV ok: index=%d, in=%s, out=%s", i, hex.EncodeToString(in[i][:]), hex.EncodeToString(out[i][:]))
		}
	}
}

func checkRctKeyM(t *testing.T, in, out types.KeyM) {
	for i := 0; i < len(in); i++ {
		for j := 0; j < len(in[i]); j++ {
			if !bytes.Equal(in[i][j][:], out[i][j][:]) {
				t.Fatalf("ccRctKeyM fail: i=%d, j=%d, in=%s, out=%s", i, j, hex.EncodeToString(in[i][j][:]), hex.EncodeToString(out[i][j][:]))
			} else {
				t.Logf("ccRctKeyM ok: i=%d, j=%d, in=%s, out=%s", i, j, hex.EncodeToString(in[i][j][:]), hex.EncodeToString(out[i][j][:]))
			}
		}
	}
}

func checkRctCtKeyV(t *testing.T, in, out types.CtkeyV) {
	for i := 0; i < len(in); i++ {
		if !bytes.Equal(in[i].Dest[:], out[i].Dest[:]) {
			t.Fatalf("ccRctCtKeyV Dest fail: index=%d, in=%s, out=%s", i, hex.EncodeToString(in[i].Dest[:]), hex.EncodeToString(out[i].Dest[:]))
		} else {
			t.Logf("ccRctCtKey Dest ok: index=%d, in=%s, out=%s", i, hex.EncodeToString(in[i].Dest[:]), hex.EncodeToString(out[i].Dest[:]))
		}

		if !bytes.Equal(in[i].Mask[:], out[i].Mask[:]) {
			t.Fatalf("ccRctCtKey Mask fail: index=%d, in=%s, out=%s", i, hex.EncodeToString(in[i].Mask[:]), hex.EncodeToString(out[i].Mask[:]))
		} else {
			t.Logf("ccRctCtKey Mask ok: index=%d, in=%s, out=%s", i, hex.EncodeToString(in[i].Mask[:]), hex.EncodeToString(out[i].Mask[:]))
		}
	}
}

func checkRctCtKeyM(t *testing.T, in, out types.CtkeyM) {
	for i := 0; i < len(in); i++ {
		for j := 0; j < len(in[i]); j++ {
			if !bytes.Equal(in[i][j].Dest[:], out[i][j].Dest[:]) {
				t.Fatalf("ccRctCtKeyM Dest fail: i=%d, j=%d, in=%s, out=%s", i, j, hex.EncodeToString(in[i][j].Dest[:]), hex.EncodeToString(out[i][j].Dest[:]))
			} else {
				t.Logf("ccRctCtKeyM Dest ok: i=%d, j=%d, in=%s, out=%s", i, j, hex.EncodeToString(in[i][j].Dest[:]), hex.EncodeToString(out[i][j].Dest[:]))
			}

			if !bytes.Equal(in[i][j].Mask[:], out[i][j].Mask[:]) {
				t.Fatalf("ccRctCtKeyM Mask fail: i=%d, j=%d, in=%s, out=%s", i, j, hex.EncodeToString(in[i][j].Mask[:]), hex.EncodeToString(out[i][j].Mask[:]))
			} else {
				t.Logf("ccRctCtKeyM Mask ok: i=%d, j=%d, in=%s, out=%s", i, j, hex.EncodeToString(in[i][j].Mask[:]), hex.EncodeToString(out[i][j].Mask[:]))
			}
		}
	}
}

func checkEcdhTupleV(t *testing.T, in, out []types.EcdhTuple) {
	for i := 0; i < len(in); i++ {
		checkRctKey(t, in[i].Mask, out[i].Mask)
		checkRctKey(t, in[i].Amount, out[i].Amount)
		// checkRctKey(t, in[i].SenderPK, out[i].SenderPK)
	}
}

func checkRctSigbase(t *testing.T, in, out *types.RctSigBase) {
	if in.Type != out.Type {
		t.Fatalf("RctSigbase Type fail: in=%d, out=%d", in.Type, out.Type)
	}
	checkRctKey(t, in.Message, out.Message)
	checkRctCtKeyM(t, in.MixRing, out.MixRing)
	checkRctKeyV(t, in.PseudoOuts, out.PseudoOuts)
	checkEcdhTupleV(t, in.EcdhInfo, out.EcdhInfo)
	checkRctCtKeyV(t, in.OutPk, out.OutPk)
	if in.TxnFee != out.TxnFee {
		t.Fatalf("RctSigbase TxnFee fail: in=%d, out=%d", in.TxnFee, out.TxnFee)
	}
}

func checkRctKey64(t *testing.T, in, out types.Key64) {
	for i := 0; i < 64; i++ {
		checkRctKey(t, in[i], out[i])
	}
}

func checkRctRangeSig(t *testing.T, in, out *types.RangeSig) {
	// BoroSig
	checkRctKey64(t, in.Asig.S0, out.Asig.S0)
	checkRctKey64(t, in.Asig.S1, out.Asig.S1)
	checkRctKey(t, in.Asig.Ee, out.Asig.Ee)

	checkRctKey64(t, in.Ci, out.Ci)
}

func checkRctBulletproof(t *testing.T, in, out *types.Bulletproof) {
	checkRctKeyV(t, in.V, out.V)
	checkRctKey(t, in.A, out.A)
	checkRctKey(t, in.S, out.S)
	checkRctKey(t, in.T1, out.T1)
	checkRctKey(t, in.T2, out.T2)
	checkRctKey(t, in.Taux, out.Taux)
	checkRctKey(t, in.Mu, out.Mu)
	checkRctKeyV(t, in.L, out.L)
	checkRctKeyV(t, in.R, out.R)
	checkRctKey(t, in.Aa, out.Aa)
	checkRctKey(t, in.B, out.B)
	checkRctKey(t, in.T, out.T)
}

func checkRctMgSig(t *testing.T, in, out *types.MgSig) {
	checkRctKeyM(t, in.Ss, out.Ss)
	checkRctKey(t, in.Cc, out.Cc)
	checkRctKeyV(t, in.II, out.II)
}

func checkRctSigPrunable(t *testing.T, in, out *types.RctSigPrunable) {
	// rangesig
	for i := 0; i < len(in.RangeSigs); i++ {
		checkRctRangeSig(t, &in.RangeSigs[i], &out.RangeSigs[i])
	}
	for i := 0; i < len(in.Bulletproofs); i++ {
		checkRctBulletproof(t, &in.Bulletproofs[i], &out.Bulletproofs[i])
	}
	for i := 0; i < len(in.MGs); i++ {
		checkRctMgSig(t, &in.MGs[i], &out.MGs[i])
	}
	checkRctKeyV(t, in.PseudoOuts, out.PseudoOuts)
}

func checkRctSig(t *testing.T, in, out *types.RctSig) {
	checkRctSigbase(t, &in.RctSigBase, &out.RctSigBase)
	checkRctSigPrunable(t, &in.P, &out.P)
}

func TestCCRctKey(t *testing.T) {
	in := makeRctKey(0)
	t.Logf("in: %s", hex.EncodeToString(in[:]))

	out := ccRctKey(in)
	checkRctKey(t, in, out)
}

func TestCCRctKeyV(t *testing.T) {
	nums := 16
	in := makeRctKeyV(nums)

	out := ccRctKeyV(in)
	checkRctKeyV(t, in, out)
}

func TestCCRctKeyM(t *testing.T) {
	nums := 16
	in := makeRctKeyM(nums)

	out := ccRctKeyM(in)
	checkRctKeyM(t, in, out)
}

func TestCCRctCtKeyV(t *testing.T) {
	nums := 16
	in := makeRctCtKeyV(nums)

	out := ccRctCtKeyV(in)
	checkRctCtKeyV(t, in, out)
}

func TestCCRctCtKeyM(t *testing.T) {
	nums := 16
	in := makeRctCtKeyM(nums)

	out := ccRctCtKeyM(in)
	checkRctCtKeyM(t, in, out)
}

func TestCCRctSigbase(t *testing.T) {
	nums := 16

	in := &types.RctSigBase{
		Type:       0x10,
		Message:    makeRctKey(nums),
		MixRing:    makeRctCtKeyM(nums),
		PseudoOuts: makeRctKeyV(nums),
		EcdhInfo:   makeRctEcdhTupleV(nums),
		OutPk:      makeRctCtKeyV(nums),
		TxnFee:     0x1000,
	}

	out := ccRctSigbase(in)
	checkRctSigbase(t, in, out)
}

func TestCCRctSig(t *testing.T) {
	nums := 16

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

	out := ccRctSig(in)
	checkRctSig(t, in, out)
}

func BenchmarkCCRctSigbase(b *testing.B) {
	nums := 12

	in := &types.RctSigBase{
		Type:       0x10,
		Message:    makeRctKey(nums),
		MixRing:    makeRctCtKeyM(nums),
		PseudoOuts: makeRctKeyV(nums),
		EcdhInfo:   makeRctEcdhTupleV(nums),
		OutPk:      makeRctCtKeyV(nums),
		TxnFee:     0x1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := ccRctSigbase(in)
		if out == nil {
			b.Fatalf("ccRctSigbase fail: i=%d", i)
		}
	}
}

func BenchmarkCCRctSig(b *testing.B) {
	nums := 12

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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := ccRctSig(in)
		if out == nil {
			b.Fatalf("ccRctSigfail: i=%d", i)
		}
	}
}
