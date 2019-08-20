package types

import (
	"fmt"
	"sync/atomic"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

var MultiSignNonceAddr = common.BytesToAddress([]byte("mst"))

type SignerEntry struct {
	Power int32          `json:"power"`
	Addr  common.Address `json:"addr"`
}

type ValidatorSign struct {
	Addr      []byte `json: "addr"`
	Signature []byte `json: "sign"`
}

type SignersInfo struct {
	MinSignerPower int32          `json:"minSignerPower"`
	Signers        []*SignerEntry `json:"signers"`
}

type MultiSignMainInfo struct {
	AccountNonce  uint64      `json:"nonce"` //the nonce of MultiSignNonceAddr
	SupportTxType SupportType `json:"txTypes"`
	SignersInfo   `json:"signersInfo"`
}

type MultiSignAccountTx struct {
	MultiSignMainInfo `json: "mainInfo"` //the main info that every validator needs sign for
	Signatures        []ValidatorSign    `json: "signs"` //validators’ signs，at least 2/3 valid ValidatorSign
	//cache
	hash atomic.Value
}

func NewMultiSignAccountTx(mainInfo *MultiSignMainInfo, Signatures []ValidatorSign) *MultiSignAccountTx {
	tx := &MultiSignAccountTx{MultiSignMainInfo: *mainInfo}
	if len(Signatures) > 0 {
		tx.Signatures = make([]ValidatorSign, len(Signatures))
		copy(tx.Signatures, Signatures)
	}

	return tx
}

func GenMultiSignBytes(signInfo MultiSignMainInfo) ([]byte, error) {
	bz, err := ser.EncodeToBytes(signInfo)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func (tx *MultiSignAccountTx) From() (common.Address, error) {
	return MultiSignNonceAddr, nil
}

func (tx *MultiSignAccountTx) To() *common.Address {
	return nil
}

func (tx *MultiSignAccountTx) Nonce() uint64 {
	return tx.MultiSignMainInfo.AccountNonce
}

// verify whether Signatures is 2/3 valid and save the  Signers
func (tx *MultiSignAccountTx) VerifySign(validators *ValidatorSet) (err error) {
	defer func() {
		if err != nil {
			logger.Report("MultiSignAccountTx VerifySign failed", "logID", LogIdSpecTxCheckError, "txHash", tx.Hash().Hex(), "err", err)
		}
	}()

	if validators == nil || validators.Size() == 0 {
		err = fmt.Errorf("validators is nil or size=0")
		return
	}

	var signBytes []byte
	if signBytes, err = GenMultiSignBytes(tx.MultiSignMainInfo); err != nil {
		return
	}

	var totalVotingPower int64
	var sig crypto.Signature
	vaddrMap := make(map[string]bool, 10)
	for _, signature := range tx.Signatures {
		if _, exists := vaddrMap[string(signature.Addr)]; exists {
			err = fmt.Errorf("duplicate signature for %x", signature.Addr)
			return
		}
		val := validators.FindAddress(signature.Addr)
		if val == nil {
			err = fmt.Errorf("invalid validator %x", signature.Addr)
			return
		}
		if sig, err = crypto.SignatureFromBytes(signature.Signature); err != nil {
			return
		}

		if !val.PubKey.VerifyBytes(signBytes, sig) {
			continue
		}
		// Good signature!
		vaddrMap[string(signature.Addr)] = true
		totalVotingPower += val.VotingPower
		if totalVotingPower > validators.TotalVotingPower()*2/3 {
			err = nil
			return
		}
	}

	err = fmt.Errorf("insufficient voting power: got %v, needed %v", totalVotingPower, (validators.TotalVotingPower()*2/3 + 1))
	return
}

func (tx *MultiSignAccountTx) Hash() common.Hash {
	hash := transactionHash(&tx.hash, tx)
	return hash
}

func (tx *MultiSignAccountTx) TypeName() string {
	return TxMultiSignAccount
}

func (tx *MultiSignAccountTx) String() string {
	if tx == nil {
		return "<nil>"
	}

	signersData := make([]string, 0)
	sigData := make([]string, 0)

	signersData = append(signersData, fmt.Sprintf("MinSignerPower:%v\n", tx.MultiSignMainInfo.MinSignerPower))
	for i := 0; i < len(tx.MultiSignMainInfo.Signers); i++ {
		signersData = append(signersData, fmt.Sprintf("Power[%d]:%v Addr[%d]:%s\n", i, tx.MultiSignMainInfo.Signers[i].Power, i, tx.MultiSignMainInfo.Signers[i].Addr.String()))
	}
	mainInfoData := fmt.Sprintf("AccountNonce:%v\n SupportTxType:%v\n SignersInfo:%v", tx.AccountNonce, tx.SupportTxType, signersData)
	for i := 0; i < len(tx.Signatures); i++ {
		sigData = append(sigData, fmt.Sprintf("Addr[%d]:0x%x\n", i, tx.Signatures[i].Addr))
	}

	return fmt.Sprintf("TxHash:(0x%x)\n type:%s\n MultiSignMainInfo:%v\n Signatures:%v",
		tx.Hash(), tx.TypeName(), mainInfoData, sigData,
	)

}

func (tx *MultiSignAccountTx) CheckBasic(censor TxCensor) error {
	if IsTestMode {
		return nil
	}

	_, vals := censor.GetLastChangedVals()
	valSets := NewValidatorSet(vals)
	return tx.VerifySign(valSets)
}

func (tx *MultiSignAccountTx) CheckState(censor TxCensor) error {
	censor.LockState()
	defer censor.UnlockState()
	state := censor.State()
	// Check if nonce is not strictly increasing
	from, _ := tx.From()
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

func (tx *MultiSignAccountTx) Sign(prv PrivValidator) error {
	cpy := &MultiSignAccountTx{MultiSignMainInfo: tx.MultiSignMainInfo}
	sigs := make([]ValidatorSign, 0, len(tx.Signatures)+1)
	for _, sig := range tx.Signatures {
		sigs = append(sigs, ValidatorSign{
			Addr:      sig.Addr[:],
			Signature: sig.Signature[:],
		})
	}

	signBytes, err := ser.EncodeToBytes(tx.MultiSignMainInfo)
	if err != nil {
		return err
	}

	signature, err := prv.SignData(signBytes)
	if err != nil {
		return err
	}
	sigs = append(sigs, ValidatorSign{Addr: []byte(prv.GetAddress())[:], Signature: signature[:]})
	cpy.Signatures = sigs
	*tx = *cpy
	return nil
}
