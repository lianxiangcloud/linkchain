package blockchain

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/libs/txmgr"

	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/types"
)

/*
BlockStore is a simple low level store for blocks.

There are three types of information stored:
 - BlockMeta:   Meta information about each block
 - Block part:  Parts of each block, aggregated w/ PartSet
 - Commit:      The commit part of each block, for gossiping precommit votes

Currently the precommit signatures are duplicated in the Block parts as
well as the Commit.  In the future this may change, perhaps by moving
the Commit data outside the Block. (TODO)

// NOTE: BlockStore methods will panic if they encounter errors
// deserializing loaded data, indicating probable corruption on disk.
*/
type BlockStore struct {
	db         dbm.DB
	crossState txmgr.CrossState

	mtx    sync.RWMutex
	height uint64

	startDeleteHeight uint64
}

var _ txmgr.IBlockStore = &BlockStore{}

// NewBlockStore returns a new BlockStore with the given DB,
// initialized to the last height that was committed to the DB.
func NewBlockStore(db dbm.DB) *BlockStore {
	bsjson := LoadBlockStoreStateJSON(db)
	return &BlockStore{
		height: bsjson.Height,
		db:     db,

		startDeleteHeight: 0,
	}
}

//SetCrossState set crossState to crossState.
func (bs *BlockStore) SetCrossState(crossState txmgr.CrossState) {
	bs.crossState = crossState
}

func (bs *BlockStore) RollBackOneBlock() {
	// Save new BlockStoreStateJSON descriptor
	bs.height -= 1
	BlockStoreStateJSON{Height: bs.height}.Save(bs.db)
}

// GetDB return the db interface.
func (bs *BlockStore) GetDB() dbm.DB {
	return bs.db
}

// Height returns the last known contiguous block height.
func (bs *BlockStore) Height() uint64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.height
}

var initBlockHeightKey = []byte("BINITH")

func (bs *BlockStore) SaveInitHeight(height uint64) {
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	bs.db.SetSync(initBlockHeightKey, heightBytes)
}

func (bs *BlockStore) LoadInitHeight() (uint64, error) {
	h, err := bs.db.Load(initBlockHeightKey)
	if err != nil {
		return 0, err
	}
	if len(h) < 8 {
		return 0, fmt.Errorf("init height(%x) is not a uint64", h)
	}
	return binary.BigEndian.Uint64(h), nil
}

// LoadBlock returns the block with the given height.
// If no block is found for that height, it returns nil.
func (bs *BlockStore) LoadBlock(height uint64) *types.Block {
	var blockMeta = bs.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}
	var block = new(types.Block)
	buf := []byte{}
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		if part == nil {
			return nil
		}
		buf = append(buf, part.Bytes...)
	}
	err := ser.DecodeBytes(buf, block)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(cmn.ErrorWrap(err, "Error reading block"))
	}

	block.Header.SetBloom(blockMeta.Header.Bloom())
	return block
}

func (bs *BlockStore) LoadBlockAndMeta(height uint64) (*types.Block, *types.BlockMeta) {
	var blockMeta = bs.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil, nil
	}

	var buf []byte
	var block = new(types.Block)
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		if part == nil {
			return nil, blockMeta
		}
		buf = append(buf, part.Bytes...)
	}
	err := ser.DecodeBytes(buf, block)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(cmn.ErrorWrap(err, "Error reading block"))
	}
	block.Header.SetBloom(blockMeta.Header.Bloom())

	return block, blockMeta
}

// LoadBlock returns the block with the given hash.
// If no block is found for that hash, it returns nil.
func (bs *BlockStore) LoadBlockByHash(hash common.Hash) *types.Block {
	var blockMeta = bs.LoadBlockMetaByHash(hash)
	if blockMeta == nil {
		return nil
	}

	var block = new(types.Block)
	buf := []byte{}
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		part := bs.LoadBlockPart(blockMeta.Header.Height, i)
		if part == nil {
			return nil
		}
		buf = append(buf, part.Bytes...)
	}
	err := ser.DecodeBytes(buf, block)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(cmn.ErrorWrap(err, "Error reading block"))
	}

	block.Header.SetBloom(blockMeta.Header.Bloom())
	return block
}

