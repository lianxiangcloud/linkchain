package types

import (
	"math/big"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
)

type TxMgr interface {
	GetMultiSignersInfo(txType SupportType) *SignersInfo
}

type State interface {
	Exist(addr common.Address) bool
	GetNonce(addr common.Address) uint64
	SetNonce(addr common.Address, nonce uint64)
	GetBalance(addr common.Address) *big.Int
	SubBalance(addr common.Address, amount *big.Int)
	GetTokenBalance(addr common.Address, token common.Address) *big.Int
	SubTokenBalance(addr common.Address, token common.Address, amount *big.Int)
	IsContract(addr common.Address) bool
}

type BlockChain interface {
	//is_tx_spendtime_unlocked
	IsTxSpendTimeUnlocked(unlockTime uint64) bool
}

type UTXOStore interface {
	GetUtxoOutput(tokenId common.Address, seq uint64) (*UTXOOutputData, error)
	GetUtxoOutputs(seqs []uint64, tokenId common.Address) ([]*UTXOOutputData, error)
	HaveTxKeyimgAsSpent(keyImg *types.Key) bool
}

type Mempool interface {
	Reap(maxTxs int) Txs
	Update(height uint64, txs Txs) error
	GetTxFromCache(common.Hash) Tx
	Lock()
	Unlock()
	KeyImageExists(key types.Key) bool
	KeyImagePush(key types.Key) bool
	KeyImageRemoveKeys([]*types.Key)
	KeyImageReset()
}

type TxCensor interface {
	TxMgr() TxMgr
	State() State
	Block() *Block
	GetLastChangedVals() (height uint64, vals []*Validator)
	LockState()
	UnlockState()
	IsWasmContract(data []byte) bool
	BlockChain() BlockChain
	UTXOStore() UTXOStore
	Mempool() Mempool
	GetUTXOGas() uint64
}
