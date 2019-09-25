// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

type TokenTransaction struct {
	data tokenData
	// caches
	hash atomic.Value
	size atomic.Value
}

func NewTokenTransaction(tokenAddress common.Address, nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *TokenTransaction {
	return newTokenTransaction(tokenAddress, nonce, &to, amount, gasLimit, gasPrice, data)
}

func newTokenTransaction(tokenAddress common.Address, nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *TokenTransaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := tokenData{

		TokenAddress: tokenAddress,
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		Signdata: signdata{
			V: new(big.Int),
			R: new(big.Int),
			S: new(big.Int),
		},
	}
	if amount != nil {
		d.Amount.Set(amount)
	}

	gasPrice = big.NewInt(1e11)
	d.Price.Set(gasPrice)
	if gasLimit > 0 {
		d.GasLimit = gasLimit
	} else {
		fee := CalNewAmountGas(amount, EverLiankeFee)
		d.GasLimit = fee
	}

	return &TokenTransaction{data: d}
}

type tokenData struct {
	TokenAddress common.Address  `json:"tokenAddress" rlp:"nil" gencodec:"required"`
	AccountNonce uint64          `json:"nonce"    gencodec:"required"`
	Price        *big.Int        `json:"gasPrice" gencodec:"required"`
	GasLimit     uint64          `json:"gas"      gencodec:"required"`
	Recipient    *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"    gencodec:"required"`
	Payload      []byte          `json:"input"    gencodec:"required"`

	// Signature values
	Signdata signdata

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func (tx *TokenTransaction) signFields() []interface{} {
	return []interface{}{
		tx.data.TokenAddress,
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Recipient,
		tx.data.Amount,
		tx.data.Payload,
	}
}

func (tx *TokenTransaction) RawString() string {
	if tx == nil {
		return ""
	}
	enc, _ := ser.EncodeToBytes(&tx.data)
	return fmt.Sprintf("0x%x", enc)
}

func (tx *TokenTransaction) Sign(signer STDSigner, prv *ecdsa.PrivateKey) error {
	tx.data.Signdata.setSignFieldsFunc(tx.signFields)
	r, s, v, err := sign(signer, prv, tx.data.Signdata.signFields())
	if err != nil {
		return err
	}
	cpy := &TokenTransaction{data: tx.data}
	cpy.data.Signdata.R, cpy.data.Signdata.S, cpy.data.Signdata.V = r, s, v
	*tx = *cpy
	return nil
}

func (tx *TokenTransaction) SignHash() common.Hash {
	tx.data.Signdata.setSignFieldsFunc(tx.signFields)
	return GlobalSTDSigner.Hash(&tx.data.Signdata)
}

func (tx *TokenTransaction) Sender(signer STDSigner) (common.Address, error) {
	tx.data.Signdata.setSignFieldsFunc(tx.signFields)
	return sender(signer, &tx.data.Signdata)
}

// SignParam returns which sign param this transaction was signed with
func (tx *TokenTransaction) SignParam() *big.Int {
	return tx.data.Signdata.SignParam()
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *TokenTransaction) Protected() bool {
	return tx.data.Signdata.Protected()
}

func (tx *TokenTransaction) StoreFrom(addr common.Address) {
	tx.data.Signdata.fromValue.Store(stdSigCache{signer: GlobalSTDSigner, from: addr})
}

func (tx *TokenTransaction) IllegalGasLimitOrGasPrice(hascode bool) bool {
	if tx.GasPrice().Cmp(big.NewInt(ParGasPrice)) != 0 {
		return true
	}
	var gasFee uint64
	if hascode {
		if common.IsLKC(tx.data.TokenAddress) {
			gasFee = CalNewAmountGas(tx.Value(), EverContractLiankeFee)
		} else {
			gasFee = 0
		}
	} else {
		if common.IsLKC(tx.data.TokenAddress) {
			gasFee = CalNewAmountGas(tx.Value(), EverLiankeFee)
		} else {
			gasFee = uint64(MinGasLimit)
		}
	}

	if tx.Value().Sign() > 0 {
		if gasFee > tx.Gas() {
			return true
		}
	}
	iscontract := IsContract(tx.Data())
	if iscontract && tx.To() == nil {
		return true
	}
	if !hascode && gasFee != tx.Gas() {
		return true
	}

	return false
}

func (tx *TokenTransaction) TypeName() string {
	return TxToken
}

func (tx *TokenTransaction) From() (common.Address, error) {
	return tx.Sender(GlobalSTDSigner)
}

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *TokenTransaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *TokenTransaction) Hash() (h common.Hash) {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}

	hashFields := append(tx.signFields(), tx.data.Signdata)
	v := rlpHash(hashFields)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the TokenTransaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *TokenTransaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	ser.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// EncodeSER implements ser.Encoder
func (tx *TokenTransaction) EncodeSER(w io.Writer) error {
	return ser.Encode(w, &tx.data)
}

// DecodeSER implements ser.Decoder
func (tx *TokenTransaction) DecodeSER(s *ser.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(ser.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx TokenTransaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *TokenTransaction) UnmarshalJSON(input []byte) error {
	var dec tokenData
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}
	var V byte
	if isProtectedV(dec.Signdata.V) {
		signParam := DeriveSignParam(dec.Signdata.V).Uint64()
		V = byte(dec.Signdata.V.Uint64() - 35 - 2*signParam)
	} else {
		V = byte(dec.Signdata.V.Uint64() - 27)
	}
	if !crypto.ValidateSignatureValues(V, dec.Signdata.R, dec.Signdata.S, false) {
		return ErrInvalidSig
	}
	*tx = TokenTransaction{data: dec}
	return nil
}

func (tx *TokenTransaction) String() string {
	var from, to string
	if tx.data.Signdata.V != nil {
		if f, err := tx.Sender(GlobalSTDSigner); err != nil { // derive but don't cache
			from = "[invalid sender: invalid sig]"
		} else {
			from = fmt.Sprintf("%x", f[:])
		}
	} else {
		from = "[invalid sender: nil V field]"
	}

	if tx.data.Recipient == nil {
		to = "[contract creation]"
	} else {
		to = fmt.Sprintf("%x", tx.data.Recipient[:])
	}

	enc, _ := ser.EncodeToBytes(&tx.data)
	return fmt.Sprintf(`
	TX(0x%x)
	Contract: %v
	Token:    0x%x
	From:     0x%s
	To:       0x%s
	Nonce:    %v
	GasPrice: %#x
	GasLimit  %#x
	Value:    %#x
	Data:     0x%x
	V:        %#x
	R:        %#x
	S:        %#x
	Hex:      0x%x
`,
		tx.Hash(),
		tx.data.Recipient == nil,
		tx.data.TokenAddress,
		from,
		to,
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.Signdata.V,
		tx.data.Signdata.R,
		tx.data.Signdata.S,
		enc,
	)
}

func (tx *TokenTransaction) AsMessage() (Message, error) {
	msg := Message{
		nonce:     tx.data.AccountNonce,
		gasLimit:  tx.data.GasLimit,
		gasPrice:  new(big.Int).Set(tx.data.Price),
		to:        tx.data.Recipient,
		amount:    tx.data.Amount,
		data:      tx.data.Payload,
		tokenAddr: tx.data.TokenAddress,
		txType:    tx.TypeName(),
	}

	var err error
	msg.from, err = tx.From()
	return msg, err
}

func (tx *TokenTransaction) TokenAddress() common.Address { return tx.data.TokenAddress }
func (tx *TokenTransaction) Data() []byte                 { return common.CopyBytes(tx.data.Payload) }
func (tx *TokenTransaction) Gas() uint64                  { return tx.data.GasLimit }
func (tx *TokenTransaction) GasPrice() *big.Int           { return new(big.Int).Set(tx.data.Price) }
func (tx *TokenTransaction) Value() *big.Int              { return new(big.Int).Set(tx.data.Amount) }
func (tx *TokenTransaction) Nonce() uint64                { return tx.data.AccountNonce }

// Cost returns gasprice * gaslimit.
func (tx *TokenTransaction) GasCost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	return total
}