// LoadBlockPart returns the Part at the given index
// from the block at the given height.
// If no part is found for the given height and index, it returns nil.
func (bs *BlockStore) LoadBlockPart(height uint64, index int) *types.Part {
	var part = new(types.Part)
	bz := bs.db.Get(calcBlockPartKey(height, index))
	if len(bz) == 0 {
		return nil
	}
	err := ser.DecodeBytes(bz, part)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block part"))
	}
	return part
}

// LoadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockMeta(height uint64) *types.BlockMeta {
	var blockMeta = new(types.BlockMeta)
	bz := bs.db.Get(calcBlockMetaKey(height))
	if len(bz) == 0 {
		return nil
	}
	err := ser.DecodeBytes(bz, blockMeta)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block meta"))
	}

	txResult, err := bs.LoadTxsResult(height)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading txs result"))
	}
	blockMeta.Header.SetBloom(txResult.LogsBloom)
	return blockMeta
}

// LoadBlockMeta returns the BlockMeta for the given hash.
// If no block is found for the given hash, it returns nil.
func (bs *BlockStore) LoadBlockMetaByHash(hash common.Hash) *types.BlockMeta {
	var blockMeta = new(types.BlockMeta)
	bz := bs.db.Get(calcBlockHashKey(hash))
	if len(bz) == 0 {
		return nil
	}
	err := ser.DecodeBytes(bz, blockMeta)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block meta"))
	}

	txResult, err := bs.LoadTxsResult(blockMeta.Header.Height)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading txs result"))
	}
	blockMeta.Header.SetBloom(txResult.LogsBloom)
	return blockMeta
}

// LoadBlockCommit returns the Commit for the given height.
// This commit consists of the +2/3 and other Precommit-votes for block at `height`,
// and it comes from the block.LastCommit for `height+1`.
// If no commit is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockCommit(height uint64) *types.Commit {
	var commit = new(types.Commit)
	bz := bs.db.Get(calcBlockCommitKey(height))
	if len(bz) == 0 {
		return nil
	}
	err := ser.DecodeBytes(bz, commit)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block commit"))
	}
	return commit
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bs *BlockStore) LoadSeenCommit(height uint64) *types.Commit {
	var commit = new(types.Commit)
	bz := bs.db.Get(calcSeenCommitKey(height))
	if len(bz) == 0 {
		return nil
	}
	err := ser.DecodeBytes(bz, commit)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block seen commit"))
	}
	return commit
}

