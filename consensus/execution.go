package consensus

import (
	"bytes"
	"fmt"

	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/metrics"
	"github.com/lianxiangcloud/linkchain/types"
)

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	db dbm.DB

	// events
	eventBus types.BlockEventPublisher

	evpool EvidencePool

	logger log.Logger

	sw *p2p.Switch // p2p connections for log
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(db dbm.DB, logger log.Logger, evpool EvidencePool) *BlockExecutor {
	return &BlockExecutor{
		db:       db,
		eventBus: types.NopEventBus{},
		evpool:   evpool,
		logger:   logger,
	}
}

// SetEventBus - sets the event bus for publishing block related events.
// If not called, it defaults to types.NopEventBus.
func (blockExec *BlockExecutor) SetEventBus(eventBus types.BlockEventPublisher) {
	blockExec.eventBus = eventBus
}

// ValidateBlock validates the given block against the given NewStatus.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(status NewStatus, block *types.Block) error {
	return validateBlock(blockExec.db, status, block)
}

// ApplyBlock validates the block against the NewStatus, executes it against the app,
// fires the relevant events, commits the app, and saves the new NewStatus and responses.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(status NewStatus, blockID types.BlockID, block *types.Block, validators []*types.Validator) (NewStatus, error) {

	if err := blockExec.ValidateBlock(status, block); err != nil {
		return status, ErrInvalidBlock(err)
	}

	// update the state with the block and responses
	newstatus, err := updateStatus(status, blockID, block.Header, validators)
	if err != nil {
		return newstatus, fmt.Errorf("consensus status update failed: %v", err)
	}

	// Update evpool with the block and state.
	blockExec.evpool.Update(block, newstatus)
	SaveStatus(blockExec.db, newstatus)

	blockValidatorsMetricsReport(status, block.Height)

	// events are fired after everything else
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	fireEvents(blockExec.logger, blockExec.eventBus, block)

	return newstatus, nil
}

func updateValidators(nextValSet *types.ValidatorSet, validators []*types.Validator) error {
	for _, new := range validators {
		_, old := nextValSet.GetByAddress(new.Address)
		if old == nil && new.VotingPower <= 0 {
			continue
		}
		if old == nil && new.VotingPower > 0 {
			added := nextValSet.Add(new)
			if !added {
				return fmt.Errorf("failed to add new validator(%s)", new.String())
			}
			continue
		}
		if new.VotingPower == 0 {
			_, removed := nextValSet.Remove(new.Address)
			if !removed {
				return fmt.Errorf("failed to remove old validator(%s)", new.String())
			}
			continue
		}

		if new.VotingPower != old.VotingPower {
			updated := nextValSet.Update(new)
			if !updated {
				return fmt.Errorf("failed to update old validator(%s) into new(%s)", old.String(), new.String())
			}
		}
	}
	return nil
}

// updateStatus returns a new NewStatus updated according to the header and responses.
func updateStatus(status NewStatus, blockID types.BlockID, header *types.Header, validators []*types.Validator) (NewStatus, error) {

	// copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators
	nextValSet := status.Validators.Copy()

	//Update all validators from beginning
	var valsChanged bool
	if validators != nil && len(validators) != 0 {
		newValSet := types.NewValidatorSet(validators)
		if !bytes.Equal(newValSet.Hash(), nextValSet.Hash()) {
			log.Info("updateValidators", "height", header.Height, "new", newValSet.String())
			nextValSet = newValSet
			valsChanged = true
		}
	}

	if !valsChanged {
		nextValSet.IncrementAccum(1)
	}

	// update the validator change flag
	lastHeightValsChanged := status.LastHeightValidatorsChanged
	if valsChanged {
		lastHeightValsChanged = header.Height + 1
	}

	// update the params with the latest status
	nextParams := status.ConsensusParams
	lastHeightParamsChanged := status.LastHeightConsensusParamsChanged

	return NewStatus{
		ChainID:                          status.ChainID,
		LastBlockHeight:                  header.Height,
		LastBlockTotalTx:                 status.LastBlockTotalTx + header.NumTxs,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		Validators:                       nextValSet,
		LastValidators:                   status.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if crashes before commit, some or all of these events may be published again.
func fireEvents(logger log.Logger, eventBus types.BlockEventPublisher, block *types.Block) {
	if eventBus == nil || block == nil {
		return
	}
	eventBus.PublishEventNewBlock(types.EventDataNewBlock{block})
	eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{block.Header})
}

func blockValidatorsMetricsReport(status NewStatus, blockHeight uint64) {
	_, current := status.GetValidators()
	fmt.Println(current.GetProposer().PubKey)
	metrics.PrometheusMetricInstance().SetCurrentProposerPubkey(current.GetProposer().PubKey)

	// Record every block's validators list ,only report when the node sis proposer
	if metrics.PrometheusMetricInstance().ProposerPubkeyEquals() {
		currentValidators := status.Validators.Validators
		validatorStr := ""
		for _, validator := range currentValidators {
			validatorStr += validator.PubKey.Address().String() + ","
		}
		if len(validatorStr) > 0 {
			validatorStr = validatorStr[:len(validatorStr)-1]
		}
		blockValidatorsListMetric := metrics.PrometheusMetricInstance().GenBlockValidatorsListMetric(validatorStr, blockHeight)
		metrics.PrometheusMetricInstance().AddMetrics(blockValidatorsListMetric)
	}
}
