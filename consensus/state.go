package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	fail "github.com/ebuchman/fail-test"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"

	cfg "github.com/lianxiangcloud/linkchain/config"
	cstypes "github.com/lianxiangcloud/linkchain/consensus/types"
	"github.com/lianxiangcloud/linkchain/libs/common"
	tmevents "github.com/lianxiangcloud/linkchain/libs/events"
	"github.com/lianxiangcloud/linkchain/types"
)

//-----------------------------------------------------------------------------
// Config

const (
	timeoutRecover       = 15 * time.Minute
	timeoutRecoverLimit  = 12 * time.Minute
	recoverFirstProposal = 3 * time.Minute

	proposalHeartbeatIntervalSeconds = 2
)

//-----------------------------------------------------------------------------
// Errors

var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
)

//-----------------------------------------------------------------------------

var (
	msgQueueSize = 1000

	blockCounter             int64 = 0
	lastBlockTime            int64 = 0
	lastCheckPointTime       int64 = 0
	needSupplementBlockCount       = 0
	isWaitForExtraTime             = false
)

// msgs from the reactor which may update the state
type msgInfo struct {
	Msg    ConsensusMessage `json:"msg"`
	PeerID string           `json:"peer_key"`
}

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration         `json:"duration"`
	Height   uint64                `json:"height"`
	Round    int                   `json:"round"`
	Step     cstypes.RoundStepType `json:"step"`
}

func (ti *timeoutInfo) String() string {
	return fmt.Sprintf("%v ; %d/%d %v", ti.Duration, ti.Height, ti.Round, ti.Step)
}

// ConsensusState handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator, and from a timer.
type ConsensusState struct {
	cmn.BaseService

	// config details
	config        *cfg.ConsensusConfig
	privValidator types.PrivValidator // for signing votes

	// services for creating and executing blocks
	// TODO: encapsulate all of this in one "BlockManager"
	blockExec *BlockExecutor
	appmgr    BlockChainApp
	mempool   Mempool
	evpool    EvidencePool

	// internal state
	mtx sync.Mutex
	cstypes.RoundState
	status NewStatus // Status until height-1.

	// state changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo
	timeoutTicker    TimeoutTicker
	timeoutTimer     *time.Timer
	stepRecover      bool
	recover          uint32

	// we use eventBus to trigger msg broadcasts in the reactor,
	// and to notify external subscribers, eg. through a websocket
	eventBus *types.EventBus

	// a Write-Ahead Log ensures we can recover from any kind of crash
	// and helps us avoid signing conflicting votes
	wal          WAL
	replayMode   bool // so we don't log signing errors during replay
	doWALCatchup bool // determines if we even try to do the catchup

	// for tests where we want to limit the number of transitions the state makes
	nSteps int

	// some functions can be overwritten for testing
	decideProposal func(height uint64, round int)
	doPrevote      func(height uint64, round int)
	setProposal    func(proposal *types.Proposal) error

	// closed when we finish shutting down
	done chan struct{}

	// synchronous pubsub between consensus state and reactor.
	// state only emits EventNewRoundStep, EventVote and EventProposalHeartbeat
	evsw tmevents.EventSwitch

	// for reporting metrics
	metrics *Metrics

	startDeleteHeight uint64
}

// CSOption sets an optional parameter on the ConsensusState.
type CSOption func(*ConsensusState)

// NewConsensusState returns a new ConsensusState.
func NewConsensusState(
	config *cfg.ConsensusConfig,
	status NewStatus,
	blockExec *BlockExecutor,
	app BlockChainApp,
	mempool Mempool,
	evpool EvidencePool,
	options ...CSOption,
) *ConsensusState {
	cs := &ConsensusState{
		config:           config,
		blockExec:        blockExec,
		appmgr:           app,
		mempool:          mempool,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(),
		done:             make(chan struct{}),
		doWALCatchup:     true,
		wal:              nilWAL{},
		evpool:           evpool,
		evsw:             tmevents.NewEventSwitch(),
		metrics:          NopMetrics(),

		startDeleteHeight: 0,
	}
	// set function defaults (may be overwritten before calling Start)
	cs.decideProposal = cs.defaultDecideProposal
	cs.doPrevote = cs.defaultDoPrevote
	cs.setProposal = cs.defaultSetProposal

	cs.updateToStatus(status)
	// Don't call scheduleRound0 yet.
	// We do that upon Start().
	cs.reconstructLastCommit(status)
	cs.BaseService = *cmn.NewBaseService(nil, "ConsensusState", cs)
	for _, option := range options {
		option(cs)
	}
	return cs
}

//----------------------------------------
// Public interface

// SetLogger implements Service.
func (cs *ConsensusState) SetLogger(l log.Logger) {
	cs.BaseService.Logger = l
	cs.timeoutTicker.SetLogger(l)
}

// SetEventBus sets event bus.
func (cs *ConsensusState) SetEventBus(b *types.EventBus) {
	cs.eventBus = b
	cs.blockExec.SetEventBus(b)
}

// WithMetrics sets the metrics.
func WithMetrics(metrics *Metrics) CSOption {
	return func(cs *ConsensusState) { cs.metrics = metrics }
}

// String returns a string.
func (cs *ConsensusState) String() string {
	// better not to access shared variables
	return cmn.Fmt("ConsensusState") //(H:%v R:%v S:%v", cs.Height, cs.Round, cs.Step)
}

// GetState returns a copy of the chain status.
func (cs *ConsensusState) GetState() NewStatus {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.status.Copy()
}

func (cs *ConsensusState) SetReceiveP2pTx(on bool) {
	cs.mempool.SetReceiveP2pTx(on)
}

// GetChainId returns consensus chainID.
func (cs *ConsensusState) GetChainId() string {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.status.ChainID
}

// GetRoundState returns a shallow copy of the internal consensus state.
func (cs *ConsensusState) GetRoundState() *cstypes.RoundState {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	rs := cs.RoundState // copy
	return &rs
}

// GetRoundStateJSON returns a json of RoundState, marshalled using go-amino.
func (cs *ConsensusState) GetRoundStateJSON() ([]byte, error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	return ser.MarshalJSON(cs.RoundState)
}

// GetRoundStateSimpleJSON returns a json of RoundStateSimple, marshalled using go-amino.
func (cs *ConsensusState) GetRoundStateSimpleJSON() ([]byte, error) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	return ser.MarshalJSON(cs.RoundState.RoundStateSimple())
}

// GetValidators returns a copy of the current validators.
func (cs *ConsensusState) GetValidators() (uint64, []*types.Validator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	return cs.status.LastBlockHeight, cs.status.Validators.Copy().Validators
}

// SetPrivValidator sets the private validator account for signing votes.
func (cs *ConsensusState) SetPrivValidator(priv types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.privValidator = priv
}

// SetTimeoutTicker sets the local timer. It may be useful to overwrite for testing.
func (cs *ConsensusState) SetTimeoutTicker(timeoutTicker TimeoutTicker) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	cs.timeoutTicker = timeoutTicker
}

// LoadCommit loads the commit for a given height.
func (cs *ConsensusState) LoadCommit(height uint64) *types.Commit {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	if height == cs.appmgr.Height() {
		return cs.appmgr.LoadSeenCommit(height)
	}
	return cs.appmgr.LoadBlockCommit(height)
}

