package types

import (
	"fmt"
	"math/big"
)

// All transaction type used
const (
	//-----normal tx type
	TxNormal = "tx"
	TxToken  = "txt"

	TxMultiSignAccount = "mst"
	TxContractCreate   = "cct"
	TxContractUpgrade  = "cut"
	TxUTXO             = "utx"

	// balance record
	TxTransfer       = "transfer"
	TxContract       = "contract"
	TxCreateContract = "create_contract"
	TxSuicide        = "suicide"
	TxFee            = "fee"

	//----UTXO input/output type
	InUTXO       = "utin"
	InAc         = "acin"
	InMine       = "minein"
	OutUTXO      = "utout"
	OutAc        = "acout"
	TypeUTXODest = "utxodest"
	TypeAcDest   = "acdest"
)

type NodeType int

// All node type used
const (
	NodeValidator NodeType = 5
	NodePeer      NodeType = 6
)

func (nodeType NodeType) String() string {
	var printType string
	switch nodeType {
	case NodeValidator:
		printType = "NodeValidator"
	case NodePeer:
		printType = "NodePeer"
	default:
		printType = fmt.Sprintf("%d", nodeType)
	}
	return fmt.Sprintf("%s", printType)
}

const (
	PubNetSignParam int64 = 29153
	TestNetSignParam int64 = 29154
)

// SignParam is const param which used to check transaction's sign is correct or not
var (
	SignParam       = big.NewInt(PubNetSignParam)
	GlobalSTDSigner = MakeSTDSigner(nil)
)

func InitSignParam(TestNet bool) {
	if TestNet {
		SignParam = big.NewInt(TestNetSignParam)
		GlobalSTDSigner = MakeSTDSigner(nil)
	}
}

func IsNormalTx(tx Tx) bool {
	return tx.TypeName() == TxNormal
}

type LogId uint64

const (
	LogIdBlockTimeError   LogId = 30003
	LogIdIllegalValidator LogId = 30006

	LogIdContractExecutionError LogId = 70000
	LogIdHeight                 LogId = 70007
	LogIdCommitBlockFail        LogId = 70008
	LogIdSyncBlockCheckError    LogId = 70009
	LogIdFastSyncBlockTimeOut   LogId = 70010 //No need
	LogIdSpecTxCheckError       LogId = 70019
	LogIdTooManyRetransTx       LogId = 70020
	LogIdBootNodeFail           LogId = 70021
	LogIdSyncBlockFail          LogId = 70022
)
