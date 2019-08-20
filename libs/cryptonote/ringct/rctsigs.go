package ringct

import (
	. "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/xcrypto"
)

//func GenRct(message Key, inSk CtkeyV, destinations KeyV,
//	amounts []Lk_amount, mixRing CtkeyM, amountKeys KeyV, kLRki *MultisigKLRki, msout MultisigOut,
//	index uint32, outSk CtkeyV, rctConfig *RctConfig) *RctSig {
//
//	return nil
//}
//func GenRctSimple(message Key, inSk CtkeyV, destinations KeyV,
//	inAmounts []Lk_amount, outAmounts []Lk_amount, fee Lk_amount, mixRing CtkeyM, amountKeys KeyV, kLRki *MultisigKLRki, msout *MultisigOut,
//	index []uint32, rctConfig *RctConfig) (*RctSig, *CtkeyV) {
//	return nil, nil
//}
func VerRct(rs *RctSig) bool {
	return xcrypto.VerRct(rs)
}
func VerRctSimple(rs *RctSig) bool {
	return xcrypto.VerRctSimple(rs)
}

func VerRctNonSemanticsSimple(rs *RctSig) bool {
	// return xcrypto.VerRctNonSemanticsSimple(rs)
	return xcrypto.TlvVerRctNotSemanticsSimple(rs)
}
func VerRctSemanticsSimple(rs *RctSig) bool {
	return xcrypto.VerRctSemanticsSimple(rs)
}

func VerRctSimpleTlv(rs *RctSig) bool {
	_, ret := xcrypto.TlvVerRctSimple(rs)
	return ret
}
func ProveRangeBulletproof(amounts KeyV, sk KeyV) (*Bulletproof, KeyV, KeyV, error) {
	return xcrypto.TlvProveRangeBulletproof(amounts, sk)
}
func ProveRangeBulletproof128(amounts KeyV, sk KeyV) (*Bulletproof, KeyV, KeyV, error) {
	return xcrypto.TlvProveRangeBulletproof128(amounts, sk)
}
func ProveRctMGSimple(message Key, pubs CtkeyV, inSk Ctkey, a, Count Key, mscout *Key, kLRki *MultisigKLRki, index uint32) (sig *MgSig, err error) {
	return xcrypto.TlvProveRctMGSimple(message, pubs, inSk, a, Count, mscout, kLRki, index)
}
func VerBulletproof(bp *Bulletproof) (bool, error) {
	return xcrypto.TlvVerBulletproof(bp)
}
func VerBulletproof128(bp *Bulletproof) (bool, error) {
	return xcrypto.TlvVerBulletproof128(bp)
}

//  41.112 µs/op(cgo)   6 µs/op(c)
func GetPreMlsagHash(rctsign *RctSig) (Key, error) {
	return xcrypto.TlvGetPreMlsagHash(rctsign)
}
func FromLkamountsToKeyv(amounts []Lk_amount) KeyV {
	vlen := len(amounts)
	ret := make(KeyV, vlen)
	for i := 0; i < vlen; i++ {
		ret[i] = Zero()
		ret[i][0] = (byte)(amounts[i] & 255)
		ret[i][1] = (byte)((amounts[i] >> 8) & 255)
		ret[i][2] = (byte)((amounts[i] >> 16) & 255)
		ret[i][3] = (byte)((amounts[i] >> 24) & 255)
		ret[i][4] = (byte)((amounts[i] >> 32) & 255)
		ret[i][5] = (byte)((amounts[i] >> 40) & 255)
		ret[i][6] = (byte)((amounts[i] >> 48) & 255)
		ret[i][7] = (byte)((amounts[i] >> 56) & 255)
	}
	return ret
}