// OnStart implements cmn.Service.
// It loads the latest state via the WAL, and starts the timeout and receive routines.
func (cs *ConsensusState) OnStart() error {
	if err := cs.evsw.Start(); err != nil {
		return err
	}

	// we may set the WAL in testing before calling Start,
	// so only OpenWAL if its still the nilWAL
	if _, ok := cs.wal.(nilWAL); ok {
		walFile := cs.config.WalFile()
		wal, err := cs.OpenWAL(walFile)
		if err != nil {
			cs.Logger.Error("Error loading ConsensusState wal", "err", err.Error())
			return err
		}
		cs.wal = wal
	}

	cs.timeoutTimer = time.NewTimer(timeoutRecover)

	// we need the timeoutRoutine for replay so
	// we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	// firing on the tockChan until the receiveRoutine is started
	// to deal with them (by that point, at most one will be valid)
	if err := cs.timeoutTicker.Start(); err != nil {
		return err
	}

	// we may have lost some votes if the process crashed
	// reload from consensus log to catchup
	if cs.doWALCatchup {
		if err := cs.catchupReplay(cs.Height); err != nil {
			cs.Logger.Error("Error on catchup replay. Proceeding to start ConsensusState anyway", "err", err.Error())
			// NOTE: if we ever do return an error here,
			// make sure to stop the timeoutTicker
		}
	}

	// now start the receiveRoutine
	go cs.receiveRoutine(0)

	// schedule the first round!
	// use GetRoundState so we don't race the receiveRoutine for access
	cs.scheduleRound0(cs.GetRoundState())

	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
func (cs *ConsensusState) startRoutines(maxSteps int) {
	err := cs.timeoutTicker.Start()
	if err != nil {
		cs.Logger.Error("Error starting timeout ticker", "err", err)
		return
	}
	go cs.receiveRoutine(maxSteps)
}

// OnStop implements cmn.Service. It stops all routines and waits for the WAL to finish.
func (cs *ConsensusState) OnStop() {
	cs.evsw.Stop()
	cs.timeoutTimer.Stop()
	cs.timeoutTicker.Stop()
}

// OnReset implements cmn.Service.
func (cs *ConsensusState) OnReset() error {
	cs.Logger.Info("reset consensus.State")
	cs.done = make(chan struct{})
	cs.wal = nilWAL{}
	cs.stepRecover = false
	err := cs.evsw.Reset()
	if err != nil {
		cs.Logger.Error("Error reset state.evsw", "err", err)
		return err
	}
	err = cs.timeoutTicker.Reset()
	return err
}

// Wait waits for the the main routine to return.
// NOTE: be sure to Stop() the event switch and drain
// any event channels or this may deadlock
func (cs *ConsensusState) Wait() {
	<-cs.done
	cs.Logger.Info("consensus.State Wait", "internalMsgQueue", len(cs.internalMsgQueue), "peerMsgQueue", len(cs.peerMsgQueue))
}

// OpenWAL opens a file to log all consensus messages and timeouts for deterministic accountability
func (cs *ConsensusState) OpenWAL(walFile string) (WAL, error) {
	wal, err := NewWAL(walFile)
	if err != nil {
		cs.Logger.Error("Failed to open WAL for consensus state", "wal", walFile, "err", err)
		return nil, err
	}
	wal.SetLogger(cs.Logger.With("wal", walFile))
	if err := wal.Start(); err != nil {
		return nil, err
	}
	return wal, nil
}

//------------------------------------------------------------
// Public interface for passing messages into the consensus state, possibly causing a state transition.
// If peerID == "", the msg is considered internal.
// Messages are added to the appropriate queue (peer or internal).
// If the queue is full, the function may block.
// TODO: should these return anything or let callers just use events?

// AddVote inputs a vote.
func (cs *ConsensusState) AddVote(vote *types.Vote, peerID string) (added bool, err error) {
	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&VoteMessage{vote}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&VoteMessage{vote}, peerID}
	}

	// TODO: wait for event?!
	return false, nil
}

// SetProposal inputs a proposal.
func (cs *ConsensusState) SetProposal(proposal *types.Proposal, peerID string) error {

	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&ProposalMessage{proposal}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&ProposalMessage{proposal}, peerID}
	}

	// TODO: wait for event?!
	return nil
}

// AddProposalBlockPart inputs a part of the proposal block.
func (cs *ConsensusState) AddProposalBlockPart(height uint64, round int, part *types.Part, peerID string) error {

	if peerID == "" {
		cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, ""}
	} else {
		cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerID}
	}

	// TODO: wait for event?!
	return nil
}

// SetProposalAndBlock inputs the proposal and all block parts.
func (cs *ConsensusState) SetProposalAndBlock(proposal *types.Proposal, block *types.Block, parts *types.PartSet, peerID string) error {
	if err := cs.SetProposal(proposal, peerID); err != nil {
		return err
	}
	for i := 0; i < parts.Total(); i++ {
		part := parts.GetPart(i)
		if err := cs.AddProposalBlockPart(proposal.Height, proposal.Round, part, peerID); err != nil {
			return err
		}
	}
	return nil
}

//------------------------------------------------------------
// internal functions for managing the state

func (cs *ConsensusState) getLastFaultValsInfo(lastCommit *types.Commit) types.Evidence {
	if cs.Height > types.BlockHeightOne && !cs.status.LastRecover {
		lastRound := lastCommit.FirstPrecommit().Round
		fvi := &types.FaultValidatorsEvidence{
			BlockHeight: cs.Height - 1, //evidence for lastBlock
			Round:       lastRound,
		}
		if lastRound == 0 {
			fvi.FaultVal = nil
			fvi.Proposer = cs.LastValidators.GetProposer().PubKey
		} else {
			fvi.FaultVal = cs.LastValidators.GetProposer().PubKey

			valset := cs.LastValidators.Copy()
			valset.IncrementAccum(lastRound)
			fvi.Proposer = valset.GetProposer().PubKey
			cs.Logger.Info("getLastFaultValsInfo: Set evidence", "FaultValidatorsEvidence", fvi)
		}
		return fvi
	}
	return nil
}

func (cs *ConsensusState) checkEvidenceAge(evidence types.Evidence) bool {
	height := cs.status.LastBlockHeight

	evidenceAge := height - evidence.Height()
	maxAge := cs.status.ConsensusParams.EvidenceParams.MaxAge
	if int64(evidenceAge) > int64(maxAge) {
		cs.Logger.Warn(cmn.Fmt("Evidence from height %d is too old. Min height is %d",
			evidence.Height(), int64(height-maxAge)))
		return false
	}
	return true
}

func (cs *ConsensusState) checkDuplicateVoteEvidence(evidence types.Evidence) bool {
	if !cs.checkEvidenceAge(evidence) {
		return false
	}

	valset, _, err := LoadValidators(cs.blockExec.db, evidence.Height())
	if err != nil {
		cs.Logger.Warn(cmn.Fmt("LoadValidators at height %d failed", evidence.Height()))
		return false
	}

	// The address must have been an active validator at the height.
	// NOTE: we will ignore evidence from H if the key was not a validator
	// at H, even if it is a validator at some nearby H'
	ev := evidence
	height, addr := ev.Height(), ev.Address()
	_, val := valset.GetByAddress(addr)
	if val == nil {
		cs.Logger.Warn(cmn.Fmt("Address %X was not a validator at height %d", addr, height))
		return false
	}

	if err := evidence.Verify(cs.status.ChainID, val.PubKey); err != nil {
		cs.Logger.Warn(cmn.Fmt("Evidence verify addr %X at height %d failed, err:%v", addr, height, err))
		return false
	}

	return true
}

