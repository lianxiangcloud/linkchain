package types

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
)

type ContractCreateMainInfo struct {
	FromAddr     common.Address `json:"from" gencodec:"required"` // the Addr's signdata must be include in Signatures
	AccountNonce uint64         `json:"nonce"     gencodec:"required"`
	Amount       *big.Int       `json:"value"     gencodec:"required"`
	Payload      []byte         `json:"input"     gencodec:"required"`
	//GasLimit     uint64         `json:"gas"       gencodec:"required"`
	//Price *big.Int `json:"price"     gencodec:"required"`
}

type ContractCreateTx struct {
	ContractCreateMainInfo `json:"mainInfo"`
	Signatures             []*signdata `json:"signs"`
	// caches
	hash atomic.Value
}

func SignContractCreateTx(prv *ecdsa.PrivateKey, mainInfo *ContractCreateMainInfo) ([]byte, error) {
	if prv == nil {
		log.Warn("SignContractCreateTx Sign prv is nil")
		return nil, ErrParams
	}
	tx := &ContractCreateTx{ContractCreateMainInfo: *mainInfo}
	signature := &signdata{}
	signature.setSignFieldsFunc(tx.signFields)
	return signToBytes(signature, prv)
}

func CreateContractTx(mainInfo *ContractCreateMainInfo, sigData [][]byte) *ContractCreateTx {
	if mainInfo == nil {
		log.Warn("CreateContractTx mainInfo is nil")
		return nil
	}
	tx := &ContractCreateTx{ContractCreateMainInfo: *mainInfo} //we can not change the content of mainInfo
	tx.Signatures = make([]*signdata, len(sigData))
	err := initSignature(tx.Signatures, sigData)
	if err != nil {
		return nil
	}
	return tx
}

func (tx *ContractCreateTx) From() (common.Address, error) {
	return tx.FromAddr, nil
}

func (tx *ContractCreateTx) To() *common.Address {
	return nil
}

func (tx *ContractCreateTx) Cost() *big.Int {
	return new(big.Int).Set(tx.Amount)
}

func (tx *ContractCreateTx) Senders() ([]common.Address, error) {
	return senders(GlobalSTDSigner, tx.Signatures, tx.signFields)
}

func (tx *ContractCreateTx) Sign(signer STDSigner, prv *ecdsa.PrivateKey) error {
	r, s, v, err := sign(signer, prv, tx.signFields())
	if err != nil {
		return err
	}
	cpy := &ContractCreateTx{ContractCreateMainInfo: tx.ContractCreateMainInfo}
	sigs := make([]*signdata, 0, len(tx.Signatures)+1)
	for _, sig := range tx.Signatures {
		sigs = append(sigs, &signdata{
			V: new(big.Int).Set(sig.V),
			R: new(big.Int).Set(sig.R),
			S: new(big.Int).Set(sig.S),
		})
	}
	sigs = append(sigs, &signdata{V: v, R: r, S: s})
	cpy.Signatures = sigs
	*tx = *cpy
	return nil
}

func (tx *ContractCreateTx) signFields() []interface{} {
	return []interface{}{
		tx.ContractCreateMainInfo,
	}
}

func (tx *ContractCreateTx) VerifySign(signersInfo *SignersInfo) (err error) {
	if tx == nil || signersInfo == nil {
		err = fmt.Errorf("tx is nil or signersInfo is nil")
		return err
	}
	return verifySpecialTxWithSender(tx.FromAddr, signersInfo, tx.Signatures, tx.signFields)
}

func (tx *ContractCreateTx) Hash() common.Hash {
	hash := transactionHash(&tx.hash, tx)
	return hash
}

func (tx *ContractCreateTx) TypeName() string {
	return TxContractCreate
}

func (tx *ContractCreateTx) String() string {
	return fmt.Sprintf("{txHash=%s type=%s, from=%s, nonce=%d, amount=%s,payload=0x%x}",
		tx.Hash().Hex(), tx.TypeName(), tx.FromAddr.Hex(), tx.Nonce(), tx.Amount.String(), tx.Payload)
}

func (tx *ContractCreateTx) txType() SupportType {
	return TxContractCreateType
}

//IMessage
func (tx *ContractCreateTx) TokenAddress() common.Address { return common.EmptyAddress }
func (tx *ContractCreateTx) Data() []byte                 { return common.CopyBytes(tx.Payload) }
func (tx *ContractCreateTx) Gas() uint64                  { return ParGasLimit * 1000 }
func (tx *ContractCreateTx) GasPrice() *big.Int           { return new(big.Int).SetInt64(ParGasPrice) }
func (tx *ContractCreateTx) Value() *big.Int              { return new(big.Int).Set(tx.Amount) }
func (tx *ContractCreateTx) Nonce() uint64                { return tx.ContractCreateMainInfo.AccountNonce }

func (tx *ContractCreateTx) AsMessage() (Message, error) {
	msg := Message{
		nonce:     tx.Nonce(),
		gasLimit:  tx.Gas(),
		gasPrice:  tx.GasPrice(),
		to:        nil,
		amount:    tx.Amount,
		data:      tx.Payload,
		tokenAddr: tx.TokenAddress(),
		txType:    tx.TypeName(),
	}

	var err error
	msg.from, err = tx.From()
	return msg, err
}

func (tx *ContractCreateTx) CheckBasic(censor TxCensor) error {
	if tx == nil {
		return ErrTxEmpty
	}
	if len(tx.Payload) == 0 {
		return ErrParams
	}
	if tx.Amount == nil {
		log.Warn("tx.Amount is nil")
		return ErrParams
	}

	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}

	return tx.VerifySign(censor.TxMgr().GetMultiSignersInfo(tx.txType()))
}

func (tx *ContractCreateTx) CheckState(censor TxCensor) error {
	censor.LockState()
	defer censor.UnlockState()

	from := tx.FromAddr

	state := censor.State()
	// Check if nonce is not strictly increasing
	nonce := state.GetNonce(from)
	if nonce > tx.Nonce() {
		log.Debug("nonce too low", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooLow
	} else if nonce < tx.Nonce() {
		log.Debug("nonce too high", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooHigh
	}
	state.SetNonce(from, tx.Nonce()+1)

	// Check Balance
	cost := tx.Cost()
	balance := state.GetBalance(from)
	if balance.Cmp(cost) < 0 {
		return ErrInsufficientFunds
	}
	state.SubBalance(from, cost)
	return nil
}
