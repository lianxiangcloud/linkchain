package types

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	RCTTypeNull         RangeProofType = 0
	RCTTypeFull         RangeProofType = 1
	RCTTypeSimple       RangeProofType = 2
	RCTTypeBulletproof  RangeProofType = 3
	RCTTypeBulletproof2 RangeProofType = 4
)

const (
	COMMONLEN = 32
	KEY64     = 64
)

type RangeProofType uint8
type Lk_amount uint64
type Key [COMMONLEN]byte

type Key64 [KEY64]Key
type KeyV []Key
type KeyM []KeyV

type CtkeyV []Ctkey
type CtkeyM []CtkeyV
type Ctkey struct {
	Dest Key
	Mask Key
}
type MultisigKLRki struct {
	K  Key
	L  Key
	R  Key
	Ki Key
}
type MultisigOut struct {
	C KeyV
}
type EcdhTuple struct {
	Mask     Key
	Amount   Key
	SenderPK Key
}

type BoroSig struct {
	S0 Key64
	S1 Key64
	Ee Key
}

type MgSig struct {
	Ss KeyM
	Cc Key
	II KeyV `json:"-" rlp:"-"`
}

type RangeSig struct {
	Asig BoroSig
	Ci   Key64
}
type Bulletproof struct {
	V            KeyV `json:"-" rlp:"-"`
	A, S, T1, T2 Key
	Taux, Mu     Key
	L, R         KeyV
	Aa, B, T     Key // Aa --> a
}
type RctConfig struct {
	RangeProofType RangeProofType
	BpVersion      int32
}
type RctSigBase struct {
	Type       uint8
	Message    Key    `json:"-" rlp:"-"`
	MixRing    CtkeyM `json:"-" rlp:"-"`
	PseudoOuts KeyV
	EcdhInfo   []EcdhTuple
	OutPk      CtkeyV
	TxnFee     Lk_amount
}
type RctSigPrunable struct {
	RangeSigs    []RangeSig
	Bulletproofs []Bulletproof
	MGs          []MgSig
	PseudoOuts   KeyV
	Ss           []Signature
}
type RctSig struct {
	RctSigBase
	P RctSigPrunable
}

type EcPoint Key
type EcScalar Key

type PublicKey EcPoint
type SecretKey EcScalar
type SecretKeyV []SecretKey

type KeyDerivation EcPoint
type KeyImage EcPoint

type Signature struct {
	C, R EcScalar
}

//AccountAddress represents the address with a view-public-key and a spend-public-ley
type AccountAddress struct {
	ViewPublicKey  PublicKey
	SpendPublicKey PublicKey
}

//AccountKey represents a utxo account
type AccountKey struct {
	Addr      AccountAddress
	SpendSKey SecretKey
	ViewSKey  SecretKey
	Address   string
	SubIdx    uint64
}

func (a *AccountKey) String() string {
	return fmt.Sprintf("{Addr:[%x %x],SubIdx:%d}",
		a.Addr.ViewPublicKey, a.Addr.SpendPublicKey, a.SubIdx)
}

func (key *Key) IsEqual(to *Key) bool {
	for i := 0; i < COMMONLEN; i++ {
		if key[i] != to[i] {
			return false
		}
	}
	return true
}
func (key *KeyV) IsEqual(to *KeyV) bool {
	if len(*key) != len(*to) {
		return false
	}
	for i := 0; i < len(*key); i++ {
		if !(*key)[i].IsEqual(&((*to)[i])) {
			return false
		}
	}
	return true
}

func (t *Ctkey) IsEqual(to *Ctkey) bool {
	if t.Mask.IsEqual(&(to.Mask)) && t.Dest.IsEqual(&(to.Dest)) {
		return true
	}
	return false
}
func (t *CtkeyV) IsEqual(to *CtkeyV) bool {
	tlen := len(*t)
	if tlen != len(*to) {
		return false
	}
	for i := 0; i < tlen; i++ {
		if !(*t)[i].IsEqual(&((*to)[i])) {
			return false
		}
	}
	return true
}
func (t *CtkeyM) IsEqual(to *CtkeyM) bool {
	tlen := len(*t)
	if tlen != len(*to) {
		return false
	}
	for i := 0; i < tlen; i++ {
		if !(*t)[i].IsEqual(&((*to)[i])) {
			return false
		}
	}
	return true
}

// Big converts a hash to a big integer.
func (h Key) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h Key) Hex() string { return strings.ToLower(hex.EncodeToString(h[:])) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Key) String() string {
	return h.Hex()
}

// Big converts a hash to a big integer.
func (h PublicKey) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h PublicKey) Hex() string { return strings.ToLower(hex.EncodeToString(h[:])) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h PublicKey) String() string {
	return h.Hex()
}

// Big converts a hash to a big integer.
func (h SecretKey) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h SecretKey) Hex() string { return strings.ToLower(hex.EncodeToString(h[:])) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h SecretKey) String() string {
	return h.Hex()
}

// Big converts a hash to a big integer.
func (h KeyDerivation) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h KeyDerivation) Hex() string { return strings.ToLower(hex.EncodeToString(h[:])) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h KeyDerivation) String() string {
	return h.Hex()
}

// Big converts a hash to a big integer.
func (h KeyImage) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

// Hex converts a hash to a hex string.
func (h KeyImage) Hex() string { return strings.ToLower(hex.EncodeToString(h[:])) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h KeyImage) String() string {
	return h.Hex()
}