// LoadTxsResult returns the process result of transactions for the given height.
func (bs *BlockStore) LoadTxsResult(height uint64) (*types.TxsResult, error) {
	var result = new(types.TxsResult)
	bz := bs.db.Get(calcTxsResultKey(height))
	if len(bz) == 0 {
		return nil, types.ErrGetTxsResult
	}
	err := ser.DecodeBytes(bz, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// blockParts: Must be parts of the block
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit, receipts *types.Receipts, txsResult *types.TxsResult) {
	if block == nil {
		cmn.PanicSanity("BlockStore can only save a non-nil block")
	}

	height := block.Height
	if height > types.BlockHeightZero {
		if g, w := height, bs.Height()+1; g != w {
			cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
		}
	}

	if !blockParts.IsComplete() {
		cmn.PanicSanity(cmn.Fmt("BlockStore can only save complete block part sets"))
	}

	bsBatch := bs.db.NewBatch()
	// Save block parts
	for i := 0; i < blockParts.Total(); i++ {
		part := blockParts.GetPart(i)
		bs.saveBlockPart(height, i, part, bsBatch)
	}

	// Save block commit (duplicate and separate from the Block)
	if height > types.BlockHeightZero {
		blockCommitBytes := ser.MustEncodeToBytes(block.LastCommit)
		bsBatch.Set(calcBlockCommitKey(height-1), blockCommitBytes)
	}

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	if seenCommit != nil || height > types.BlockHeightZero {
		seenCommitBytes := ser.MustEncodeToBytes(seenCommit)
		bsBatch.Set(calcSeenCommitKey(height), seenCommitBytes)
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		// Save block Receipts
		bs.saveReceipts(height, receipts)
		wg.Done()
	}()

	go func() {
		// Save process result of transactions
		bs.saveTxsResult(height, txsResult)
		wg.Done()
	}()

	go func() {
		if bs.crossState != nil {
			// Save index
			bs.crossState.SaveTxEntry(block, txsResult)
			// Save specialtx
			bs.crossState.AddSpecialTx(txsResult.SpecialTxs())
			// flush
			bs.crossState.Sync()
		}
		wg.Done()
	}()

	wg.Wait()

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	metaBytes := ser.MustEncodeToBytes(blockMeta)
	bsBatch.Set(calcBlockMetaKey(height), metaBytes)
	bsBatch.Set(calcBlockHashKey(block.Hash()), metaBytes)

	// Commit block store db batch
	bsBatch.Commit()

	// Save new BlockStoreStateJSON descriptor
	BlockStoreStateJSON{Height: height}.Save(bs.db)

	// Done!
	bs.mtx.Lock()
	bs.height = height
	bs.mtx.Unlock()

	// Flush
	bs.db.SetSync(nil, nil)
}

func (bs *BlockStore) saveBlockPart(height uint64, index int, part *types.Part, bsBatch dbm.Batch) {
	//if height > types.BlockHeightZero && height != bs.Height()+1 {
	//	cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	//}
	partBytes := ser.MustEncodeToBytes(part)
	bsBatch.Set(calcBlockPartKey(height, index), partBytes)
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func (bs *BlockStore) saveReceipts(height uint64, receipts *types.Receipts) {
	//if height > types.BlockHeightZero && height != bs.Height()+1 {
	//	cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous receipts. Wanted %v, got %v", bs.Height()+1, height))
	//}
	if receipts == nil {
		//cmn.PanicSanity("BlockStore can only save a non-nil receipts")
		return
	}

	storageReceipts := make([]*types.ReceiptForStorage, len(*receipts))
	for index, r := range *receipts {
		storageReceipts[index] = r.ForStorage()
	}

	data, err := ser.EncodeToBytes(storageReceipts)
	if err != nil {
		cmn.PanicSanity(fmt.Sprintf("Failed to encode block receipts err:%v", err))
	}
	bs.db.Set(calcBlockReceiptsKey(height), data)
}

// GetReceipts return all the transaction receipts belonging to a block.
func (bs *BlockStore) GetReceipts(height uint64) *types.Receipts {
	data := bs.db.Get(calcBlockReceiptsKey(height))
	if len(data) == 0 {
		return nil
	}

	storageReceipts := make([]*types.ReceiptForStorage, 0)
	if err := ser.DecodeBytes(data, &storageReceipts); err != nil {
		log.Error("Invalid receipt array SER", "height", height, "err", err)
		return nil
	}

	receipts := make([]*types.Receipt, len(storageReceipts))
	for index, s := range storageReceipts {
		receipts[index] = s.ToReceipt()
	}

	tmp := (types.Receipts)(receipts)
	return &tmp
}

// GetHeader returns the header for the given height.
// If not found for the given height, it returns nil.
func (bs *BlockStore) GetHeader(height uint64) *types.Header {
	blockMeta := bs.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}
	return blockMeta.Header
}

// saveTxsResult stores all the transaction process result.
func (bs *BlockStore) saveTxsResult(height uint64, txsResult *types.TxsResult) {
	//if height > types.BlockHeightZero && height != bs.Height()+1 {
	//	cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous txsResult. Wanted %v, got %v", bs.Height()+1, height))
	//}
	if txsResult == nil {
		cmn.PanicSanity("BlockStore can only save a non-nil txsResult")
	}
	data := ser.MustEncodeToBytes(txsResult)
	bs.db.Set(calcTxsResultKey(height), data)
}

//GetTxFromBlock iterator Txs from block to find tx with hash.
func (bs *BlockStore) GetTxFromBlock(block *types.Block, hash common.Hash) types.Tx {
	for _, tx := range block.Txs {
		if tx.Hash() == hash {
			return tx
		}
	}
	return nil
}

func (bs *BlockStore) GetTx(hash common.Hash) (types.Tx, *types.TxEntry) { //common.Hash, uint64, uint64) {
	entry := bs.crossState.GetTxEntry(hash)
	if entry == nil {
		log.Debug("BlockStore GetTx: GetTxEntry nil", "hash", hash.String())
		return nil, nil
	}

	block := bs.LoadBlock(entry.BlockHeight)
	if block == nil {
		log.Warn("BlockStore GetTx: LoadBlock fail", "height", entry.BlockHeight, "hash", entry.BlockHash.String())
		return nil, entry
	}

	if entry.Index >= uint64(len(block.Txs)) {
		log.Warn("BlockStore GetTx: invalid tx entry", "entry", entry, "hash", hash.String())
		return nil, entry
	}
	tx := block.Txs[entry.Index]
	return tx, entry
}

func (bs *BlockStore) GetTransactionReceipt(hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64) {
	entry := bs.crossState.GetTxEntry(hash)
	if entry == nil {
		log.Debug("BlockStore GetTransactionReceipt: GetTxEntry nil", "hash", hash.String())
		return nil, common.EmptyHash, 0, 0
	}

	receipts := bs.GetReceipts(entry.BlockHeight)
	if receipts == nil {
		log.Warn("BlockStore GetTransactionReceipt: GetReceipts fail", "height", entry.BlockHeight, "block_hash", entry.BlockHash.String())
		return nil, common.EmptyHash, 0, 0
	}

	if entry.Index >= uint64(len(*receipts)) {
		log.Warn("BlockStore GetTransactionReceipt: invalid tx entry", "entry", entry, "hash", hash.String())
		return nil, common.EmptyHash, 0, 0
	}

	rr := *receipts
	return rr[entry.Index], entry.BlockHash, entry.BlockHeight, entry.Index
}

func (bs *BlockStore) deleteBlock(height uint64) (nTxs int, err error) {
	block, blockMeta := bs.LoadBlockAndMeta(height)
	if blockMeta == nil || blockMeta.Header == nil || block == nil {
		return 0, nil
	}

	if bs.crossState != nil {
		bs.crossState.DeleteTxEntry(block)
	}

	nTxs = len(block.Data.Txs) + 1
	bat := bs.db.NewBatch()

	//bat.Delete(calcBlockMetaKey(height)) // keep for evm
	//bat.Delete(calcTxsResultKey(height)) // keep for evm
	bat.Delete(calcBlockHashKey(blockMeta.BlockID.Hash))
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		bat.Delete(calcBlockPartKey(height, i))
	}
	bat.Delete(calcBlockCommitKey(height - 1))
	bat.Delete(calcSeenCommitKey(height))
	bat.Delete(calcBlockReceiptsKey(height))

	return nTxs, bat.Commit()
}

