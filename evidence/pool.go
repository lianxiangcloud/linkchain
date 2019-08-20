package evidence

import (
	"fmt"
	"sync"

	clist "github.com/lianxiangcloud/linkchain/libs/clist"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"

	cs "github.com/lianxiangcloud/linkchain/consensus"
	"github.com/lianxiangcloud/linkchain/types"
)

// EvidencePool maintains a pool of valid evidence
// in an EvidenceStore.
type EvidencePool struct {
	logger log.Logger

	evidenceStore *EvidenceStore
	evidenceList  *clist.CList // concurrent linked-list of evidence

	// needed to load validators to verify evidence
	statusDB dbm.DB

	// latest state
	mtx    sync.Mutex
	status cs.NewStatus
}

func NewEvidencePool(statusDB dbm.DB, evidenceStore *EvidenceStore, status cs.NewStatus) *EvidencePool {
	evpool := &EvidencePool{
		statusDB:      statusDB,
		status:        status,
		logger:        log.NewNopLogger(),
		evidenceStore: evidenceStore,
		evidenceList:  clist.New(),
	}
	return evpool
}

func (evpool *EvidencePool) EvidenceFront() *clist.CElement {
	return evpool.evidenceList.Front()
}

func (evpool *EvidencePool) EvidenceWaitChan() <-chan struct{} {
	return evpool.evidenceList.WaitChan()
}

// SetLogger sets the Logger.
func (evpool *EvidencePool) SetLogger(l log.Logger) {
	evpool.logger = l
}

// PriorityEvidence returns the priority evidence.
func (evpool *EvidencePool) PriorityEvidence() []types.Evidence {
	return evpool.evidenceStore.PriorityEvidence()
}

// PendingEvidence returns all uncommitted evidence.
func (evpool *EvidencePool) PendingEvidence() []types.Evidence {
	return evpool.evidenceStore.PendingEvidence()
}

// Status returns the current status of the evpool.
func (evpool *EvidencePool) Status() cs.NewStatus {
	evpool.mtx.Lock()
	defer evpool.mtx.Unlock()
	return evpool.status
}

// Update loads the latest
func (evpool *EvidencePool) Update(block *types.Block, status cs.NewStatus) {

	// sanity check
	if status.LastBlockHeight != block.Height {
		panic(fmt.Sprintf("Failed EvidencePool.Update sanity check: got status.Height=%d with block.Height=%d", status.LastBlockHeight, block.Height))
	}

	// update the status
	evpool.mtx.Lock()
	evpool.status = status
	evpool.mtx.Unlock()

	// remove evidence from pending and mark committed
	evpool.MarkEvidenceAsCommitted(block.Height, block.Evidence.Evidence)
}

// AddEvidence checks the evidence is valid and adds it to the pool.
func (evpool *EvidencePool) AddEvidence(evidence types.Evidence) (err error) {

	// TODO: check if we already have evidence for this
	// validator at this height so we dont get spammed

	if err := cs.VerifyEvidence(evpool.statusDB, evpool.Status(), evidence); err != nil {
		return err
	}

	// fetch the validator and return its voting power as its priority
	// TODO: something better ?
	valset, _, _ := cs.LoadValidators(evpool.statusDB, evidence.Height())
	_, val := valset.GetByAddress(evidence.Address())
	priority := val.VotingPower

	added := evpool.evidenceStore.AddNewEvidence(evidence, priority)
	if !added {
		// evidence already known, just ignore
		return
	}

	evpool.logger.Info("Verified new evidence of byzantine behaviour", "evidence", evidence)

	// add evidence to clist
	evpool.evidenceList.PushBack(evidence)

	return nil
}

// MarkEvidenceAsCommitted marks all the evidence as committed and removes it from the queue.
func (evpool *EvidencePool) MarkEvidenceAsCommitted(height uint64, evidence []types.Evidence) {
	// make a map of committed evidence to remove from the clist
	blockEvidenceMap := make(map[string]struct{})
	for _, ev := range evidence {
		if _, ok := ev.(*types.FaultValidatorsEvidence); ok {
			continue
		}
		evpool.evidenceStore.MarkEvidenceAsCommitted(ev)
		blockEvidenceMap[evMapKey(ev)] = struct{}{}

	}

	// remove committed evidence from the clist
	maxAge := evpool.Status().ConsensusParams.EvidenceParams.MaxAge
	evpool.removeEvidence(height, maxAge, blockEvidenceMap)

}

func (evpool *EvidencePool) removeEvidence(height, maxAge uint64, blockEvidenceMap map[string]struct{}) {
	for e := evpool.evidenceList.Front(); e != nil; e = e.Next() {
		ev := e.Value.(types.Evidence)

		// Remove the evidence if it's already in a block
		// or if it's now too old.
		if _, ok := blockEvidenceMap[evMapKey(ev)]; ok ||
			ev.Height() < height-maxAge {

			// remove from clist
			evpool.evidenceList.Remove(e)
			e.DetachPrev()
		}
	}
}

func evMapKey(ev types.Evidence) string {
	return string(ev.Hash())
}
