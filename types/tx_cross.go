package types

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"
)

var (
	_ Tx       = &Transaction{}
	_ Tx       = &TokenTransaction{}
	_ Tx       = &ContractCreateTx{}
	_ Tx       = &ContractUpgradeTx{}
	_ Tx       = &MultiSignAccountTx{}
	_ IMessage = &Transaction{}
	_ IMessage = &TokenTransaction{}
	_ IMessage = &ContractCreateTx{}
	_ IMessage = &ContractUpgradeTx{}
)

type SupportType int

const (
	TxUpdateValidatorsType SupportType = iota
	TxContractCreateType
)

func (txType SupportType) String() string {
	var printTxType string
	switch txType {
	case TxUpdateValidatorsType:
		printTxType = "TxUpdateValidators"
	case TxContractCreateType:
		printTxType = "TxContractCreate"
	}
	return fmt.Sprintf("%s", printTxType)
}

const (
	DBupdateValidatorsKey = "multisign_updatevalidators"
	DBcontractCreateKey   = "multisign_contractcreate"
)

type TokenValue struct {
	TokenAddr common.Address `json:"tokenAddress"`
	Value     *big.Int       `json:"value"`
}

type TokenValues []TokenValue

func (t TokenValues) Len() int {
	return len(t)
}

func (t TokenValues) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t TokenValues) Less(i, j int) bool {
	return t[i].TokenAddr.String() < t[j].TokenAddr.String()
}

func signToBytes(signature *signdata, prv *ecdsa.PrivateKey) ([]byte, error) {
	r, s, v, err := sign(GlobalSTDSigner, prv, signature.signFields())
	if err != nil {
		log.Warn("SignTogether sign failed", "prv", prv, "err", err)
		return nil, err
	}
	signature.R, signature.S, signature.V = r, s, v
	sigData, err := ser.EncodeToBytes(signature)
	if err != nil {
		log.Error("sign EncodeToBytes failed", "err", err)
		return nil, err
	}
	return sigData, nil
}

func initSignature(signatures []*signdata, sigData [][]byte) error {
	for i := 0; i < len(sigData); i++ {
		signature := &signdata{}
		err := ser.DecodeBytes(sigData[i], signature)
		if err != nil {
			log.Error("initSignature DecodeBytes failed", "err", err)
			return err
		}
		signatures[i] = signature
	}
	return nil
}

func verifySignature(signatures []*signdata, savedSignersInfo *SignersInfo, signFields func() []interface{}) (err error) {
	if savedSignersInfo == nil || len(savedSignersInfo.Signers) == 0 {
		err = fmt.Errorf("VerifySignature signers is nil or no signers,len(signers):%v", len(savedSignersInfo.Signers))
		return err
	}
	if len(signatures) > len(savedSignersInfo.Signers) {
		err = fmt.Errorf("recved len(signature):%v > len(savedMultiSigners):%v", len(signatures), len(savedSignersInfo.Signers))
		return err
	}
	signerAddrMap := make(map[string]bool)
	var totalPower int32
	for i := 0; i < len(signatures); i++ {
		if signatures[i] == nil {
			continue
		}
		signatures[i].setSignFieldsFunc(signFields)
		fromAddr, err := sender(GlobalSTDSigner, signatures[i])
		if err != nil {
			log.Info("verifySenderSignature err", "i", i, "len(signatures)", len(signatures), "err", err)
			continue
		}
		log.Debug("verifySenderSignature", "fromAddr", fromAddr)
		_, ok := signerAddrMap[fromAddr.String()]
		if ok {
			continue
		}
		signerAddrMap[fromAddr.String()] = true
		for i := 0; i < len(savedSignersInfo.Signers); i++ {
			if fromAddr == savedSignersInfo.Signers[i].Addr {
				totalPower += savedSignersInfo.Signers[i].Power
				if totalPower >= savedSignersInfo.MinSignerPower {
					return nil
				}
			}
		}
	}
	err = fmt.Errorf("VerifySignature failed,totalPower:%v minPower:%v signers:%v", totalPower, savedSignersInfo.MinSignerPower, savedSignersInfo.Signers)
	return err
}

func verifySenderSignature(senderAddr common.Address, signatures []*signdata, savedSignersInfo *SignersInfo, signFields func() []interface{}) (err error) {
	if savedSignersInfo == nil || len(savedSignersInfo.Signers) == 0 {
		err = fmt.Errorf("VerifySignature signers is nil or no signers,len(signers):%v", len(savedSignersInfo.Signers))
		return err
	}
	if len(signatures) > len(savedSignersInfo.Signers)+1 { //1 means senderAddr's signature
		err = fmt.Errorf("recved len(signature):%v > len(savedMultiSigners)+1:%v", len(signatures), len(savedSignersInfo.Signers)+1)
		return err
	}
	signerAddrMap := make(map[string]bool)
	var totalPower int32
	matchFromFlag := false
	matchPowerFlag := false
	for i := 0; i < len(signatures); i++ {
		if signatures[i] == nil {
			continue
		}
		signatures[i].setSignFieldsFunc(signFields)
		fromAddr, err := sender(GlobalSTDSigner, signatures[i])
		if err != nil {
			log.Info("verifySenderSignature err", "i", i, "len(signatures)", len(signatures), "err", err)
			continue
		}
		log.Debug("verifySenderSignature", "fromAddr", fromAddr)
		_, ok := signerAddrMap[fromAddr.String()]
		if ok {
			continue
		}
		signerAddrMap[fromAddr.String()] = true
		for _, s := range savedSignersInfo.Signers {
			if fromAddr == s.Addr {
				totalPower += s.Power
			}
		}
		if fromAddr == senderAddr {
			matchFromFlag = true
		}
		if matchFromFlag && totalPower >= savedSignersInfo.MinSignerPower {
			return nil
		}
	}
	err = fmt.Errorf("VerifySenderSignature failed,totalPower:%v minPower:%v matchFromFlag:%v matchPowerFlag:%v signers:%v", totalPower, savedSignersInfo.MinSignerPower, matchFromFlag, matchPowerFlag, savedSignersInfo.Signers)
	return err
}

func verifySpecialTx(signersInfo *SignersInfo, signatures []*signdata, signFields func() []interface{}) error {
	return verifySignature(signatures, signersInfo, signFields)
}

func verifySpecialTxWithSender(senderAddr common.Address, signersInfo *SignersInfo, signatures []*signdata, signFields func() []interface{}) error {
	return verifySenderSignature(senderAddr, signatures, signersInfo, signFields)
}

//----------