func (bs *BlockStore) DeleteHistoricalData(keepLatestBlocks uint64) {
	minHeight := bs.startDeleteHeight
	if minHeight == 0 {
		minHeight = loadStartDeleteHeight(bs.db)
		bs.startDeleteHeight = minHeight
	}
	maxHeight := bs.Height()
	if maxHeight-keepLatestBlocks < minHeight {
		return
	}

	log.Info("deleteHistoricalData: in blockChain and txmgr", "minHeight", minHeight, "maxHeight", maxHeight)

	for total := 0; minHeight <= maxHeight-keepLatestBlocks; minHeight++ {
		n, err := bs.deleteBlock(minHeight)
		if err != nil {
			log.Warn("deleteHistoricalData: ", "height", minHeight, "err", err)
		}
		total += n
		if total > 3000 {
			total = 0
			time.Sleep(59 * time.Millisecond)
		}
	}

	bs.startDeleteHeight = minHeight
	saveStartDeleteHeight(bs.db, minHeight)
}

func (bs *BlockStore) HaveTxKeyimgAsSpent(keyImage lktypes.Key) bool {
	return false
}

func (bs *BlockStore) IsTxSpendTimeUnlocked(unlockTime uint64) bool {
	return true
}

func (bs *BlockStore) GetOutputKey(amount *big.Int, index uint64, includeCommitment bool) types.UTXOOutputData {
	var output types.UTXOOutputData
	return output
}

