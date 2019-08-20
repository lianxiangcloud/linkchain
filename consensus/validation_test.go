package consensus

import (
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/common"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/stretchr/testify/require"
)

func TestValidateBlock(t *testing.T) {
	state, _ := state(1, 1)

	blockExec := NewBlockExecutor(dbm.NewMemDB(), log.Test(), nil)

	// proper block must pass
	block := makeBlock(state, 1)
	err := blockExec.ValidateBlock(state, block)
	require.NoError(t, err)

	// wrong chain fails
	block = makeBlock(state, 1)
	block.ChainID = "not-the-real-one"
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong height fails
	block = makeBlock(state, 1)
	block.Height += 10
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong total tx fails
	block = makeBlock(state, 1)
	block.TotalTxs += 10
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong blockid fails
	block = makeBlock(state, 1)
	block.LastBlockID.PartsHeader.Total += 10
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong app hash fails, dont check here since  it vaildated in checkBlock
	/*
		block = makeBlock(state, 1)
		block.ParentHash = common.BytesToHash([]byte("wrong app hash"))
		err = blockExec.ValidateBlock(state, block)
		require.Error(t, err)
	*/
	// wrong consensus hash fails
	block = makeBlock(state, 1)
	block.ConsensusHash = common.BytesToHash([]byte("wrong consensus hash"))
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong results hash fails, dont check here since  it vaildated in checkBlock
	/*
		block = makeBlock(state, 1)
		block.ReceiptHash = common.BytesToHash([]byte("wrong Receipt hash"))
		err = blockExec.ValidateBlock(state, block)
		require.Error(t, err)
	*/
	// wrong validators hash fails
	block = makeBlock(state, 1)
	block.ValidatorsHash = common.BytesToHash([]byte("wrong validators hash"))
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)
}
