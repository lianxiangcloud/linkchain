package blockchain

import (
	"fmt"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

func TestInitHeight(t *testing.T) {
	db := db.NewMemDB()
	bs := NewBlockStore(db)
	h, err := bs.LoadInitHeight()
	assert.Equal(t, "init height() is not a uint64", err.Error())

	db.SetSync(initBlockHeightKey, []byte{1})
	h, err = bs.LoadInitHeight()
	assert.Equal(t, "init height(01) is not a uint64", err.Error())

	bs.SaveInitHeight(1)
	h, err = bs.LoadInitHeight()
	assert.Equal(t, uint64(1), h)
	assert.Nil(t, err)
}

func TestLoadBlockStoreStateJSON(t *testing.T) {
	db := db.NewMemDB()

	bsj := &BlockStoreStateJSON{Height: 1000}
	bsj.Save(db)

	retrBSJ := LoadBlockStoreStateJSON(db)

	assert.Equal(t, *bsj, retrBSJ, "expected the retrieved DBs to match")
}

func TestNewBlockStore(t *testing.T) {
	db := db.NewMemDB()
	db.Set(blockStoreKey, []byte(`{"height": "10000"}`))
	bs := NewBlockStore(db)
	require.Equal(t, uint64(10000), bs.Height(), "failed to properly parse blockstore")

	panicCausers := []struct {
		data    []byte
		wantErr string
	}{
		{[]byte("artful-doger"), "not unmarshal bytes"},
		{[]byte(" "), "unmarshal bytes"},
	}

	for i, tt := range panicCausers {
		// Expecting a panic here on trying to parse an invalid blockStore
		_, _, panicErr := doFn(func() (interface{}, error) {
			db.Set(blockStoreKey, tt.data)
			_ = NewBlockStore(db)
			return nil, nil
		})
		require.NotNil(t, panicErr, "#%d panicCauser: %q expected a panic", i, tt.data)
		assert.Contains(t, panicErr.Error(), tt.wantErr, "#%d data: %q", i, tt.data)
	}

	db.Set(blockStoreKey, nil)
	bs = NewBlockStore(db)
	assert.Equal(t, bs.Height(), uint64(0), "expecting nil bytes to be unmarshaled alright")
}

func freshBlockStore() (*BlockStore, db.DB) {
	db := db.NewMemDB()
	return NewBlockStore(db), db
}

var (
	//state, _ = makeStateAndBlockStore()

	block       = makeBlock(1)
	partSet     = block.MakePartSet(2)
	part1       = partSet.GetPart(0)
	part2       = partSet.GetPart(1)
	seenCommit1 = &types.Commit{Precommits: []*types.Vote{{Height: 10,
		Timestamp: time.Now().UTC()}}}
)

// TODO: This test should be simplified ...

func TestBlockStoreSaveLoadBlock(t *testing.T) {
	_, bs := initializeValidatorState(0)
	require.Equal(t, bs.Height(), uint64(0), "initially the height should be zero")

	// check there are no blocks at various heights
	noBlockHeights := []uint64{0, 1, 100, 1000, 2}
	for i, height := range noBlockHeights {
		if g := bs.LoadBlock(height); g != nil {
			t.Errorf("#%d: height(%d) got a block; want nil", i, height)
		}
	}

	// save a block
	block := makeBlock(bs.Height() + 1)
	validPartSet := block.MakePartSet(2)
	seenCommit := &types.Commit{Precommits: []*types.Vote{{Height: 10,
		Timestamp: time.Now().UTC()}}}
	txsResult := &types.TxsResult{}
	bs.SaveBlock(block, partSet, seenCommit, nil, txsResult)
	require.Equal(t, bs.Height(), block.Header.Height, "expecting the new height to be changed")

	incompletePartSet := types.NewPartSetFromHeader(types.PartSetHeader{Total: 2})
	uncontiguousPartSet := types.NewPartSetFromHeader(types.PartSetHeader{Total: 0})
	uncontiguousPartSet.AddPart(part2)

	header1 := types.Header{
		Height:  1,
		NumTxs:  100,
		ChainID: "block_test",
		Time:    uint64(time.Now().Unix()),
	}
	header2 := header1
	header2.Height = 4

	// End of setup, test data

	commitAtH10 := &types.Commit{Precommits: []*types.Vote{{Height: 10,
		Timestamp: time.Now().UTC()}}}
	tuples := []struct {
		block      *types.Block
		parts      *types.PartSet
		seenCommit *types.Commit
		wantErr    bool
		wantPanic  string

		corruptBlockInDB      bool
		corruptCommitInDB     bool
		corruptSeenCommitInDB bool
		eraseCommitInDB       bool
		eraseSeenCommitInDB   bool
	}{
		{
			block:      newBlock(&header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,
		},

		{
			block:     nil,
			wantPanic: "only save a non-nil block",
		},

		{
			block:     newBlock(&header2, commitAtH10),
			parts:     uncontiguousPartSet,
			wantPanic: "only save contiguous blocks", // and incomplete and uncontiguous parts
		},

		{
			block:     newBlock(&header1, commitAtH10),
			parts:     incompletePartSet,
			wantPanic: "only save complete block", // incomplete parts
		},

		{
			block:             newBlock(&header1, commitAtH10),
			parts:             validPartSet,
			seenCommit:        seenCommit1,
			corruptCommitInDB: true,                                               // Corrupt the DB's commit entry
			wantPanic:         "Error{rlp: expected input list for types.Commit}", //"unmarshal to types.Commit failed",
		},

		{
			block:            newBlock(&header1, commitAtH10),
			parts:            validPartSet,
			seenCommit:       seenCommit1,
			wantPanic:        "Error{rlp: expected input list for types.BlockMeta}", //"unmarshal to types.BlockMeta failed",
			corruptBlockInDB: true,                                                  // Corrupt the DB's block entry
		},

		{
			block:      newBlock(&header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,

			// Expecting no error and we want a nil back
			eraseSeenCommitInDB: true,
		},

		{
			block:      newBlock(&header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,

			corruptSeenCommitInDB: true,
			wantPanic:             "Error{rlp: expected input list for types.Commit}", // "unmarshal to types.Commit failed",
		},

		{
			block:      newBlock(&header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,

			// Expecting no error and we want a nil back
			eraseCommitInDB: true,
		},
	}

	type quad struct {
		block  *types.Block
		commit *types.Commit
		meta   *types.BlockMeta

		seenCommit *types.Commit
	}

	for i, tuple := range tuples {
		bs, db := freshBlockStore()
		// SaveBlock
		res, err, panicErr := doFn(func() (interface{}, error) {
			bs.SaveBlock(tuple.block, tuple.parts, tuple.seenCommit, nil, txsResult)
			if tuple.block == nil {
				return nil, nil
			}

			if tuple.corruptBlockInDB {
				db.Set(calcBlockMetaKey(tuple.block.Height), []byte("block-bogus"))
			}
			bBlock := bs.LoadBlock(tuple.block.Height)
			bBlockMeta := bs.LoadBlockMeta(tuple.block.Height)

			if tuple.eraseSeenCommitInDB {
				db.Delete(calcSeenCommitKey(tuple.block.Height))
			}
			if tuple.corruptSeenCommitInDB {
				db.Set(calcSeenCommitKey(tuple.block.Height), []byte("bogus-seen-commit"))
			}
			bSeenCommit := bs.LoadSeenCommit(tuple.block.Height)

			commitHeight := tuple.block.Height - 1
			if tuple.eraseCommitInDB {
				db.Delete(calcBlockCommitKey(commitHeight))
			}
			if tuple.corruptCommitInDB {
				db.Set(calcBlockCommitKey(commitHeight), []byte("foo-bogus"))
			}
			bCommit := bs.LoadBlockCommit(commitHeight)
			return &quad{block: bBlock, seenCommit: bSeenCommit, commit: bCommit,
				meta: bBlockMeta}, nil
		})

		if subStr := tuple.wantPanic; subStr != "" {
			if panicErr == nil {
				t.Errorf("#%d: want a non-nil panic", i)
			} else if got := panicErr.Error(); !strings.Contains(got, subStr) {
				t.Errorf("#%d:\n\tgotErr: %q\nwant substring: %q", i, got, subStr)
			}
			continue
		}

		if tuple.wantErr {
			if err == nil {
				t.Errorf("#%d: got nil error", i)
			}
			continue
		}

		assert.Nil(t, panicErr, "#%d: unexpected panic", i)
		assert.Nil(t, err, "#%d: expecting a non-nil error", i)
		qua, ok := res.(*quad)
		if !ok || qua == nil {
			t.Errorf("#%d: got nil quad back; gotType=%T", i, res)
			continue
		}
		if tuple.eraseSeenCommitInDB {
			assert.Nil(t, qua.seenCommit,
				"erased the seenCommit in the DB hence we should get back a nil seenCommit")
		}
		if tuple.eraseCommitInDB {
			assert.Nil(t, qua.commit,
				"erased the commit in the DB hence we should get back a nil commit")
		}
	}
	bs.startDeleteHeight = 1
	bs.height = 1
	bs.DeleteHistoricalData(0)
	if bs.startDeleteHeight != loadStartDeleteHeight(bs.db) {
		t.Fatal("start delete height not eq:", bs.startDeleteHeight)
	}

	blockDeleted := bs.LoadBlock(1)
	if blockDeleted != nil {
		t.Fatal("block 1 should deleted.")
	}

}

func TestBlockStoreTxs(t *testing.T) {
	_, bs := initializeValidatorState(0)
	require.Equal(t, bs.Height(), uint64(0), "initially the height should be zero")

	// check there are no blocks at various heights
	noBlockHeights := []uint64{0, 1, 100, 1000, 2}
	for i, height := range noBlockHeights {
		if g := bs.LoadBlock(height); g != nil {
			t.Errorf("#%d: height(%d) got a block; want nil", i, height)
		}
	}

	// save a block
	block := makeBlock(bs.Height() + 1)
	// validPartSet := block.MakePartSet(2)
	seenCommit := &types.Commit{Precommits: []*types.Vote{{Height: 10,
		Timestamp: time.Now().UTC()}}}
	txsResult := &types.TxsResult{}
	receipts := types.Receipts{}
	bs.SaveBlock(block, partSet, seenCommit, &receipts, txsResult)
	require.Equal(t, bs.Height(), block.Header.Height, "expecting the new height to be changed")

	b := bs.LoadBlockByHash(block.Hash())
	if b == nil {
		t.Fatal("LoadBlockByHash exec failed.", "hash", block.Hash().String())
	}

	r := bs.GetReceipts(bs.Height())
	if r == nil {
		t.Fatal("GetReceipts failed.")
	}
	h := bs.GetHeader(bs.Height())
	if h == nil {
		t.Fatal("GetHeader failed.")
	}
	bs.GetTxFromBlock(b, common.EmptyHash)
	bs.RollBackOneBlock()
}

func TestLoadBlockPart(t *testing.T) {
	bs, db := freshBlockStore()
	height, index := uint64(10), 1
	loadPart := func() (interface{}, error) {
		part := bs.LoadBlockPart(height, index)
		return part, nil
	}

	// Initially no contents.
	// 1. Requesting for a non-existent block shouldn't fail
	res, _, panicErr := doFn(loadPart)
	require.Nil(t, panicErr, "a non-existent block part shouldn't cause a panic")
	require.Nil(t, res, "a non-existent block part should return nil")

	// 2. Next save a corrupted block then try to load it
	db.Set(calcBlockPartKey(height, index), []byte("blockchain"))
	res, _, panicErr = doFn(loadPart)
	require.NotNil(t, panicErr, "expecting a non-nil panic")
	require.Contains(t, panicErr.Error(), "rlp: expected input list for types.Part") //"unmarshal to types.Part failed")

	// 3. A good block serialized and saved to the DB should be retrievable
	db.Set(calcBlockPartKey(height, index), ser.MustEncodeToBytes(part1))
	gotPart, _, panicErr := doFn(loadPart)
	require.Nil(t, panicErr, "an existent and proper block should not panic")
	require.Nil(t, res, "a properly saved block should return a proper block")
	require.Equal(t, gotPart.(*types.Part).Hash(), part1.Hash(),
		"expecting successful retrieval of previously saved block")
}

func TestLoadBlockMeta(t *testing.T) {
	bs, db := freshBlockStore()
	height := uint64(10)
	loadMeta := func() (interface{}, error) {
		meta := bs.LoadBlockMeta(height)
		return meta, nil
	}

	// Initially no contents.
	// 1. Requesting for a non-existent blockMeta shouldn't fail
	res, _, panicErr := doFn(loadMeta)
	require.Nil(t, panicErr, "a non-existent blockMeta shouldn't cause a panic")
	require.Nil(t, res, "a non-existent blockMeta should return nil")

	// 2. Next save a corrupted blockMeta then try to load it
	db.Set(calcBlockMetaKey(height), []byte("blockchain-Meta"))
	res, _, panicErr = doFn(loadMeta)
	require.NotNil(t, panicErr, "expecting a non-nil panic")
	require.Contains(t, panicErr.Error(), "rlp: expected input list for types.BlockMeta") // "unmarshal to types.BlockMeta")

	// 3. A good blockMeta serialized and saved to the DB should be retrievable
	meta := &types.BlockMeta{}
	meta.Header = &types.Header{}
	db.Set(calcBlockMetaKey(height), ser.MustEncodeToBytes(meta))
	txsResult := &types.TxsResult{}
	data := ser.MustEncodeToBytes(txsResult)
	bs.db.Set(calcTxsResultKey(height), data)
	gotMeta, _, panicErr := doFn(loadMeta)
	require.Nil(t, panicErr, "an existent and proper block should not panic")
	require.Nil(t, res, "a properly saved blockMeta should return a proper blocMeta ")
	require.Equal(t, ser.MustEncodeToBytes(meta), ser.MustEncodeToBytes(gotMeta),
		"expecting successful retrieval of previously saved blockMeta")
}

func doFn(fn func() (interface{}, error)) (res interface{}, err error, panicErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case error:
				panicErr = e
			case string:
				panicErr = fmt.Errorf("%s", e)
			default:
				if st, ok := r.(fmt.Stringer); ok {
					panicErr = fmt.Errorf("%s", st)
				} else {
					panicErr = fmt.Errorf("%s", debug.Stack())
				}
			}
		}
	}()

	res, err = fn()
	return res, err, panicErr
}

func newBlock(hdr *types.Header, lastCommit *types.Commit) *types.Block {
	return &types.Block{
		Header:     hdr,
		LastCommit: lastCommit,
	}
}
