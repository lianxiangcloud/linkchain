package consensus

import (
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/types"
)

//------------------------------------------------------
// mempool

// Mempool defines the mempool interface as used by the ConsensusState.
// Updates to the mempool need to be synchronized with committing a block
// so apps can reset their transient state on Commit
type Mempool interface {
	Lock()
	Unlock()

	GoodTxsSize() int
	Reap(int) types.Txs
	Update(height uint64, txs types.Txs) error

	SetReceiveP2pTx(on bool)

	TxsAvailable() <-chan struct{}
	EnableTxsAvailable()
}

// MockMempool is an empty implementation of a Mempool, useful for testing.
type MockMempool struct {
}

func (m MockMempool) Lock()            {}
func (m MockMempool) Unlock()          {}
func (m MockMempool) GoodTxsSize() int { return 0 }
func (m MockMempool) Reap(n int) types.Txs {
	return types.Txs{}
}
func (m MockMempool) Update(height uint64, txs types.Txs) error { return nil }
func (m MockMempool) Flush()                                    {}
func (m MockMempool) FlushAppConn() error                       { return nil }
func (m MockMempool) TxsAvailable() <-chan struct{}             { return make(chan struct{}) }
func (m MockMempool) EnableTxsAvailable()                       {}
func (m MockMempool) GetTxFromCache(hash common.Hash) types.Tx {
	return nil
}
func (m MockMempool) SetReceiveP2pTx(on bool) {}

//-----------------------------------------------------------------------------------------------------
// evidence pool

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
type EvidencePool interface {
	PendingEvidence() []types.Evidence
	AddEvidence(types.Evidence) error
	Update(*types.Block, NewStatus)
}

// MockMempool is an empty implementation of a Mempool, useful for testing.
type MockEvidencePool struct {
}

func (m MockEvidencePool) PendingEvidence() []types.Evidence { return nil }
func (m MockEvidencePool) AddEvidence(types.Evidence) error  { return nil }
func (m MockEvidencePool) Update(*types.Block, NewStatus)    {}

//-----------------------------------------------------------------------------------------------------
// BlockChainApp

// BlockChainApp is the block manage interface.
type BlockChainApp interface {
	Height() uint64

	LoadBlockMeta(height uint64) *types.BlockMeta
	LoadBlock(height uint64) *types.Block
	LoadBlockPart(height uint64, index int) *types.Part

	LoadBlockCommit(height uint64) *types.Commit
	LoadSeenCommit(height uint64) *types.Commit
	GetValidators(height uint64) []*types.Validator
	GetRecoverValidators(height uint64) []*types.Validator

	CreateBlock(height uint64, maxTxs int, gasLimit uint64, timeUinx uint64) *types.Block
	PreRunBlock(block *types.Block)
	CheckBlock(block *types.Block) bool
	// CheckProcessResult(blockHash common.Hash, txsResult *types.TxsResult) bool
	CommitBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit, fastsync bool) ([]*types.Validator, error)

	SetLastChangedVals(height uint64, vals []*types.Validator)
}
