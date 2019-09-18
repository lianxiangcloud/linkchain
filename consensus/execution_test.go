package consensus

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	crypto "github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/types"
	cfg "github.com/lianxiangcloud/linkchain/config"
)

var (
	chainID      = "execution_chain"
	testPartSize = 65536
	nTxsPerBlock = 10
)

func init() {
	pk := crypto.GenPrivKeyEd25519FromSecret([]byte("test"))
	metrics.PrometheusMetricInstance.Init(cfg.DefaultConfig(),pk.PubKey(),log.Root())
}
func TestApplyBlock(t *testing.T) {

	state, stateDB := state(1, 1)

	blockExec := NewBlockExecutor(stateDB, log.Test(), MockEvidencePool{})

	block := makeBlock(state, 1)
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

	_, err := blockExec.ApplyBlock(state, blockID, block, nil)
	require.Nil(t, err)

}

// make some bogus txs
func makeTxs(height uint64) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		prikey, _ := crypto.GenerateKey()
		tx := types.NewTransaction(uint64(i), cmn.Address{byte(i)}, big.NewInt(0), 0, big.NewInt(1e11), []byte{})
		tx.Sign(types.GlobalSTDSigner, prikey)
		txs = append(txs, tx)
	}
	return txs
}

func state(nVals, height int) (NewStatus, dbm.DB) {
	vals := make([]types.GenesisValidator, nVals)
	for i := 0; i < nVals; i++ {
		secret := []byte(fmt.Sprintf("test%d", i))
		pk := crypto.GenPrivKeyEd25519FromSecret(secret)
		vals[i] = types.GenesisValidator{
			pk.PubKey(), cmn.EmptyAddress, 1000, fmt.Sprintf("test%d", i),
		}
	}
	s, _ := MakeGenesisStatus(&types.GenesisDoc{
		ChainID:    chainID,
		Validators: vals,
	})

	// save validators to db for 2 heights
	stateDB := dbm.NewMemDB()
	SaveStatus(stateDB, s)

	for i := 1; i < height; i++ {
		s.LastBlockHeight++
		SaveStatus(stateDB, s)
	}
	return s, stateDB
}

func makeBlock(state NewStatus, height uint64) *types.Block {
	txs := makeTxs(state.LastBlockHeight)
	block := types.MakeBlock(height, txs, new(types.Commit))

	block.DataHash = block.Data.Hash()
	block.TotalTxs = uint64(len(txs))
	block.ChainID = chainID
	block.ConsensusHash = common.BytesToHash(state.ConsensusParams.Hash())
	block.ValidatorsHash = common.BytesToHash(state.Validators.Hash())
	return block
}