func (cs *ConsensusState) checkFaultValEvidence(ev *types.FaultValidatorsEvidence, lastCommit *types.Commit) bool {
	if !cs.checkEvidenceAge(ev) {
		return false
	}

	lastRound := lastCommit.FirstPrecommit().Round
	lastHeight := lastCommit.FirstPrecommit().Height

	if ev.Round != lastRound || ev.Height() != lastHeight {
		cs.Logger.Warn("Evidence round/height error.",
			"lastRound", lastRound, "lastHeight", lastHeight, "ev.Round", ev.Round, "ev.Height", ev.Height())
		return false
	}

	addr := cs.LastValidators.GetProposer().Address.String()

	if lastRound == 0 {
		if ev.FaultVal != nil {
			cs.Logger.Warn("Evidence FaultVal not nil when round 0.")
			return false
		}
		if ev.Proposer.Address().String() != addr {
			cs.Logger.Warn("Evidence proposer error.", "exp", addr, "ev.Proposer", ev.Proposer.Address().String())
			return false
		}
		return true
	}

	if ev.FaultVal.Address().String() != addr {
		cs.Logger.Warn("Evidence FaultVal error.", "exp", addr, "ev.FaultVal", ev.FaultVal.Address().String())
		return false
	}

	valset := cs.LastValidators.Copy()
	valset.IncrementAccum(lastRound)
	proposer := valset.GetProposer().Address.String()
	if ev.Proposer.Address().String() != proposer {
		cs.Logger.Warn("Evidence proposer error.",
			"exp", proposer, "ev.Proposer", ev.Proposer.Address().String())
		return false
	}

	return true
}

func (cs *ConsensusState) checkBlockEvidence(block *types.Block) bool {
	if cs.Height <= types.BlockHeightOne || cs.status.LastRecover {
		return true
	}

	var seenFve bool

	for _, ev := range block.Evidence.Evidence {
		switch evi := ev.(type) {
		case *types.FaultValidatorsEvidence:
			cs.Logger.Info("Check FaultValidatorsEvidence")

			if seenFve || !cs.checkFaultValEvidence(evi, block.LastCommit) {
				return false
			}
			seenFve = true

		case *types.DuplicateVoteEvidence:
			cs.Logger.Info("Check DuplicateVoteEvidence")
			if !cs.checkDuplicateVoteEvidence(ev) {
				return false
			}

		default:
			cs.Logger.Warn("Inlavid vidence")
			return false
		}
	}

	return true
}

func (cs *ConsensusState) updateHeight(height uint64) {
	cs.metrics.Height.Set(float64(height))
	cs.Height = height
}

func (cs *ConsensusState) updateRoundStep(round int, step cstypes.RoundStepType) {
	if cs.stepRecover && round > cs.Round {
		cs.recover += uint32(round - cs.Round)
	}
	cs.Round = round
	cs.Step = step
}

// enterNewRound(height, 0) at cs.StartTime.
func (cs *ConsensusState) scheduleRound0(rs *cstypes.RoundState) {
	//cs.Logger.Info("scheduleRound0", "now", time.Now(), "startTime", cs.StartTime)
	sleepDuration := rs.StartTime.Sub(time.Now()) // nolint: gotype, gosimple
	cs.scheduleTimeout(sleepDuration, rs.Height, 0, cstypes.RoundStepNewHeight)
}

// Attempt to schedule a timeout (by sending timeoutInfo on the tickChan)
func (cs *ConsensusState) scheduleTimeout(duration time.Duration, height uint64, round int, step cstypes.RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

// send a msg into the receiveRoutine regarding our own proposal, block part, or vote
func (cs *ConsensusState) sendInternalMessage(mi msgInfo) {
	select {
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		// TODO: use CList here for strict determinism and
		// attempt push to internalMsgQueue in receiveRoutine
		cs.Logger.Info("Internal msg queue is full. Using a go-routine")
		go func() { cs.internalMsgQueue <- mi }()
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (cs *ConsensusState) reconstructLastCommit(status NewStatus) {
	if status.LastBlockHeight == types.BlockHeightZero {
		return
	}
	seenCommit := cs.appmgr.LoadSeenCommit(status.LastBlockHeight)
	lastPrecommits := types.NewVoteSet(status.ChainID, status.LastBlockHeight, seenCommit.Round(), types.VoteTypePrecommit, status.LastValidators)
	for _, precommit := range seenCommit.Precommits {
		if precommit == nil {
			continue
		}
		added, err := lastPrecommits.AddVote(precommit)
		if !added || err != nil {
			cmn.PanicCrisis(cmn.Fmt("Failed to reconstruct LastCommit: %v", err))
		}
	}
	if !lastPrecommits.HasTwoThirdsMajority() {
		cmn.PanicSanity("Failed to reconstruct LastCommit: Does not have +2/3 maj")
	}
	cs.LastCommit = lastPrecommits
}

// Updates ConsensusState and increments height to match that of status.
// The round becomes 0 and cs.Step becomes cstypes.RoundStepNewHeight.
func (cs *ConsensusState) updateToStatus(status NewStatus) {
	if cs.CommitRound > -1 && 0 < cs.Height && cs.Height != status.LastBlockHeight {
		cmn.PanicSanity(cmn.Fmt("updateToStatus() expected status height of %v but found %v",
			cs.Height, status.LastBlockHeight))
	}
	if !cs.status.IsEmpty() && cs.status.LastBlockHeight+1 != cs.Height {
		// This might happen when someone else is mutating cs.status.
		// Someone forgot to pass in status.Copy() somewhere?!
		cmn.PanicSanity(cmn.Fmt("Inconsistent cs.status.LastBlockHeight+1 %v vs cs.Height %v",
			cs.status.LastBlockHeight+1, cs.Height))
	}

	// If status isn't further out than cs.status, just ignore.
	// This happens when SwitchToConsensus() is called in the reactor.
	// We don't want to reset e.g. the Votes, but we still want to
	// signal the new round step, because other services (eg. mempool)
	// depend on having an up-to-date peer status!
	if !cs.status.IsEmpty() && (status.LastBlockHeight < cs.status.LastBlockHeight) {
		cs.Logger.Info("Ignoring updateToStatus()", "newHeight", status.LastBlockHeight+1, "oldHeight", cs.status.LastBlockHeight+1)
		cs.newStep()
		return
	}

	// Reset fields based on status.
	validators := status.Validators
	lastPrecommits := (*types.VoteSet)(nil)
	if cs.CommitRound > -1 && cs.Votes != nil {
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			cmn.PanicSanity("updateToStatus(status) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = cs.Votes.Precommits(cs.CommitRound)
	}

	// Next desired block height
	height := status.LastBlockHeight + 1

	if cs.status.LastHeightValidatorsChanged != status.LastHeightValidatorsChanged {
		cs.appmgr.SetLastChangedVals(status.LastHeightValidatorsChanged, status.Validators.Copy().Validators)
	}

	// RoundState fields
	cs.updateHeight(height)
	cs.updateRoundStep(0, cstypes.RoundStepNewHeight)
	if cs.CommitTime.IsZero() {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		//  cs.StartTime = status.LastBlockTime.Add(timeoutCommit)
		cs.StartTime = cs.config.Commit(time.Now())
	} else {
		cs.StartTime = cs.config.Commit(cs.CommitTime)
	}

	cs.status = status
	if cs.stepRecover {
		cs.Logger.Info("updateToStatus we are in recover step in current block")
		cs.stepRecover = false
		cs.status.LastRecover = true
		cs.status.LastValidators = cs.Validators
		cs.timeoutTimer.Reset(cs.StartTime.Add(timeoutRecover).Sub(time.Now()))
	}

	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.ValidRound = 0
	cs.ValidBlock = nil
	cs.ValidBlockParts = nil
	cs.Votes = cstypes.NewHeightVoteSet(status.ChainID, height, validators)
	cs.CommitRound = -1
	cs.LastCommit = lastPrecommits
	cs.LastValidators = status.LastValidators

	cs.recover = 0

	// Finally, broadcast RoundState
	cs.newStep()
}

func (cs *ConsensusState) newStep() {
	rs := cs.RoundStateEvent()
	cs.wal.Write(rs)
	cs.nSteps++
	// newStep is called by updateToStatus in NewConsensusState before the eventBus is set!
	if cs.eventBus != nil {
		cs.eventBus.PublishEventNewRoundStep(rs)
		cs.evsw.FireEvent(types.EventNewRoundStep, &cs.RoundState)
	}
}

//-----------------------------------------
// the main go routines

// receiveRoutine handles messages which may cause state transitions.
// it's argument (n) is the number of messages to process before exiting - use 0 to run forever
// It keeps the RoundState and is the only thing that updates it.
// Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities.
// ConsensusState must be locked before any internal state is updated.
func (cs *ConsensusState) receiveRoutine(maxSteps int) {
	defer func() {
		if r := recover(); r != nil {
			cs.Logger.Error("CONSENSUS FAILURE!!!", "err", r, "stack", string(debug.Stack()))
		}
	}()

	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				cs.Logger.Info("reached max steps. exiting receive routine")
				cs.nSteps = 0
				return
			}
		}
		rs := cs.RoundState
		var mi msgInfo

		select {
		case <-cs.mempool.TxsAvailable():
			cs.handleTxsAvailable()
		case mi = <-cs.peerMsgQueue:
			cs.wal.Write(mi)
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			cs.handleMsg(mi)
		case mi = <-cs.internalMsgQueue:
			cs.wal.WriteSync(mi) // NOTE: fsync
			// handles proposals, block parts, votes
			cs.handleMsg(mi)
		case <-cs.timeoutTimer.C:
			now := time.Now()
			recover := cs.StartTime.Add(timeoutRecover)
			cs.Logger.Info("Recover timer", "nextRecover", recover, "stepRecover", cs.stepRecover)
			if now.Before(recover) {
				cs.timeoutTimer.Reset(recover.Sub(now))

			} else if !cs.stepRecover {
				cs.timeoutTimer.Reset(timeoutRecover)
				cs.stepRecover = true
				cs.Logger.Warn("Recover", "lastTime", cs.StartTime)
				cs.Validators = types.NewValidatorSet(cs.appmgr.GetRecoverValidators(cs.Height - 1))
				cs.Votes = cstypes.NewHeightVoteSet(cs.status.ChainID, cs.Height, cs.Validators)
				cs.handleTimeout(timeoutInfo{
					Duration: now.Sub(cs.CommitTime),
					Height:   cs.Height,
					Round:    cs.Round,
					Step:     cstypes.RoundStepRecover,
				}, rs)
			}
		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			cs.wal.Write(ti)
			// if the timeout is relevant to the rs
			// go to the next step
			cs.handleTimeout(ti, rs)
		case <-cs.Quit():

			// NOTE: the internalMsgQueue may have signed messages from our
			// priv_val that haven't hit the WAL, but its ok because
			// priv_val tracks LastSig

			// close wal now that we're done writing to it
			cs.wal.Stop()
			cs.wal.Wait()

			close(cs.done) // keep an eye on "close of closed channel"
			return
		}
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *ConsensusState) handleMsg(mi msgInfo) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	var err error
	msg, peerID := mi.Msg, mi.PeerID
	switch msg := msg.(type) {
	case *ProposalMessage:
		// will not cause transition.
		// once proposal is set, we can receive block parts
		err = cs.setProposal(msg.Proposal)
	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		_, err = cs.addProposalBlockPart(msg, peerID)
		if err != nil && msg.Round != cs.Round {
			cs.Logger.Debug("Received block part from wrong round", "height", cs.Height, "csRound", cs.Round, "blockRound", msg.Round)
			err = nil
		}
	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		err := cs.tryAddVote(msg.Vote, peerID)
		if err == ErrAddingVote {
			// TODO: punish peer
			// We probably don't want to stop the peer here. The vote does not
			// necessarily comes from a malicious peer but can be just broadcasted by
			// a typical peer.
		}

		// NOTE: the vote is broadcast to peers by the reactor listening
		// for vote events

		// TODO: If rs.Height == vote.Height && rs.Round < vote.Round,
		// the peer is sending us CatchupCommit precommits.
		// We could make note of this and help filter in broadcastHasVoteMessage().
	default:
		cs.Logger.Error("Unknown msg type", reflect.TypeOf(msg))
	}
	if err != nil {
		cs.Logger.Error("Error with msg", "height", cs.Height, "round", cs.Round, "type", reflect.TypeOf(msg), "peer", peerID, "err", err, "msg", msg)
	}
}

func (cs *ConsensusState) handleTimeout(ti timeoutInfo, rs cstypes.RoundState) {
	cs.Logger.Debug("Received tock", "timeout", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)

	// timeouts must be for current height, round, step
	if ti.Height != rs.Height || ti.Round < rs.Round || (ti.Round == rs.Round && ti.Step < rs.Step) {
		cs.Logger.Debug("Ignoring tock because we're ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case cstypes.RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		// XXX: should we fire timeout here (for timeout commit)?
		cs.enterNewRound(ti.Height, 0)
	case cstypes.RoundStepNewRound:
		cs.enterPropose(ti.Height, 0)
	case cstypes.RoundStepPropose:
		cs.eventBus.PublishEventTimeoutPropose(cs.RoundStateEvent())
		cs.enterPrevote(ti.Height, ti.Round)
	case cstypes.RoundStepPrevoteWait:
		cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent())
		cs.enterPrecommit(ti.Height, ti.Round)
	case cstypes.RoundStepPrecommitWait:
		cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent())
		cs.enterNewRound(ti.Height, ti.Round+1)
	case cstypes.RoundStepRecover:
		cs.enterNewRound(ti.Height, ti.Round+1)
	default:
		panic(cmn.Fmt("Invalid timeout step: %v", ti.Step))
	}

}

