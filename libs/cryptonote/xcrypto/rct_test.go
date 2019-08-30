package xcrypto

import (
	"math/rand"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

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
