package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

// stdSigCache is used to cache the derived sender and contains
// the signer used to derive it.
type stdSigCache struct {
	signer STDSigner
	from   common.Address
}

// MakeSTDSigner returns a STDSigner based on the given signparam or default param.
func MakeSTDSigner(signParam *big.Int) STDSigner {
	var signer STDSigner
	if signParam != nil {
		signer = NewSTDEIP155Signer(signParam)
	} else {
		signer = NewSTDEIP155Signer(SignParam)
	}
	return signer
}

type signerData interface {
	recover(hash common.Hash, signParamMul *big.Int, homestead bool) (common.Address, error)
	SignParam() *big.Int
	Protected() bool
	signFields() []interface{}
	from() *atomic.Value
}

// STDSigner encapsulates signdata signature handling. Note that this interface is not a
// stable API and may change at any time to accommodate new protocol rules.
type STDSigner interface {
	// Sender returns the sender address of the signdata.
	Sender(data signerData) (common.Address, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(data signerData) common.Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(STDSigner) bool
	// SignParam return the field signParam
	SignParam() *big.Int
}

var _ signerData = &signdata{}
var big8 = big.NewInt(8)

type signdata struct {
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	fromValue atomic.Value

	signFieldsFunc func() []interface{}
}

// Sign signs the signdata using the given signer and private key
func sign(signer STDSigner, prv *ecdsa.PrivateKey, data []interface{}) (*big.Int, *big.Int, *big.Int, error) {
	fields := append(data, signer.SignParam(), uint(0), uint(0))
	h := rlpHash(fields)

	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, nil, nil, err
	}

	return signer.SignatureValues(sig)
}