func (tx *TokenTransaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

func (tx *TokenTransaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.Signdata.V, tx.data.Signdata.R, tx.data.Signdata.S
}

func (tx *TokenTransaction) CheckBasic(censor TxCensor) error {
	return tx.CheckBasicWithState(censor, nil)
}

func (tx *TokenTransaction) CheckBasicWithState(censor TxCensor, state State) error {
	if tx == nil {
		return ErrTxEmpty
	}

	if tx.To() == nil {
		return ErrInvalidReceiver
	}
	if tx.data.Amount == nil || tx.data.Price == nil {
		log.Warn("tx.Amount or tx.Price is nil")
		return ErrParams
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}
	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > MaxPureTransactionSize {
		if tx.To() == nil {
			if tx.Size() > MaxWasmTransactionSize && IsWasmContract(tx.data.Payload[:wasmIDLength+1]) {
				return ErrOversizedData
			}
		} else {
			return ErrOversizedData
		}
	}

	hascode := false
	if state == nil { // without state, need lock and get state
		if tx.To() != nil {
			censor.LockState()
			state := censor.State()
			if state.IsContract(*tx.To()) {
				hascode = true
			}
			censor.UnlockState()
		}
	} else { // state is locked outside
		if tx.To() != nil {
			if state.IsContract(*tx.To()) {
				hascode = true
			}
		}
	}

	if tx.IllegalGasLimitOrGasPrice(hascode) {
		return ErrGasLimitOrGasPrice
	}

	// Make sure the transaction is signed properly
	if _, err := tx.From(); err != nil {
		return ErrInvalidSender
	}

	intrGas, err := intrinsicGas(tx.Data(), false, true) // homestead == true
	if err != nil {
		return ErrOutOfGas
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}

	return nil
}

func (tx *TokenTransaction) CheckState(censor TxCensor) error {
	censor.LockState()
	defer censor.UnlockState()

	from, err := tx.From()
	if err != nil {
		return ErrInvalidSender
	}

	state := censor.State()
	// Make sure the account exist - cant send from non-existing account.
	// if !state.Exist(from) {
	// 	return ErrInvalidSender
	// }

	// Check if nonce is not strictly increasing
	nonce := state.GetNonce(from)
	if nonce > tx.Nonce() {
		log.Warn("nonce too low", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooLow
	} else if nonce < tx.Nonce() {
		return ErrNonceTooHigh
	}

	isBalance := false
	token := tx.TokenAddress()
	if token == common.EmptyAddress {
		isBalance = true
	}
	if isBalance {
		cost := tx.Cost()
		if state.GetBalance(from).Cmp(cost) < 0 {
			return ErrInsufficientFunds
		}
		state.SubBalance(from, cost)
	} else {
		value := tx.Value()
		gasCost := tx.GasCost()
		if state.GetTokenBalance(from, token).Cmp(value) < 0 ||
			state.GetBalance(from).Cmp(gasCost) < 0 {
			return ErrInsufficientFunds
		}
		state.SubBalance(from, gasCost)
		state.SubTokenBalance(from, token, value)
	}

	state.SetNonce(from, tx.Nonce()+1)
	return nil
}
