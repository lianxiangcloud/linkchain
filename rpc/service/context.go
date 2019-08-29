package service

import (
	"math/big"

	"github.com/lianxiangcloud/linkchain/accounts"
	cs "github.com/lianxiangcloud/linkchain/consensus"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/libs/txmgr"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
)

type App interface {
	GetNonce(addr common.Address) uint64
	GetBalance(addr common.Address) *big.Int
	GetPendingStateDB() *state.StateDB
	GetLatestStateDB() *state.StateDB
	GetPendingBlock() *types.Block
	GetUTXOGas() uint64
}

type Mempool interface {
	AddTx(peerID string, tx types.Tx) error
	//PendingTxs(nums int) (types.Txs, error)
	Stats() (int, int, int)
}

type Consensus interface {
	GetState() cs.NewStatus
	GetValidators() (uint64, []*types.Validator)
	GetRoundStateJSON() ([]byte, error)
	GetRoundStateSimpleJSON() ([]byte, error)
}

type BlockStore interface {
	Height() uint64
	LoadBlockMeta(height uint64) *types.BlockMeta
	LoadTxsResult(height uint64) (*types.TxsResult, error)
	LoadBlock(height uint64) *types.Block
	GetDB() dbm.DB
	//GetTx(hash common.Hash) (types.Tx, common.Hash, uint64, uint64)
	GetTx(hash common.Hash) (types.Tx, *types.TxEntry)
	GetTransactionReceipt(hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64)
	GetHeader(height uint64) *types.Header
	LoadBlockByHash(hash common.Hash) *types.Block
	LoadBlockMetaByHash(hash common.Hash) *types.BlockMeta
	GetReceipts(height uint64) *types.Receipts
}

type BalanceRecordStore interface {
	Save(blockHeight uint64, bbr *types.BlockBalanceRecords)
	Get(blockHeight uint64) *types.BlockBalanceRecords
}

//UtxoStore utxo storage
type UtxoStore interface {
	GetUtxoOutput(token common.Address, index uint64) (*types.UTXOOutputData, error)
	GetMaxUtxoOutputSeq(token common.Address) int64
	GetBlockTokenUtxoOutputSeq(blockHeight uint64) map[string]int64
}

type Context struct {
	// interface
	logger     log.Logger
	stateDB    dbm.DB
	blockStore BlockStore
	brs        BalanceRecordStore
	mempool    Mempool
	app        App
	triedb     state.Database
	utxo       UtxoStore

	pubKey    crypto.PubKey
	p2pSwitch *p2p.Switch

	consensusState   Consensus
	consensusReactor *cs.ConsensusReactor

	// objects
	accManager *accounts.Manager
	eventBus   *types.EventBus // thread safe
	txService  *txmgr.Service
}

func NewContext() *Context {
	return &Context{}
}

func (c *Context) SetTxService(txSrv *txmgr.Service) {
	c.txService = txSrv
}

func (c *Context) SetTrieDB(db dbm.DB, isTrie bool) {
	c.triedb = state.NewKeyValueDBWithCache(db, 0, isTrie, 0)
}

func (c *Context) SetStateDB(db dbm.DB) {
	c.stateDB = db
}

func (c *Context) SetPubKey(pk crypto.PubKey) {
	c.pubKey = pk
}

func (c *Context) SetSwitch(sw *p2p.Switch) {
	c.p2pSwitch = sw
}

func (c *Context) SetConsensus(cs Consensus) {
	c.consensusState = cs
}

func (c *Context) SetConsensusReactor(conR *cs.ConsensusReactor) {
	c.consensusReactor = conR
}

func (c *Context) GetConsensusReactor() *cs.ConsensusReactor {
	return c.consensusReactor
}

func (c *Context) SetBlockstore(bs BlockStore) {
	c.blockStore = bs
}

func (c *Context) SetBalanceRecordStore(brs BalanceRecordStore) {
	c.brs = brs
}

func (c *Context) SetMempool(mem Mempool) {
	c.mempool = mem
}

func (c *Context) GetMempool() Mempool {
	return c.mempool
}

func (c *Context) SetApp(app App) {
	c.app = app
}

func (c *Context) SetUTXO(utxo UtxoStore) {
	c.utxo = utxo
}

func (c *Context) SetEventBus(eb *types.EventBus) {
	c.eventBus = eb
}

func (c *Context) SetAccountManager(am *accounts.Manager) {
	c.accManager = am
}

func (c *Context) GetCoinBase() common.Address {
	//TODO set coinbase
	return common.EmptyAddress
}

func (c *Context) SetLogger(logger log.Logger) {
	c.logger = logger.With("module", "rpc.service")
}
