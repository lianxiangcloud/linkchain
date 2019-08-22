package consensus

import (
	"bytes"
	"errors"
	"fmt"

	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/types"
)

//-----------------------------------------------------
// Validate block

func validateBlock(statusDB dbm.DB, status NewStatus, block *types.Block) error {
	// validate internal consistency
	if err := block.ValidateBasic(); err != nil {
		return err
	}

	// validate basic info
	if block.ChainID != status.ChainID {
		return fmt.Errorf("Wrong Block.Header.ChainID. Expected %v, got %v", status.ChainID, block.ChainID)
	}
	if block.Height != status.LastBlockHeight+1 {
		return fmt.Errorf("Wrong Block.Header.Height. Expected %v, got %v", status.LastBlockHeight+1, block.Height)
	}
	/*	TODO: Determine bounds for Time
		See blockchain/reactor "stopSyncingDurationMinutes"

		if !block.Time.After(lastBlockTime) {
			return errors.New("Invalid Block.Header.Time")
		}
	*/

	// validate prev block info
	if !block.LastBlockID.Equals(status.LastBlockID) {
		return fmt.Errorf("Wrong Block.Header.LastBlockID.  Expected %v, got %v", status.LastBlockID, block.LastBlockID)
	}
	newTxs := uint64(len(block.Data.Txs))
	if block.TotalTxs != status.LastBlockTotalTx+newTxs {
		return fmt.Errorf("Wrong Block.Header.TotalTxs. Expected %v, got %v", status.LastBlockTotalTx+newTxs, block.TotalTxs)
	}

	// validate app info
	if !bytes.Equal(block.ConsensusHash.Bytes(), status.ConsensusParams.Hash()) {
		return fmt.Errorf("Wrong Block.Header.ConsensusHash.  Expected %X, got %v", status.ConsensusParams.Hash(), block.ConsensusHash)
	}
	if !bytes.Equal(block.ValidatorsHash.Bytes(), status.Validators.Hash()) && block.Recover < 1 {
		return fmt.Errorf("Wrong Block.Header.ValidatorsHash.  Expected %X, got %v", status.Validators.Hash(), block.ValidatorsHash)
	}

	// Validate block LastCommit.
	if block.Height == types.BlockHeightOne {
		if len(block.LastCommit.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		if len(block.LastCommit.Precommits) != status.LastValidators.Size() {
			return fmt.Errorf("Invalid block commit size. Expected %v, got %v",
				status.LastValidators.Size(), len(block.LastCommit.Precommits))
		}
		err := status.LastValidators.VerifyCommit(
			status.ChainID, status.LastBlockID, block.Height-1, block.LastCommit)
		if err != nil {
			return err
		}
	}

	// TODO: Each check requires loading an old validator set.
	// We should cap the amount of evidence per block
	// to prevent potential proposer DoS.
	var onlyOneFvi bool
	for _, ev := range block.Evidence.Evidence {
		switch evi := ev.(type) {
		case *types.DuplicateVoteEvidence:
			if err := VerifyEvidence(statusDB, status, ev); err != nil {
				return types.NewEvidenceInvalidErr(ev, err)
			}
		case *types.FaultValidatorsEvidence:
			if err := VerifyFaultValEvidence(status, block.LastCommit, evi); err != nil || onlyOneFvi {
				return types.NewEvidenceInvalidErr(ev, err)
			}
			onlyOneFvi = true
		default:
			return types.NewEvidenceInvalidErr(ev, nil)
		}
	}

	if !onlyOneFvi && block.Height > types.BlockHeightOne && !status.LastRecover {
		return fmt.Errorf("Not found FaultValidatorsEvidence height:%d", block.Height)
	}

	return nil
}

// VerifyEvidence verifies the evidence fully by checking:
// - it is sufficiently recent (MaxAge)
// - it is from a key who was a validator at the given height
// - it is internally consistent
// - it was properly signed by the alleged equivocator
func VerifyEvidence(statusDB dbm.DB, status NewStatus, evidence types.Evidence) error {
	height := status.LastBlockHeight

	evidenceAge := height - evidence.Height()
	maxAge := status.ConsensusParams.EvidenceParams.MaxAge
	if int64(evidenceAge) > int64(maxAge) {
		return fmt.Errorf("Evidence from height %d is too old. Min height is %d",
			evidence.Height(), int64(height-maxAge))
	}

	valset, _, err := LoadValidators(statusDB, evidence.Height())
	if err != nil {
		// TODO: if err is just that we cant find it cuz we pruned, ignore.
		// TODO: if its actually bad evidence, punish peer
		return err
	}

	// The address must have been an active validator at the height.
	// NOTE: we will ignore evidence from H if the key was not a validator
	// at H, even if it is a validator at some nearby H'
	ev := evidence
	height, addr := ev.Height(), ev.Address()
	_, val := valset.GetByAddress(addr)
	if val == nil {
		return fmt.Errorf("Address %X was not a validator at height %d", addr, height)
	}

	if err := evidence.Verify(status.ChainID, val.PubKey); err != nil {
		return err
	}

	return nil
}

// VerifyFaultValEvidence check the FaultValidatorsEvidence
// Just compare lastblock produce rounds and fault proposer which should produce block but not
func VerifyFaultValEvidence(status NewStatus, lastCommit *types.Commit, fvi *types.FaultValidatorsEvidence) error {
	cRound, height := lastCommit.FirstPrecommit().Round, lastCommit.FirstPrecommit().Height
	if fvi.Round != cRound || fvi.Height() != height {
		return fmt.Errorf("Evidence round/height error exp:%d/%d but:%d/%d", cRound, height, fvi.Round, fvi.Height())
	}

	addr := status.LastValidators.GetProposer().Address.String()
	if cRound == 0 {
		if fvi.FaultVal != nil {
			return fmt.Errorf("Evidence FaultVal not nil when round 0 ")
		}
		if fvi.Proposer.Address().String() != addr {
			return fmt.Errorf("Evidence proposer error exp:%v but:%v", addr, fvi.Proposer.Address().String())
		}
		return nil
	}

	if fvi.FaultVal.Address().String() != addr {
		return fmt.Errorf("Evidence FaultVal error exp:%v but:%v", addr, fvi.FaultVal.Address().String())
	}

	valset := status.LastValidators.Copy()
	valset.IncrementAccum(cRound)
	proposer := valset.GetProposer().Address
	if fvi.Proposer.Address().String() != proposer.String() {
		return fmt.Errorf("Evidence proposer error exp:%v but:%v", proposer.String(), fvi.Proposer.Address().String())
	}

	return nil
}
