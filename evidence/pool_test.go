package evidence

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cs "github.com/lianxiangcloud/linkchain/consensus"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/types"
)

var mockStatus = cs.NewStatus{}

func initializeValidatorState(valAddr []byte, height uint64) (dbm.DB, cs.NewStatus) {
	stateDB := dbm.NewMemDB()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			{Address: valAddr},
		},
	}
	state := cs.NewStatus{
		LastBlockHeight:             0,
		LastBlockTime:               uint64(time.Now().Unix()),
		Validators:                  valSet,
		LastHeightValidatorsChanged: 1,
		ConsensusParams: types.ConsensusParams{
			EvidenceParams: types.EvidenceParams{
				MaxAge: 1000000,
			},
		},
	}

	// save all states up to height
	for i := uint64(0); i < height; i++ {
		state.LastBlockHeight = i
		cs.SaveStatus(stateDB, state)
	}

	return stateDB, state
}

func TestEvidencePool(t *testing.T) {

	valAddr := []byte("val1")
	height := uint64(5)
	stateDB, newstatus := initializeValidatorState(valAddr, height)
	store := NewEvidenceStore(dbm.NewMemDB())
	pool := NewEvidencePool(stateDB, store, newstatus)

	goodEvidence := types.NewMockGoodEvidence(height, 0, valAddr)
	badEvidence := types.MockBadEvidence{goodEvidence}

	// bad evidence
	err := pool.AddEvidence(badEvidence)
	assert.NotNil(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-pool.EvidenceWaitChan()
		wg.Done()
	}()
	goodEvidence.Height_--
	err = pool.AddEvidence(goodEvidence)
	assert.Nil(t, err)
	wg.Wait()

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt change the size
	err = pool.AddEvidence(goodEvidence)
	assert.Nil(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}