func (cs *ConsensusState) handleTxsAvailable() {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	// we only need to do this for round 0
	cs.enterPropose(cs.Height, 0)
}

//-----------------------------------------------------------------------------
// State functions
// Used internally by handleTimeout and handleMsg to make state transitions

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
// 	or, if SkipTimeout==true, after receiving all precommits from (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (cs *ConsensusState) enterNewRound(height uint64, round int) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != cstypes.RoundStepNewHeight) {
		logger.Debug(cmn.Fmt("enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	if now := time.Now(); cs.StartTime.After(now) {
		logger.Info("Need to set a buffer and log message here for sanity.", "startTime", cs.StartTime, "now", now)
	}

	logger.Info(cmn.Fmt("enterNewRound(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {
		validators = validators.Copy()
		validators.IncrementAccum(round - cs.Round)
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, cstypes.RoundStepNewRound)
	cs.Validators = validators
	if round == 0 {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		logger.Info("Resetting Proposal info")
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
	}
	cs.Votes.SetRound(round + 1) // also track next round (round+1) to allow round-skipping

	cs.eventBus.PublishEventNewRound(cs.RoundStateEvent())
	cs.metrics.Rounds.Set(float64(round))

	// Wait for txs to be available in the mempool
	// before we enterPropose in round 0. If the last block changed the app hash,
	// we may need an empty "proof" block, and enterPropose immediately.
	waitForTxs := cs.config.WaitForTxs() && round == 0 && !cs.stepRecover /*&& !cs.needProofBlock(height)*/
	if waitForTxs {
		if cs.config.CreateEmptyBlocksInterval > 0 {
			cs.scheduleTimeout(cs.config.EmptyBlocksInterval(), height, round, cstypes.RoundStepNewRound)
		}
		go cs.proposalHeartbeat(height, round)
	} else {
		cs.enterPropose(height, round)
	}
}

/*
// needProofBlock returns true on the first height (so the genesis app hash is signed right away)
// and where the last block (height-1) caused the app hash to change
func (cs *ConsensusState) needProofBlock(height uint64) bool {
	if height == types.BlockHeightOne {
		return true
	}

	lastBlockMeta := cs.appmgr.LoadBlockMeta(height - 1)
	return !bytes.Equal(cs.status.ParentHash, lastBlockMeta.Header.ParentHash.Bytes())
}*/

func (cs *ConsensusState) proposalHeartbeat(height uint64, round int) {
	counter := 0
	addr := cs.privValidator.GetAddress()
	valIndex, _ := cs.Validators.GetByAddress(addr)
	chainID := cs.status.ChainID
	for {
		rs := cs.GetRoundState()
		// if we've already moved on, no need to send more heartbeats
		if rs.Step > cstypes.RoundStepNewRound || rs.Round > round || rs.Height > height {
			return
		}
		heartbeat := &types.Heartbeat{
			Height:           rs.Height,
			Round:            rs.Round,
			Sequence:         counter,
			ValidatorAddress: addr,
			ValidatorIndex:   valIndex,
		}
		cs.privValidator.SignHeartbeat(chainID, heartbeat)
		cs.eventBus.PublishEventProposalHeartbeat(types.EventDataProposalHeartbeat{heartbeat})
		cs.evsw.FireEvent(types.EventProposalHeartbeat, heartbeat)
		counter++
		time.Sleep(proposalHeartbeatIntervalSeconds * time.Second)
	}
}

// Enter (CreateEmptyBlocks): from enterNewRound(height,round)
// Enter (CreateEmptyBlocks, CreateEmptyBlocksInterval > 0 ): after enterNewRound(height,round), after timeout of CreateEmptyBlocksInterval
// Enter (!CreateEmptyBlocks) : after enterNewRound(height,round), once txs are in the mempool
func (cs *ConsensusState) enterPropose(height uint64, round int) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPropose <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPropose(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(cmn.Fmt("enterPropose(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPropose:
		cs.updateRoundStep(round, cstypes.RoundStepPropose)
		cs.newStep()

		// If we have the whole proposal + POL, then goto Prevote now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		// or else after timeoutPropose
		if cs.isProposalComplete() {
			cs.enterPrevote(height, cs.Round)
		}
	}()

	// If we don't get the proposal and all block parts quick enough, enterPrevote
	wait := cs.config.Propose(round)
	if cs.recover == 1 {
		wait = recoverFirstProposal
	}
	cs.scheduleTimeout(wait, height, round, cstypes.RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("This node is not a validator")
		return
	}

	// if not a validator, we're done
	if !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
		logger.Debug("This node is not a validator", "addr", cs.privValidator.GetAddress(), "vals", cs.Validators)
		return
	}
	logger.Debug("This node is a validator")

	if cs.isProposer() {
		logger.Info("enterPropose: Our turn to propose", "proposer", cs.Validators.GetProposer().Address, "privValidator", cs.privValidator)
		cs.decideProposal(height, round)
	} else {
		logger.Info("enterPropose: Not our turn to propose", "proposer", cs.Validators.GetProposer().Address, "privValidator", cs.privValidator)
	}
}

func (cs *ConsensusState) isProposer() bool {
	return bytes.Equal(cs.Validators.GetProposer().Address, cs.privValidator.GetAddress())
}

func (cs *ConsensusState) defaultDecideProposal(height uint64, round int) {
	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block
	if cs.LockedBlock != nil {
		// If we're locked onto a block, just choose that.
		block, blockParts = cs.LockedBlock, cs.LockedBlockParts
	} else if cs.ValidBlock != nil {
		// If there is valid block, choose that.
		block, blockParts = cs.ValidBlock, cs.ValidBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		block, blockParts = cs.createProposalBlock()
		if block == nil { // on error
			return
		}
	}

	// Make proposal
	polRound, polBlockID := cs.Votes.POLInfo()
	proposal := types.NewProposal(height, round, blockParts.Header(), polRound, polBlockID)
	proposal.Type = types.ProposalTypeNormal
	if cs.stepRecover {
		proposal.Type = types.ProposalTypeRecover
	}
	if err := cs.privValidator.SignProposal(cs.status.ChainID, proposal); err == nil {
		// Set fields
		/*  fields set by setProposal and addBlockPart
		cs.Proposal = proposal
		cs.ProposalBlock = block
		cs.ProposalBlockParts = blockParts
		*/

		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
		for i := 0; i < blockParts.Total(); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}
		cs.Logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
		cs.Logger.Debug(cmn.Fmt("Signed proposal block: %v", block))
	} else {
		if !cs.replayMode {
			cs.Logger.Error("enterPropose: Error signing proposal", "height", height, "round", round, "err", err)
		}
	}
}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there).
func (cs *ConsensusState) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if cs.Proposal.POLRound < 0 {
		return true
	}
	// if this is false the proposer is lying or we haven't received the POL yet
	return cs.Votes.Prevotes(cs.Proposal.POLRound).HasTwoThirdsMajority()

}

