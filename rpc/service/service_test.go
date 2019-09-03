package service

import (
	"math/big"

	"github.com/lianxiangcloud/linkchain/accounts/keystore"
	"github.com/lianxiangcloud/linkchain/blockchain"
	"github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/db"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/state"
	"github.com/lianxiangcloud/linkchain/types"
)

var (
	testAccounts        []*keystore.Key
	initBalance, _      = big.NewInt(1e18).SetString("0xfffffffffffffffffffffffffff", 0)
	initTokenBalance, _ = big.NewInt(1e18).SetString("0xfffffffffffffffffffffffffff", 0)
	tokenAddr           = common.Address{11}
	logger              = log.Root()
)

type ks struct {
	key string
	pwd string
}

var kss = []ks{
	ks{
		key: `{"address":"54fb1c7d0f011dd63b08f85ed7b518ab82028100","crypto":{"cipher":"aes-128-ctr","ciphertext":"e77ec15da9bdec5488ce40b07a860fb5383dffce6950defeb80f6fcad4916b3a","cipherparams":{"iv":"5df504a561d39675b0f9ebcbafe5098c"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":262144,"p":1,"r":8,"salt":"908cd3b189fc8ceba599382cf28c772b735fb598c7dbbc59ef0772d2b851f57f"},"mac":"9bb92ffd436f5248b73a641a26ae73c0a7d673bb700064f388b2be0f35fedabd"},"id":"2e15f180-b4f1-4d9c-b401-59eeeab36c87","version":3}`,
		pwd: `1234`,
	},
}

func init() {
	logger.SetHandler(log.StdoutHandler)
	for _, k := range kss {
		key, err := keystore.DecryptKey([]byte(k.key), k.pwd)
		if err != nil {
			panic(err)
		}
		testAccounts = append(testAccounts, key)
	}
}

func testRPCConfig() *config.RPCConfig {
	return &config.RPCConfig{
		IpcEndpoint:  "linkchain.ipc",
		HTTPEndpoint: "127.0.0.1:9001",
		HTTPModules:  []string{"web3", "eth", "personal", "debug", "txpool", "net"},
		HTTPCores:    []string{"*"},
		VHosts:       []string{"*"},
	}
}

func newTestBlockStore() *blockchain.BlockStore {
	blockStoreDB := db.NewMemDB()
	return blockchain.NewBlockStore(blockStoreDB)
}

func newTestStateDB() dbm.DB {
	return dbm.NewMemDB()
}

func buildGenesisBlock(blockStore *blockchain.BlockStore) {
	stateDB := newTestStateDB()
	state, err := state.New(common.EmptyHash, state.NewDatabase(stateDB))
	if err != nil {
		return
	}
	for _, acc := range testAccounts {
		state.AddBalance(acc.Address, initBalance)
		state.AddTokenBalance(acc.Address, tokenAddr, initTokenBalance)
	}
	root := state.IntermediateRoot(false)

	block := &types.Block{
		Header: &types.Header{
			Height:     0,
			Coinbase:   common.HexToAddress("0x0000000000000000000000000000000000000000"),
			Time:       uint64(1507737600),
			NumTxs:     0,
			TotalTxs:   0,
			ParentHash: common.EmptyHash,
			StateHash:  common.EmptyHash,
			GasLimit:   types.DefaultConsensusParams().BlockSize.MaxGas,
		},
		Data:       &types.Data{},
		LastCommit: &types.Commit{},
	}

	txsResult := types.TxsResult{StateHash: root}
	state.Commit(false, block.Height)
	state.Database().TrieDB().Commit(root, false)

	BlockPartSet := types.DefaultConsensusParams().BlockGossip.BlockPartSizeBytes
	blockParts := block.MakePartSet(BlockPartSet)

	blockStore.SaveBlock(block, blockParts, nil, nil, &txsResult)
}
