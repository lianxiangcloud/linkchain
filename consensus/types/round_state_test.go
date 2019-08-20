package types

import (
	"testing"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

func BenchmarkRoundStateDeepCopy(b *testing.B) {
	b.StopTimer()

	// Random validators
	nval, ntxs := 100, 100
	vset, _ := types.RandValidatorSet(nval, 1)
	precommits := make([]*types.Vote, nval)
	blockID := types.BlockID{
		Hash: common.BytesToHash(cmn.RandBytes(20)),
		PartsHeader: types.PartSetHeader{
			Hash: cmn.RandBytes(20),
		},
	}
	sig := crypto.SignatureEd25519{}
	for i := 0; i < nval; i++ {
		precommits[i] = &types.Vote{
			ValidatorAddress: crypto.Address(cmn.RandBytes(20)),
			Timestamp:        time.Now(),
			BlockID:          blockID,
			Signature:        sig,
		}
	}
	txs := make([]types.Tx, ntxs)
	// Random block
	block := &types.Block{
		Header: &types.Header{
			ChainID:        config.ChainID,
			Time:           uint64(time.Now().Unix()),
			LastBlockID:    blockID,
			LastCommitHash: common.BytesToHash(cmn.RandBytes(20)),
			DataHash:       common.BytesToHash(cmn.RandBytes(20)),
			ValidatorsHash: common.BytesToHash(cmn.RandBytes(20)),
			ConsensusHash:  common.BytesToHash(cmn.RandBytes(20)),
			ReceiptHash:    common.BytesToHash(cmn.RandBytes(20)),
			StateHash:      common.BytesToHash(cmn.RandBytes(20)),
			EvidenceHash:   common.BytesToHash(cmn.RandBytes(20)),
		},
		Data: &types.Data{
			Txs: txs,
		},
		Evidence: types.EvidenceData{},
		LastCommit: &types.Commit{
			BlockID:    blockID,
			Precommits: precommits,
		},
	}
	parts := block.MakePartSet(4096)
	// Random Proposal
	proposal := &types.Proposal{
		Timestamp: time.Now(),
		BlockPartsHeader: types.PartSetHeader{
			Hash: cmn.RandBytes(20),
		},
		POLBlockID: blockID,
		Signature:  sig,
	}
	// Random HeightVoteSet
	// TODO: hvs :=

	rs := &RoundState{
		StartTime:          time.Now(),
		CommitTime:         time.Now(),
		Validators:         vset,
		Proposal:           proposal,
		ProposalBlock:      block,
		ProposalBlockParts: parts,
		LockedBlock:        block,
		LockedBlockParts:   parts,
		ValidBlock:         block,
		ValidBlockParts:    parts,
		Votes:              nil, // TODO
		LastCommit:         nil, // TODO
		LastValidators:     vset,
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		ser.DeepCopy(rs)
	}
}