func (bs *BlockStore) GetOutputKeys(amounts []*big.Int, offsets []uint64, outputs []types.UTXOOutputData, allowPartial bool) {
}

//-----------------------------------------------------------------------------

func calcBlockMetaKey(height uint64) []byte {
	return []byte(fmt.Sprintf("BM:%v", height))
}

func calcBlockHashKey(hash common.Hash) []byte {
	return append(blockHashKey, hash.Bytes()...)
}

func calcBlockPartKey(height uint64, partIndex int) []byte {
	return []byte(fmt.Sprintf("BP:%v:%v", height, partIndex))
}

func calcBlockCommitKey(height uint64) []byte {
	return []byte(fmt.Sprintf("BC:%v", height))
}

func calcSeenCommitKey(height uint64) []byte {
	return []byte(fmt.Sprintf("BSC:%v", height))
}

func calcBlockReceiptsKey(height uint64) []byte {
	return []byte(fmt.Sprintf("BR:%v", height))
}

func calcTxsResultKey(height uint64) []byte {
	return []byte(fmt.Sprintf("BTR:%v", height))
}

//-----------------------------------------------------------------------------

var blockHashKey = []byte("BH:")
var blockStoreKey = []byte("blockStore")

type BlockStoreStateJSON struct {
	Height uint64 `json:"height"`
}

// Save persists the blockStore state to the database as JSON.
func (bsj BlockStoreStateJSON) Save(db dbm.DB) {
	bytes, err := ser.MarshalJSON(bsj)
	if err != nil {
		cmn.PanicSanity(cmn.Fmt("Could not marshal state bytes: %v", err))
	}
	db.Set(blockStoreKey, bytes)
}

// LoadBlockStoreStateJSON returns the BlockStoreStateJSON as loaded from disk.
// If no BlockStoreStateJSON was previously persisted, it returns the zero value.
func LoadBlockStoreStateJSON(db dbm.DB) BlockStoreStateJSON {
	bytes := db.Get(blockStoreKey)
	if len(bytes) == 0 {
		return BlockStoreStateJSON{
			Height: types.BlockHeightZero,
		}
	}
	bsj := BlockStoreStateJSON{}
	err := ser.UnmarshalJSON(bytes, &bsj)
	if err != nil {
		panic(fmt.Sprintf("Could not unmarshal bytes: %X", bytes))
	}
	return bsj
}

var startDeleteHeight = []byte("BCSDH")
var heightBytes = make([]byte, 8)

func saveStartDeleteHeight(db dbm.DB, height uint64) {
	binary.BigEndian.PutUint64(heightBytes, height)
	db.Set(startDeleteHeight, heightBytes)
}

func loadStartDeleteHeight(db dbm.DB) uint64 {
	h, err := db.Load(startDeleteHeight)
	if err == nil && len(h) != 0 {
		return binary.BigEndian.Uint64(h)
	}
	return 1
}

//---------------------------------------------------------------------------
