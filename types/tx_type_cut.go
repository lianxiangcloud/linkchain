package types

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
)

type ContractUpgradeMainInfo struct {
	FromAddr     common.Address `json:"from"      gencodec:"required"` // the Addr's signdata must be include in Signatures
	Recipient    common.Address `json:"contract"  gencodec:"required" rlp:"nil"`
	AccountNonce uint64         `json:"nonce"     gencodec:"required"`
	Payload      []byte         `json:"input"     gencodec:"required"`
}

type ContractUpgradeTx struct {
	ContractUpgradeMainInfo `json:"mainInfo"`
	Signatures              []*signdata `json:"signs"`
	// caches
	hash atomic.Value
}

func SignContractUpgradeTx(prv *ecdsa.PrivateKey, mainInfo *ContractUpgradeMainInfo) ([]byte, error) {
	if prv == nil {
		log.Warn("SignContractUpgradeTx Sign prv is nil")
		return nil, ErrParams
	}
	tx := &ContractUpgradeTx{ContractUpgradeMainInfo: *mainInfo}
	signature := &signdata{}
	signature.setSignFieldsFunc(tx.signFields)
	return signToBytes(signature, prv)
}

func UpgradeContractTx(mainInfo *ContractUpgradeMainInfo, sigData [][]byte) *ContractUpgradeTx {
	if mainInfo == nil {
		log.Warn("CreateContractTx mainInfo is nil")
		return nil
	}
	tx := &ContractUpgradeTx{ContractUpgradeMainInfo: *mainInfo} //we can not change the content of mainInfo
	tx.Signatures = make([]*signdata, len(sigData))
	err := initSignature(tx.Signatures, sigData)
	if err != nil {
		return nil
	}
	return tx
}

func (tx *ContractUpgradeTx) From() (common.Address, error) {
	return tx.FromAddr, nil
}

func (tx *ContractUpgradeTx) To() *common.Address {
	return &tx.Recipient
}

func (tx *ContractUpgradeTx) Cost() *big.Int {
	return big.NewInt(0)
}

func (tx *ContractUpgradeTx) Senders() ([]common.Address, error) {
	return senders(GlobalSTDSigner, tx.Signatures, tx.signFields)
}

func (tx *ContractUpgradeTx) Sign(signer STDSigner, prv *ecdsa.PrivateKey) error {
	r, s, v, err := sign(signer, prv, tx.signFields())
	if err != nil {
		return err
	}
	cpy := &ContractUpgradeTx{ContractUpgradeMainInfo: tx.ContractUpgradeMainInfo}
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

func (tx *ContractUpgradeTx) signFields() []interface{} {
	return []interface{}{
		tx.ContractUpgradeMainInfo,
	}
}

func (tx *ContractUpgradeTx) VerifySign(signersInfo *SignersInfo) (err error) {
	if tx == nil || signersInfo == nil {
		err = fmt.Errorf("tx is nil or signersInfo is nil")
		return err
	}
	return verifySpecialTxWithSender(tx.FromAddr, signersInfo, tx.Signatures, tx.signFields)
}

func (tx *ContractUpgradeTx) Hash() common.Hash {
	hash := transactionHash(&tx.hash, tx)
	return hash
}

func (tx *ContractUpgradeTx) TypeName() string {
	return TxContractUpgrade
}

func (tx *ContractUpgradeTx) String() string {
	return fmt.Sprintf("{txHash=0x%x type=%s, from=0x%s, contract=%s, nonce=%d, payload=0x%x}",
		tx.Hash(), tx.TypeName(), tx.FromAddr.Hex(), tx.Recipient.Hex(), tx.Nonce(), tx.Payload)
}

func (tx *ContractUpgradeTx) txType() SupportType {
	return TxContractCreateType
}

//IMessage
func (tx *ContractUpgradeTx) TokenAddress() common.Address { return common.EmptyAddress }
func (tx *ContractUpgradeTx) Data() []byte                 { return common.CopyBytes(tx.Payload) }
func (tx *ContractUpgradeTx) Gas() uint64                  { return ParGasLimit * 1000 }
func (tx *ContractUpgradeTx) GasPrice() *big.Int           { return new(big.Int).SetInt64(ParGasPrice) }
func (tx *ContractUpgradeTx) Value() *big.Int              { return big.NewInt(0) }
func (tx *ContractUpgradeTx) Nonce() uint64                { return tx.ContractUpgradeMainInfo.AccountNonce }

func (tx *ContractUpgradeTx) AsMessage() (Message, error) {
	msg := Message{
		nonce:     tx.Nonce(),
		gasLimit:  tx.Gas(),
		gasPrice:  tx.GasPrice(),
		to:        tx.To(),
		amount:    tx.Value(),
		data:      tx.Payload,
		tokenAddr: tx.TokenAddress(),
		txType:    tx.TypeName(),
	}

	var err error
	msg.from, err = tx.From()
	return msg, err
}

func (tx *ContractUpgradeTx) CheckBasic(censor TxCensor) error {
	if tx == nil {
		return ErrTxEmpty
	}
	if len(tx.Payload) == 0 {
		return ErrParams
	}

	if !censor.IsWasmContract(tx.Payload) {
		return fmt.Errorf("UpgardeCOntractTx: Payload not wasm")
	}

	return tx.VerifySign(censor.TxMgr().GetMultiSignersInfo(tx.txType()))
}

func (tx *ContractUpgradeTx) CheckState(censor TxCensor) error {
	censor.LockState()
	defer censor.UnlockState()

	state := censor.State()
	// Check if nonce is not strictly increasing
	from := tx.FromAddr
	nonce := state.GetNonce(from)
	if nonce > tx.Nonce() {
		log.Debug("nonce too low", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooLow
	} else if nonce < tx.Nonce() {
		log.Debug("nonce too high", "got", tx.Nonce(), "want", nonce)
		return ErrNonceTooHigh
	}
	state.SetNonce(from, tx.Nonce()+1)
	return nil
}