// Sender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func sender(signer STDSigner, data signerData) (common.Address, error) {
	if sc := data.from().Load(); sc != nil {
		sigCache := sc.(stdSigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(data)
	if err != nil {
		return common.EmptyAddress, err
	}
	data.from().Store(stdSigCache{signer: signer, from: addr})
	return addr, nil
}

func senders(signer STDSigner, signatures []*signdata, signFields func() []interface{}) ([]common.Address, error) {
	addrs := make([]common.Address, 0, len(signatures))
	for _, s := range signatures {
		s.setSignFieldsFunc(signFields)
		addr, err := sender(signer, s)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

func isProtectedV(V *big.Int) bool {
	if V != nil && V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 are considered unprotected
	return true
}

// Protected returns whether the transaction is protected from replay protection.
func (data signdata) Protected() bool {
	return isProtectedV(data.V)
}

func (data *signdata) from() *atomic.Value {
	return &data.fromValue
}

func (data *signdata) setSignFieldsFunc(signFields func() []interface{}) {
	data.signFieldsFunc = signFields
}

func (data signdata) signFields() []interface{} {
	return data.signFieldsFunc()
}

func (data signdata) recover(hash common.Hash, signParamMul *big.Int, homestead bool) (common.Address, error) {
	if signParamMul == nil {
		return recoverPlain(hash, data.R, data.S, data.V, homestead)
	}
	V := new(big.Int).Sub(data.V, signParamMul)
	V.Sub(V, big8)
	return recoverPlain(hash, data.R, data.S, V, homestead)
}

// SignParam returns which sign param this transaction was signed with
func (data signdata) SignParam() *big.Int {
	return deriveSignParam(data.V)
}

// STDEIP155Signer implements STDSigner using the EIP155 rules.
type STDEIP155Signer struct {
	signParam, signParamMul *big.Int
}

// NewSTDEIP155Signer return a STDEIP155Signer
func NewSTDEIP155Signer(signParam *big.Int) STDEIP155Signer {
	if signParam == nil {
		signParam = new(big.Int)
	}
	return STDEIP155Signer{
		signParam:    signParam,
		signParamMul: new(big.Int).Mul(signParam, big.NewInt(2)),
	}
}

// SignParam return the field signParam
func (s STDEIP155Signer) SignParam() *big.Int {
	return s.signParam
}

// Equal returns true if the given signer is the same as the receiver.
func (s STDEIP155Signer) Equal(s2 STDSigner) bool {
	eip155, ok := s2.(STDEIP155Signer)
	return ok && eip155.signParam.Cmp(s.signParam) == 0
}

// Sender returns the sender address of the signdata.
func (s STDEIP155Signer) Sender(data signerData) (common.Address, error) {
	if !data.Protected() {
		return STDHomesteadSigner{}.Sender(data)
	}
	if data.SignParam().Cmp(s.signParam) != 0 {
		return common.EmptyAddress, ErrInvalidSignParam
	}
	return data.recover(s.Hash(data), s.signParamMul, true)
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s STDEIP155Signer) SignatureValues(sig []byte) (R, S, V *big.Int, err error) {
	R, S, V, err = STDHomesteadSigner{}.SignatureValues(sig)
	if err != nil {
		return nil, nil, nil, err
	}
	if s.signParam.Sign() != 0 {
		V = big.NewInt(int64(sig[64] + 35))
		V.Add(V, s.signParamMul)
	}
	return R, S, V, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the signdata.
func (s STDEIP155Signer) Hash(data signerData) common.Hash {
	h := data.signFields()
	h = append(h, s.signParam, uint(0), uint(0))
	return rlpHash(h)
}

// STDHomesteadSigner implements TransactionInterface using the homestead rules.
type STDHomesteadSigner struct{ STDFrontierSigner }

// SignParam return the field signParam
func (s STDHomesteadSigner) SignParam() *big.Int {
	return nil
}

// Equal returns true if the given signer is the same as the receiver.
func (s STDHomesteadSigner) Equal(s2 STDSigner) bool {
	_, ok := s2.(STDHomesteadSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s STDHomesteadSigner) SignatureValues(sig []byte) (*big.Int, *big.Int, *big.Int, error) {
	return s.STDFrontierSigner.SignatureValues(sig)
}

// Sender returns the sender address of the signdata.
func (s STDHomesteadSigner) Sender(data signerData) (common.Address, error) {
	return data.recover(s.Hash(data), nil, true)
}

// STDFrontierSigner implements TransactionInterface using the homestead rules.
type STDFrontierSigner struct{}

// SignParam return the field signParam
func (s STDFrontierSigner) SignParam() *big.Int {
	return nil
}

// Equal returns true if the given signer is the same as the receiver.
func (s STDFrontierSigner) Equal(s2 STDSigner) bool {
	_, ok := s2.(STDFrontierSigner)
	return ok
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s STDFrontierSigner) SignatureValues(sig []byte) (R, S, V *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	R = new(big.Int).SetBytes(sig[:32])
	S = new(big.Int).SetBytes(sig[32:64])
	V = new(big.Int).SetBytes([]byte{sig[64] + 27})
	return
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s STDFrontierSigner) Hash(data signerData) common.Hash {
	return rlpHash(data.signFields())
}

// Sender returns the sender address of the signdata.
func (s STDFrontierSigner) Sender(data signerData) (common.Address, error) {
	return data.recover(s.Hash(data), nil, false)
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (common.Address, error) {
	if Vb.BitLen() > 8 {
		return common.EmptyAddress, ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return common.EmptyAddress, ErrInvalidSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return common.EmptyAddress, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return common.EmptyAddress, errors.New("invalid public key")
	}
	var addr common.Address
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}

// deriveSignParam derives the sign param from the given v parameter
func deriveSignParam(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	if v.BitLen() <= 64 {
		v := v.Uint64()
		if v == 27 || v == 28 {
			return new(big.Int)
		}
		return new(big.Int).SetUint64((v - 35) / 2)
	}
	v = new(big.Int).Sub(v, big.NewInt(35))
	return v.Div(v, big.NewInt(2))
}