// Create the next block to propose and return it.
// Returns nil block upon error.
// NOTE: keep it side-effect free for clarity.
func (cs *ConsensusState) createProposalBlock() (*types.Block, *types.PartSet) {
	var commit *types.Commit
	if cs.Height == types.BlockHeightOne {
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		commit = &types.Commit{}
	} else if cs.LastCommit.HasTwoThirdsMajority() {
		// Make the commit from LastCommit
		commit = cs.LastCommit.MakeCommit()
	} else {
		// This shouldn't happen.
		cs.Logger.Error("enterPropose: Cannot propose anything: No commit for the previous block.")
		return nil, nil
	}

	maxTxs := cs.status.ConsensusParams.BlockSize.MaxTxs
	if cs.stepRecover {
		cs.Logger.Warn("Recover", "times", cs.recover)
		maxTxs = 0
	}

	coinbase := cs.Validators.GetProposer().CoinBase
	block := cs.appmgr.CreateBlock(cs.Height, maxTxs, cs.status.ConsensusParams.BlockSize.MaxGas, uint64(time.Now().Unix()))
	if block == nil {
		return nil, nil
	}

	//we use the deploy address as coinbase
	block.Header.Coinbase = coinbase

	evidence := cs.evpool.PendingEvidence()
	block.AddEvidence(evidence)
	if evi := cs.getLastFaultValsInfo(commit); evi != nil {
		block.AddEvidence([]types.Evidence{evi})
	}
	block.Recover = cs.recover
	block.ChainID = cs.status.ChainID
	block.LastCommit = commit
	block.LastBlockID = cs.status.LastBlockID
	block.LastCommitHash = block.LastCommit.Hash()
	block.EvidenceHash = block.Evidence.Hash()
	//NOT right,should be update data TODO
	block.ConsensusHash = common.BytesToHash(cs.status.ConsensusParams.Hash())
	block.ValidatorsHash = common.BytesToHash(cs.status.Validators.Hash())
	cs.appmgr.PreRunBlock(block)
	return block, block.MakePartSet(cs.status.ConsensusParams.BlockGossip.BlockPartSizeBytes)
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// Enter: any +2/3 prevotes for future round.
// Prevote for LockedBlock if we're locked, or ProposalBlock if valid.
// Otherwise vote nil.
func (cs *ConsensusState) enterPrevote(height uint64, round int) {
	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrevote <= cs.Step) {
		cs.Logger.Debug(cmn.Fmt("enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, cstypes.RoundStepPrevote)
		cs.newStep()
	}()

	// fire event for how we got here
	if cs.isProposalComplete() {
		cs.eventBus.PublishEventCompleteProposal(cs.RoundStateEvent())
	} else {
		// we received +2/3 prevotes for a future round
		// TODO: catchup event?
	}

	cs.Logger.Info(cmn.Fmt("enterPrevote(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	// Sign and broadcast vote as necessary
	cs.doPrevote(height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *ConsensusState) defaultDoPrevote(height uint64, round int) {
	logger := cs.Logger.With("height", height, "round", round)
	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Block was locked")
		cs.signAddVote(types.VoteTypePrevote, cs.LockedBlock.Hash().Bytes(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Info("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	if !cs.checkBlockEvidence(cs.ProposalBlock) {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: Evidence of cs.ProposalBlock is invalid")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Validate proposal block
	ok := cs.appmgr.CheckBlock(cs.ProposalBlock)
	if !ok {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: ProposalBlock is invalid")
		cs.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	logger.Info("enterPrevote: ProposalBlock is valid")
	cs.signAddVote(types.VoteTypePrevote, cs.ProposalBlock.Hash().Bytes(), cs.ProposalBlockParts.Header())
}

// Enter: any +2/3 prevotes at next round.
func (cs *ConsensusState) enterPrevoteWait(height uint64, round int) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrevoteWait <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		cmn.PanicSanity(cmn.Fmt("enterPrevoteWait(%v/%v), but Prevotes does not have any +2/3 votes", height, round))
	}
	logger.Info(cmn.Fmt("enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.config.Prevote(round), height, round, cstypes.RoundStepPrevoteWait)
}

// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: +2/3 precomits for block or nil.
// Enter: any +2/3 precommits for next round.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, unlock an existing lock and precommit nil if +2/3 of prevotes were nil,
// else, precommit nil otherwise.
func (cs *ConsensusState) enterPrecommit(height uint64, round int) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrecommit <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}

	logger.Info(cmn.Fmt("enterPrecommit(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, cstypes.RoundStepPrecommit)
		cs.newStep()
	}()

	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	cs.eventBus.PublishEventPolka(cs.RoundStateEvent())

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		cmn.PanicSanity(cmn.Fmt("This POLRound should be %v but got %", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if blockID.IsZero() {
		if cs.LockedBlock == nil {
			logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = 0
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}
		cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash.Bytes()) {
		logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		cs.eventBus.PublishEventRelock(cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID.Hash.Bytes(), blockID.PartsHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash.Bytes()) {
		logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID.Hash)
		// Validate the block.
		if !cs.checkBlockEvidence(cs.ProposalBlock) {
			// ProposalBlock is invalid.
			cmn.PanicConsensus(cmn.Fmt("enterPrecommit: +2/3 prevoted for an invalid block evidence"))
		}
		if ok := cs.appmgr.CheckBlock(cs.ProposalBlock); !ok {
			cmn.PanicConsensus(cmn.Fmt("enterPrecommit: +2/3 prevoted for an invalid block"))
		}

		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.eventBus.PublishEventLock(cs.RoundStateEvent())
		cs.signAddVote(types.VoteTypePrecommit, blockID.Hash.Bytes(), blockID.PartsHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	// TODO: In the future save the POL prevotes for justification.
	cs.LockedRound = 0
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
	}
	cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
	cs.signAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
}

// Enter: any +2/3 precommits for next round.
func (cs *ConsensusState) enterPrecommitWait(height uint64, round int) {
	logger := cs.Logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrecommitWait <= cs.Step) {
		logger.Debug(cmn.Fmt("enterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))
		return
	}
	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		cmn.PanicSanity(cmn.Fmt("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
	}
	logger.Info(cmn.Fmt("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterPrecommitWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrecommitWait)
		cs.newStep()
	}()

	// Wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.config.Precommit(round), height, round, cstypes.RoundStepPrecommitWait)

}

// Enter: +2/3 precommits for block
func (cs *ConsensusState) enterCommit(height uint64, commitRound int) {
	logger := cs.Logger.With("height", height, "commitRound", commitRound)

	if cs.Height != height || cstypes.RoundStepCommit <= cs.Step {
		logger.Debug(cmn.Fmt("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))
		return
	}
	logger.Info(cmn.Fmt("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, cs.Height, cs.Round, cs.Step))

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		cs.updateRoundStep(cs.Round, cstypes.RoundStepCommit)
		cs.CommitRound = commitRound
		cs.CommitTime = time.Now()
		cs.newStep()

		// Maybe finalize immediately.
		cs.tryFinalizeCommit(height)
	}()

	blockID, ok := cs.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		cmn.PanicSanity("RunActionCommit() expects +2/3 precommits")
	}

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they'll be cleared in updateToStatus.
	if cs.LockedBlock.HashesTo(blockID.Hash.Bytes()) {
		logger.Info("Commit is for locked block. Set ProposalBlock=LockedBlock", "blockHash", blockID.Hash)
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
	}

	// If we don't have the block being committed, set up to get it.
	if !cs.ProposalBlock.HashesTo(blockID.Hash.Bytes()) {
		if !cs.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
			logger.Info("Commit is for a block we don't know about. Set ProposalBlock=nil", "proposal", cs.ProposalBlock.Hash(), "commit", blockID.Hash)
			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
		} else {
			// We just need to keep waiting.
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *ConsensusState) tryFinalizeCommit(height uint64) {
	logger := cs.Logger.With("height", height)

	if cs.Height != height {
		cmn.PanicSanity(cmn.Fmt("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	if !ok || blockID.IsZero() {
		logger.Error("Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.")
		return
	}
	if !cs.ProposalBlock.HashesTo(blockID.Hash.Bytes()) {
		// TODO: this happens every time if we're not a validator (ugly logs)
		// TODO: ^^ wait, why does it matter that we're a validator?
		logger.Info("Attempt to finalize failed. We don't have the commit block.", "proposal-block", cs.ProposalBlock.Hash(), "commit-block", blockID.Hash)
		return
	}

	//	go
	cs.finalizeCommit(height)
}

// Increment height and goto cstypes.RoundStepNewHeight
func (cs *ConsensusState) finalizeCommit(height uint64) {
	if cs.Height != height || cs.Step != cstypes.RoundStepCommit {
		cs.Logger.Debug(cmn.Fmt("finalizeCommit(%v): Invalid args. Current step: %v/%v/%v", height, cs.Height, cs.Round, cs.Step))
		return
	}

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	block, blockParts := cs.ProposalBlock, cs.ProposalBlockParts

	if !ok {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, commit does not have two thirds majority"))
	}
	if !blockParts.HasHeader(blockID.PartsHeader) {
		cmn.PanicSanity(cmn.Fmt("Expected ProposalBlockParts header to be commit header"))
	}
	if !block.HashesTo(blockID.Hash.Bytes()) {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	if !cs.checkBlockEvidence(block) {
		// ProposalBlock is invalid.
		cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block evidence"))
	}
	if ok := cs.appmgr.CheckBlock(block); !ok {
		cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block"))
	}
	cs.Logger.Info(cmn.Fmt("Finalizing commit of block with %d txs", block.NumTxs),
		"height", block.Height, "hash", block.Hash())
	cs.Logger.Info(cmn.Fmt("%v", block))

	fail.Fail() // XXX

	// Save to blockStore.
	validators := make([]*types.Validator, 0)
	if cs.appmgr.Height() < block.Height {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		precommits := cs.Votes.Precommits(cs.CommitRound)
		seenCommit := precommits.MakeCommit()
		var err error
		validators, err = cs.appmgr.CommitBlock(block, blockParts, seenCommit, false)
		if err != nil {
			cs.Logger.Warn("finalizeCommit: CommitBlock failed", "height", block.Height)
			cs.Logger.Report("finalizeCommit", "logID", types.LogIdCommitBlockFail, "height", block.Height, "err", err)
			cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block, err:%v", err))
		}
	} else {
		// Happens during replay if we already saved the block but didn't commit
		validators = cs.appmgr.GetValidators(block.Height)
		cs.Logger.Info("Calling finalizeCommit on already stored block", "height", block.Height)
	}

	fail.Fail() // XXX

	// Write EndHeightMessage{} for this height, implying that the blockstore
	// has saved the block.
	//
	// If we crash before writing this EndHeightMessage{}, we will recover by
	// running ApplyBlock during the ABCI handshake when we restart.  If we
	// didn't save the block to the blockstore before writing
	// EndHeightMessage{}, we'd have to change WAL replay -- currently it
	// complains about replaying for heights where an #ENDHEIGHT entry already
	// exists.
	//
	// Either way, the ConsensusState should not be resumed until we
	// successfully call ApplyBlock (ie. later here, or in Handshake after
	// restart).
	cs.wal.WriteSync(EndHeightMessage{height}) // NOTE: fsync

	fail.Fail() // XXX

	// Create a copy of the status for staging and an event cache for txs.
	statusCopy := cs.status.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	var err error
	statusCopy, err = cs.blockExec.ApplyBlock(statusCopy, types.BlockID{block.Hash(), blockParts.Header()}, block, validators)
	if err != nil {
		cs.Logger.Error("Error on ApplyBlock. Did the application crash? Please restart", "err", err)
		err := cmn.Kill()
		if err != nil {
			cs.Logger.Error("Failed to kill this process - please do so manually", "err", err)
		}
		return
	}

	fail.Fail() // XXX

	// must be called before we update state
	//cs.recordMetrics(height, block)

	// NewHeightStep!
	cs.updateToStatus(statusCopy)

	fail.Fail() // XXX

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0(&cs.RoundState)

	// By here,
	// * cs.Height has been increment to height+1
	// * cs.Step is now cstypes.RoundStepNewHeight
	// * cs.StartTime is set to when we will start round0.
}

func (cs *ConsensusState) recordMetrics(height uint64, block *types.Block) {
	cs.metrics.Validators.Set(float64(cs.Validators.Size()))
	cs.metrics.ValidatorsPower.Set(float64(cs.Validators.TotalVotingPower()))
	missingValidators := 0
	missingValidatorsPower := int64(0)
	for i, val := range cs.Validators.Validators {
		var vote *types.Vote
		if i < len(block.LastCommit.Precommits) {
			vote = block.LastCommit.Precommits[i]
		}
		if vote == nil {
			missingValidators++
			missingValidatorsPower += val.VotingPower
		}
	}
	cs.metrics.MissingValidators.Set(float64(missingValidators))
	cs.metrics.MissingValidatorsPower.Set(float64(missingValidatorsPower))
	cs.metrics.ByzantineValidators.Set(float64(len(block.Evidence.Evidence)))
	byzantineValidatorsPower := int64(0)
	for _, ev := range block.Evidence.Evidence {
		if _, ok := ev.(*types.FaultValidatorsEvidence); ok {
			continue
		}
		if _, val := cs.Validators.GetByAddress(ev.Address()); val != nil {
			byzantineValidatorsPower += val.VotingPower
		}
	}
	cs.metrics.ByzantineValidatorsPower.Set(float64(byzantineValidatorsPower))

	if height > types.BlockHeightOne {
		//lastBlockMeta := cs.blockStore.LoadBlockMeta(height - 1)
		//cs.metrics.BlockIntervalSeconds.Observe(
		//	block.Time.Sub(lastBlockMeta.Header.Time).Seconds(),
		//)
	}

	cs.metrics.NumTxs.Set(float64(block.NumTxs))
	cs.metrics.BlockSizeBytes.Set(float64(block.Size()))
	cs.metrics.TotalTxs.Set(float64(block.TotalTxs))
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) defaultSetProposal(proposal *types.Proposal) error {
	// Already have one
	// TODO: possibly catch double proposals
	if cs.Proposal != nil {
		return nil
	}

	if proposal.Type == types.ProposalTypeRecover && !cs.stepRecover {
		cs.Logger.Warn("Received recover proposal", "proposal", proposal)
		if proposal.Height != cs.Height || proposal.Round <= cs.Round {
			cs.Logger.Warn("Height or Round mismatch", "Height", cs.Height, "Round", cs.Round)
			return nil
		}
		if cs.StartTime.Add(timeoutRecoverLimit).After(time.Now()) {
			cs.Logger.Warn("It`s not the right time", "limit", cs.StartTime.Add(timeoutRecoverLimit))
			return ErrInvalidProposalPOLRound
		}
		cs.timeoutTimer.Reset(timeoutRecover)
		cs.stepRecover = true
		cs.Logger.Warn("Recover", "lastTime", cs.StartTime)
		cs.Validators = types.NewValidatorSet(cs.appmgr.GetRecoverValidators(cs.Height - 1))
		cs.Votes = cstypes.NewHeightVoteSet(cs.status.ChainID, cs.Height, cs.Validators)
		cs.enterNewRound(cs.Height, cs.Round+1)
	}

	// Does not apply
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// We don't care about the proposal if we're already in cstypes.RoundStepCommit.
	if cstypes.RoundStepCommit <= cs.Step {
		return nil
	}

	// Verify POLRound, which must be -1 or between 0 and proposal.Round exclusive.
	if proposal.POLRound != -1 &&
		(proposal.POLRound < 0 || proposal.Round <= proposal.POLRound) {
		return ErrInvalidProposalPOLRound
	}

	// Verify signature
	if !cs.Validators.GetProposer().PubKey.VerifyBytes(proposal.SignBytes(cs.status.ChainID), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	cs.Proposal = proposal
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockPartsHeader)
	cs.Logger.Info("Received proposal", "proposal", proposal)
	return nil
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit, once we have the full block.
func (cs *ConsensusState) addProposalBlockPart(msg *BlockPartMessage, peerID string) (added bool, err error) {
	height, round, part := msg.Height, msg.Round, msg.Part

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		cs.Logger.Debug("Received block part from wrong height", "height", height, "cs.Height", cs.Height, "round", round)
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.Logger.Info("Received a block part when we're not expecting any",
			"height", height, "round", round, "index", part.Index, "peer", peerID)
		return false, nil
	}

	added, err = cs.ProposalBlockParts.AddPart(part)
	if err != nil {
		return added, err
	}
	if added && cs.ProposalBlockParts.IsComplete() {
		// Added and completed!
		_, err = ser.DecodeReader(cs.ProposalBlockParts.GetReader(), &cs.ProposalBlock, int64(cs.status.ConsensusParams.BlockSize.MaxBytes))
		if err != nil {
			return true, err
		}

		if cs.ProposalBlock.Recover != cs.recover {
			cs.Logger.Warn("Received a block recover state different with local",
				"height", height, "round", round, "BlockRecover", cs.ProposalBlock.Recover, "local", cs.recover)
			cs.ProposalBlock = nil
			return false, nil
		}

		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		cs.Logger.Info("Received complete proposal block", "height", cs.ProposalBlock.Height, "hash", cs.ProposalBlock.Hash())

		// Update Valid* if we can.
		prevotes := cs.Votes.Prevotes(cs.Round)
		blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
		if hasTwoThirds && !blockID.IsZero() && (cs.ValidRound < cs.Round) {
			if cs.ProposalBlock.HashesTo(blockID.Hash.Bytes()) {
				cs.Logger.Info("Updating valid block to new proposal block",
					"valid-round", cs.Round, "valid-block-hash", cs.ProposalBlock.Hash())
				cs.ValidRound = cs.Round
				cs.ValidBlock = cs.ProposalBlock
				cs.ValidBlockParts = cs.ProposalBlockParts
			}
			// TODO: In case there is +2/3 majority in Prevotes set for some
			// block and cs.ProposalBlock contains different block, either
			// proposer is faulty or voting power of faulty processes is more
			// than 1/3. We should trigger in the future accountability
			// procedure at this point.
		}

		if cs.Step <= cstypes.RoundStepPropose && cs.isProposalComplete() {
			// Move onto the next step
			cs.enterPrevote(height, cs.Round)
			if hasTwoThirds { // this is optimisation as this will be triggered when prevote is added
				cs.enterPrecommit(height, cs.Round)
			}
		} else if cs.Step == cstypes.RoundStepCommit {
			// If we're waiting on the proposal block...
			cs.tryFinalizeCommit(height)
		}
		return true, nil
	}
	return added, nil
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *ConsensusState) tryAddVote(vote *types.Vote, peerID string) error {
	_, err := cs.addVote(vote, peerID)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, add it to the cs.evpool.
		// If it's otherwise invalid, punish peer.
		if err == ErrVoteHeightMismatch {
			return err
		} else if voteErr, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if bytes.Equal(vote.ValidatorAddress, cs.privValidator.GetAddress()) {
				cs.Logger.Error("Found conflicting vote from ourselves. Did you unsafe_reset a validator?", "height", vote.Height, "round", vote.Round, "type", vote.Type)
				return err
			}
			cs.evpool.AddEvidence(voteErr.DuplicateVoteEvidence)
			cs.Logger.Report("add vote", "logID", types.LogIdIllegalValidator, "height", vote.Height, "peer", peerID, "err", voteErr.Error())
			return err
		} else {
			// Probably an invalid signature / Bad peer.
			// Seems this can also err sometimes with "Unexpected step" - perhaps not from a bad peer ?
			cs.Logger.Error("Error attempting to add vote", "err", err)
			return ErrAddingVote
		}
	}
	return nil
}

//-----------------------------------------------------------------------------

func (cs *ConsensusState) addVote(vote *types.Vote, peerID string) (added bool, err error) {
	cs.Logger.Debug("addVote", "voteHeight", vote.Height, "voteType", vote.Type, "valIndex", vote.ValidatorIndex, "csHeight", cs.Height)

	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height+1 == cs.Height {
		if !(cs.Step == cstypes.RoundStepNewHeight && vote.Type == types.VoteTypePrecommit) {
			// TODO: give the reason ..
			// fmt.Errorf("tryAddVote: Wrong height, not a LastCommit straggler commit.")
			return added, ErrVoteHeightMismatch
		}
		added, err = cs.LastCommit.AddVote(vote)
		if !added {
			return added, err
		}

		cs.Logger.Info(cmn.Fmt("Added to lastPrecommits: %v", cs.LastCommit.StringShort()))
		cs.eventBus.PublishEventVote(types.EventDataVote{vote})
		cs.evsw.FireEvent(types.EventVote, vote)

		// if we can skip timeoutCommit and have all the votes now,
		if cs.config.SkipTimeoutCommit && cs.LastCommit.HasAll() {
			// go straight to new round (skip timeout commit)
			// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
			cs.enterNewRound(cs.Height, 0)
		}

		return
	}

	// Height mismatch is ignored.
	// Not necessarily a bad peer, but not favourable behaviour.
	if vote.Height != cs.Height {
		err = ErrVoteHeightMismatch
		cs.Logger.Info("Vote ignored and not added", "voteHeight", vote.Height, "csHeight", cs.Height, "err", err)
		return
	}

	height := cs.Height
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	}

	cs.eventBus.PublishEventVote(types.EventDataVote{vote})
	cs.evsw.FireEvent(types.EventVote, vote)

	switch vote.Type {
	case types.VoteTypePrevote:
		prevotes := cs.Votes.Prevotes(vote.Round)
		cs.Logger.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())

		// If +2/3 prevotes for a block or nil for *any* round:
		if blockID, ok := prevotes.TwoThirdsMajority(); ok {

			// There was a polka!
			// If we're locked but this is a recent polka, unlock.
			// If it matches our ProposalBlock, update the ValidBlock

			// Unlock if `cs.LockedRound < vote.Round <= cs.Round`
			// NOTE: If vote.Round > cs.Round, we'll deal with it when we get to vote.Round
			if (cs.LockedBlock != nil) &&
				(cs.LockedRound < vote.Round) &&
				(vote.Round <= cs.Round) &&
				!cs.LockedBlock.HashesTo(blockID.Hash.Bytes()) {

				cs.Logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
				cs.LockedRound = 0
				cs.LockedBlock = nil
				cs.LockedBlockParts = nil
				cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
			}

			// Update Valid* if we can.
			// NOTE: our proposal block may be nil or not what received a polka..
			// TODO: we may want to still update the ValidBlock and obtain it via gossipping
			if !blockID.IsZero() &&
				(cs.ValidRound < vote.Round) &&
				(vote.Round <= cs.Round) &&
				cs.ProposalBlock.HashesTo(blockID.Hash.Bytes()) {

				cs.Logger.Info("Updating ValidBlock because of POL.", "validRound", cs.ValidRound, "POLRound", vote.Round)
				cs.ValidRound = vote.Round
				cs.ValidBlock = cs.ProposalBlock
				cs.ValidBlockParts = cs.ProposalBlockParts
			}
		}

		// If +2/3 prevotes for *anything* for this or future round:
		if cs.Round <= vote.Round && prevotes.HasTwoThirdsAny() {
			// Round-skip over to PrevoteWait or goto Precommit.
			cs.enterNewRound(height, vote.Round) // if the vote is ahead of us
			if prevotes.HasTwoThirdsMajority() {
				cs.enterPrecommit(height, vote.Round)
			} else {
				cs.enterPrevote(height, vote.Round) // if the vote is ahead of us
				cs.enterPrevoteWait(height, vote.Round)
			}
		} else if cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == vote.Round {
			// If the proposal is now complete, enter prevote of cs.Round.
			if cs.isProposalComplete() {
				cs.enterPrevote(height, cs.Round)
			}
		}

	case types.VoteTypePrecommit:
		precommits := cs.Votes.Precommits(vote.Round)
		cs.Logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())
		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			if blockID.Hash == common.EmptyHash {
				cs.enterNewRound(height, vote.Round+1)
			} else {
				cs.enterNewRound(height, vote.Round)
				cs.enterPrecommit(height, vote.Round)
				cs.enterCommit(height, vote.Round)

				if cs.config.SkipTimeoutCommit && precommits.HasAll() {
					// if we have all the votes now,
					// go straight to new round (skip timeout commit)
					// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
					cs.enterNewRound(cs.Height, 0)
				}

			}
		} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(height, vote.Round)
			cs.enterPrecommit(height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}
	default:
		panic(cmn.Fmt("Unexpected vote type %X", vote.Type)) // go-wire should prevent this.
	}

	return
}

func (cs *ConsensusState) signVote(type_ byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	addr := cs.privValidator.GetAddress()
	valIndex, _ := cs.Validators.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   valIndex,
		ValidatorSize:    cs.Validators.Size(),
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        time.Now().UTC(),
		Type:             type_,
		BlockID:          types.BlockID{common.BytesToHash(hash), header},
	}
	err := cs.privValidator.SignVote(cs.status.ChainID, vote)
	return vote, err
}

// sign the vote and publish on internalMsgQueue
func (cs *ConsensusState) signAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	// if we don't have a key or we're not in the validator set, do nothing
	if cs.privValidator == nil || !cs.Validators.HasAddress(cs.privValidator.GetAddress()) {
		return nil
	}
	vote, err := cs.signVote(type_, hash, header)
	if err == nil {
		cs.sendInternalMessage(msgInfo{&VoteMessage{vote}, ""})
		cs.Logger.Info("Signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
		return vote
	}
	//if !cs.replayMode {
	cs.Logger.Error("Error signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
	//}
	return nil
}

//---------------------------------------------------------

func CompareHRS(h1 uint64, r1 int, s1 cstypes.RoundStepType, h2 uint64, r2 int, s2 cstypes.RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}

func (cs *ConsensusState) DeleteHistoricalData(keepLatestBlocks uint64) {
	minHeight := cs.startDeleteHeight
	if minHeight == 0 {
		minHeight := loadStartDeleteHeight(cs.blockExec.db)
		cs.startDeleteHeight = minHeight
	}

	cs.mtx.Lock()
	maxHeight := cs.Height
	cs.mtx.Unlock()

	if maxHeight < minHeight+keepLatestBlocks {
		return
	}

	for i := 0; minHeight <= maxHeight; minHeight++ {
		cs.blockExec.db.Delete(calcValidatorsKey(minHeight))
		cs.blockExec.db.Delete(calcConsensusParamsKey(minHeight))
		i++
		if (i % 500) == 0 {
			time.Sleep(59 * time.Millisecond)
		}
	}

	cs.startDeleteHeight = minHeight
	saveStartDeleteHeight(cs.blockExec.db, minHeight)
}
