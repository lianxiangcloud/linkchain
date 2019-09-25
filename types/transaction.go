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
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"sync/atomic"

	"encoding/json"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/hexutil"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

const (
	ParGasPrice            int64  = 1e11
	ParGasLimit            uint64 = 1e5
	MaxTransactionSize            = 256 * 1024
	MaxWasmTransactionSize        = 256 * 1024
	MaxPureTransactionSize        = 32 * 1024
	wasmID                 uint32 = 0x6d736100
	wasmIDLength                  = 4
)

var (
	ErrInvalidSig           = errors.New("invalid transaction v, r, s values")
	ErrAccountOutputTooMore = errors.New("account output too more")
)

var _ signerData = &txdata{}

type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"    gencodec:"required"`
	Price        *big.Int        `json:"gasPrice" gencodec:"required"`
	GasLimit     uint64          `json:"gas"      gencodec:"required"`
	Recipient    *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"    gencodec:"required"`
	Payload      []byte          `json:"input"    gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	fromValue atomic.Value

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

// Protected returns whether the transaction is protected from replay protection.
func (data txdata) Protected() bool {
	return isProtectedV(data.V)
}

func (data *txdata) from() *atomic.Value {
	return &data.fromValue
}

func (data txdata) signFields() []interface{} {
	return []interface{}{
		data.AccountNonce,
		data.Price,
		data.GasLimit,
		data.Recipient,
		data.Amount,
		data.Payload,
	}
}

func (data txdata) recover(hash common.Hash, signParamMul *big.Int, homestead bool) (common.Address, error) {
	if signParamMul == nil {
		return recoverPlain(hash, data.R, data.S, data.V, homestead)
	}
	V := new(big.Int).Sub(data.V, signParamMul)
	V.Sub(V, big8)
	return recoverPlain(hash, data.R, data.S, V, homestead)
}

// SignParam returns which sign param this transaction was signed with
func (data txdata) SignParam() *big.Int {
	return DeriveSignParam(data.V)
}

func (tx *Transaction) Sign(signer STDSigner, prv *ecdsa.PrivateKey) error {
	r, s, v, err := sign(signer, prv, tx.data.signFields())
	if err != nil {
		return err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	*tx = *cpy
	return nil
}

func (tx *Transaction) SignHash() common.Hash {
	return GlobalSTDSigner.Hash(&tx.data)
}

func (tx *Transaction) Sender(signer STDSigner) (common.Address, error) {
	return sender(signer, &tx.data)
}

// SignParam returns which sign param this transaction was signed with
func (tx *Transaction) SignParam() *big.Int {
	return tx.data.SignParam()
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *Transaction) Protected() bool {
	return tx.data.Protected()
}

func (tx *Transaction) TypeName() string {
	return TxNormal
}

func (tx *Transaction) From() (common.Address, error) {
	return tx.Sender(GlobalSTDSigner)
}

func (tx *Transaction) StoreFrom(addr common.Address) {
	tx.data.from().Store(stdSigCache{signer: GlobalSTDSigner, from: addr})
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

func NewTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data)
}

func NewContractCreation(nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, nil, amount, gasLimit, gasPrice, data)
}

func newTransaction(nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
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

	return &Transaction{data: d}
}

func IsContract(data []byte) bool {
	if len(data) > 0 {
		var extra map[string]interface{}
		err := json.Unmarshal(data, &extra)
		if err == nil {
			// third party json payload
			return false
		}
		return true
	}
	return false
}

func (tx *Transaction) IllegalGasLimitOrGasPrice(hascode bool) bool {
	if tx.GasPrice().Cmp(big.NewInt(ParGasPrice)) != 0 {
		log.Info("ParGasPrice!=0", "GasPrice", tx.GasPrice())
		return true
	}
	var gasFee uint64
	if hascode {
		gasFee = CalNewAmountGas(tx.Value(), EverContractLiankeFee)
	} else {
		gasFee = CalNewAmountGas(tx.Value(), EverLiankeFee)
	}

	if tx.Value().Sign() > 0 {
		if gasFee > tx.Gas() {
			log.Info("gasFee > tx.Gas()", "gasFee", gasFee, "tx.Gas()", tx.Gas())
			return true
		}
	}
	iscontract := IsContract(tx.Data())
	if iscontract && tx.To() == nil {
		return false
	}
	if !hascode && gasFee != tx.Gas() {
		log.Info("gasFee != tx.Gas()", "gasFee", gasFee, "tx.Gas()", tx.Gas())
		return true
	}

	return false
}

// EncodeSER implements ser.Encoder
func (tx *Transaction) EncodeSER(w io.Writer) error {
	return ser.Encode(w, &tx.data)
}

// DecodeSER implements ser.Decoder
func (tx *Transaction) DecodeSER(s *ser.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(ser.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}
	var V byte
	if isProtectedV(dec.V) {
		signParam := DeriveSignParam(dec.V).Uint64()
		V = byte(dec.V.Uint64() - 35 - 2*signParam)
	} else {
		V = byte(dec.V.Uint64() - 27)
	}
	if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
		return ErrInvalidSig
	}
	*tx = Transaction{data: dec}
	return nil
}

func (tx Transaction) GetTxData() txdata             { return tx.data }
func (tx *Transaction) TokenAddress() common.Address { return common.EmptyAddress }
func (tx *Transaction) Data() []byte                 { return common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64                  { return tx.data.GasLimit }
func (tx *Transaction) GasPrice() *big.Int           { return new(big.Int).Set(tx.data.Price) }
func (tx *Transaction) Value() *big.Int              { return new(big.Int).Set(tx.data.Amount) }
func (tx *Transaction) Nonce() uint64                { return tx.data.AccountNonce }

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	ser.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// AsMessage returns the transaction as a core.Message.
//
// AsMessage requires a signer to derive the sender.
//
// XXX Rename message to something less arbitrary?
func (tx *Transaction) AsMessage() (Message, error) {
	msg := Message{
		nonce:     tx.data.AccountNonce,
		gasLimit:  tx.data.GasLimit,
		gasPrice:  new(big.Int).Set(tx.data.Price),
		to:        tx.data.Recipient,
		amount:    tx.data.Amount,
		data:      tx.data.Payload,
		tokenAddr: common.EmptyAddress,
		txType:    TxNormal,
	}

	var err error
	msg.from, err = tx.From()
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (tx *Transaction) WithSignature(signer STDSigner, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// Cost returns amount + gasprice * gaslimit.
func (tx *Transaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

func (tx *Transaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

func (tx *Transaction) RawString() string {
	if tx == nil {
		return ""
	}
	enc, _ := ser.EncodeToBytes(&tx.data)
	return fmt.Sprintf("0x%x", enc)
}

func (tx *Transaction) String() string {
	var from, to string
	if tx.data.V != nil {
		if f, err := tx.From(); err != nil { // derive but don't cache
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
		from,
		to,
		tx.data.AccountNonce,
		tx.data.Price,
		tx.data.GasLimit,
		tx.data.Amount,
		tx.data.Payload,
		tx.data.V,
		tx.data.R,
		tx.data.S,
		enc,
	)
}

// IsWasmContract check contract's id
func IsWasmContract(code []byte) bool {
	if len(code) > wasmIDLength {
		if wasmID == (uint32(code[0]) | uint32(code[1])<<8 | uint32(code[2])<<16 | uint32(code[3])<<24) {
			return true
		}
	}
	return false
}

func (tx *Transaction) CheckBasic(censor TxCensor) error {
	return tx.CheckBasicWithState(censor, nil)
}
func (tx *Transaction) CheckBasicWithState(censor TxCensor, state State) error {
	if tx == nil {
		return ErrTxEmpty
	}

	if tx.To() == nil && !IsTestMode {
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

func (tx *Transaction) CheckState(censor TxCensor) error {
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
		log.Info("nonce too low", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooLow
	} else if nonce < tx.Nonce() {
		log.Debug("nonce too high", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooHigh
	}

	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	cost := tx.Cost()
	balance := state.GetBalance(from)
	if balance.Cmp(cost) < 0 {
		return ErrInsufficientFunds
	}

	// Update ether balances
	// amount + gasprice * gaslimit
	state.SubBalance(from, cost)
	state.SetNonce(from, tx.Nonce()+1)
	return nil
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func intrinsicGas(data []byte, contractCreation, homestead bool) (gas uint64, err error) {
	// Set the starting gas for the raw transaction
	if contractCreation && homestead {
		gas = cfg.TxGasContractCreation
	} else {
		gas = cfg.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) <= 0 {
		return
	}
	// Zero and non-zero bytes are priced differently
	var nz uint64
	for _, byt := range data {
		if byt != 0 {
			nz++
		}
	}
	// Make sure we don't exceed uint64 for all data combinations
	if (math.MaxUint64-gas)/cfg.TxDataNonZeroGas < nz {
		return 0, ErrOutOfGas
	}
	gas += nz * cfg.TxDataNonZeroGas

	z := uint64(len(data)) - nz
	if (math.MaxUint64-gas)/cfg.TxDataZeroGas < z {
		return 0, ErrOutOfGas
	}
	gas += z * cfg.TxDataZeroGas
	return
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []RegularTx

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := ser.EncodeToBytes(s[i])
	return enc
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].Nonce() < s[j].Nonce() }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type OutputData struct {
	To     common.Address
	Amount *big.Int
	Data   []byte
}

func (od OutputData) String() string {
	return fmt.Sprintf("To:%s, Amount:%v, Data:%x", od.To.String(), od.Amount, od.Data)
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to        *common.Address
	from      common.Address
	tokenAddr common.Address
	nonce     uint64
	amount    *big.Int //UTXOTransaction only use as AccountInput amount
	gasLimit  uint64
	gasPrice  *big.Int
	data      []byte
	//for ContractCreateTransaction
	txType string
	//for UTXO
	utxoKind       UTXOKind
	accountOutputs []OutputData //available if UTXOTransaction has accountOutput
}

func NewMessage(from common.Address, to *common.Address, tokenAddr common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) Message {
	return Message{
		from:      from,
		to:        to,
		tokenAddr: tokenAddr,
		nonce:     nonce,
		amount:    amount,
		gasLimit:  gasLimit,
		gasPrice:  gasPrice,
		data:      data,
	}
}
func (m Message) String() string {
	outputStr := "["
	for _, accountOutput := range m.accountOutputs {
		outputStr += accountOutput.String()
		outputStr += ", "
	}
	outputStr += "]"
	toAddr := ""
	if m.to != nil {
		toAddr = m.to.String()
	}
	return fmt.Sprintf("To:%s, From:%s, TokenAddr:%s, nonce:%v, amount:%v, gasLimit:%v, gasPrice:%v, data:%x, txType:%s, utxoKind:%b, outputs:%s",
		toAddr, m.from.String(), m.tokenAddr.String(), m.nonce, m.amount, m.gasLimit, m.gasPrice, m.data, m.txType, m.utxoKind, outputStr)
}
func (m Message) MsgFrom() common.Address { return m.from }
func (m Message) To() *common.Address     { return m.to }
func (m Message) GasPrice() *big.Int      { return m.gasPrice }

// GasCost returns gasprice * gaslimit.
func (m Message) GasCost() *big.Int {
	return new(big.Int).Mul(m.gasPrice, new(big.Int).SetUint64(m.gasLimit))
}
func (m Message) Value() *big.Int              { return m.amount }
func (m Message) Gas() uint64                  { return m.gasLimit }
func (m Message) Nonce() uint64                { return m.nonce }
func (m Message) Data() []byte                 { return m.data }
func (m Message) TokenAddress() common.Address { return m.tokenAddr }
func (m Message) AsMessage() (Message, error)  { return m, nil }
func (m Message) TxType() string               { return m.txType }
func (m *Message) UTXOKind() UTXOKind          { return m.utxoKind }
func (m *Message) SetTxType(txType string)     { m.txType = txType }
func (m *Message) SetUTXOKind(kind UTXOKind)   { m.utxoKind = kind }
func (m *Message) OutputData() []OutputData    { return m.accountOutputs }
func (m *Message) SetOutputData(outputs []OutputData) {
	m.accountOutputs = append(m.accountOutputs, outputs...)
}

//UTXOTransaction use TODO
func (m *Message) From(tx Tx, state State) (common.Address, error) {
	if (m.utxoKind&Ain) == Ain || len(m.accountOutputs) > 0 && state.IsContract(m.accountOutputs[0].To) {
		var err error
		m.from, err = tx.From()
		return m.from, err
	}
	return common.EmptyAddress, nil
}
