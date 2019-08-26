package blockchain

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	cs "github.com/lianxiangcloud/linkchain/consensus"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

const (
	// BlockchainChannel is a channel for blocks and status updates
	BlockchainChannel = byte(0x40)

	trySyncIntervalMS = 50
	// stop syncing when last block's time is
	// within this much of the system time.
	// stopSyncingDurationMinutes = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 1

	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
	maxLaggingBlocks = 5
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(cs.NewStatus, int)
	SwitchToFastSync()
}

type peerError struct {
	err    error
	peerID string
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	p2p.BaseReactor
	sw p2p.P2PManager

	// immutable
	initialStatus cs.NewStatus

	blockExec *cs.BlockExecutor
	appmgr    cs.BlockChainApp
	pool      *BlockPool
	fastSync  bool

	inFastSync  uint32
	canFastSync uint32

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(status cs.NewStatus, blockExec *cs.BlockExecutor, app cs.BlockChainApp,
	fastSync bool, p2pmanager p2p.P2PManager) *BlockchainReactor {

	if status.LastBlockHeight != app.Height() {
		panic(fmt.Sprintf("status (%v) and app (%v) height mismatch", status.LastBlockHeight,
			app.Height()))
	}

	const capacity = 1000 // must be bigger than peers count
	requestsCh := make(chan BlockRequest, capacity)
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	pool := NewBlockPool(
		app.Height()+1,
		requestsCh,
		errorsCh,
	)

	bcR := &BlockchainReactor{
		sw:            p2pmanager,
		initialStatus: status,
		blockExec:     blockExec,
		appmgr:        app,
		pool:          pool,
		fastSync:      fastSync,
		inFastSync:    0,
		canFastSync:   0,
		requestsCh:    requestsCh,
		errorsCh:      errorsCh,
	}
	bcR.BaseReactor = *p2p.NewBaseReactor("BlockchainReactor", bcR)
	return bcR
}

// SetLogger implements cmn.Service by setting the logger on reactor and pool.
func (bcR *BlockchainReactor) SetLogger(l log.Logger) {
	bcR.BaseService.Logger = l
	bcR.pool.Logger = l
}

// OnStart implements cmn.Service.
func (bcR *BlockchainReactor) OnStart() error {
	if err := bcR.BaseReactor.OnStart(); err != nil {
		return err
	}
	if bcR.fastSync {
		atomic.StoreUint32(&bcR.inFastSync, 1)
		err := bcR.pool.Start()
		if err != nil {
			return err
		}
		go bcR.poolRoutine()
	}
	go bcR.statusUpdateRoutine()
	return nil
}

func (bcR *BlockchainReactor) KeepFastSync(keepFastSync bool) {
	bcR.pool.NeverCaughtUp(keepFastSync)
}

func (bcR *BlockchainReactor) StopFastSync() bool {
	bcR.Logger.Info("StopFastSync trigger by conR.StopTheWorld()")
	atomic.StoreUint32(&bcR.canFastSync, 0)
	if atomic.LoadUint32(&bcR.inFastSync) == 1 {
		bcR.pool.MockCaughtUp(true)
		return true
	}
	return false
}

// Restart switches from consensus mode to fastSync mode.
func (bcR *BlockchainReactor) RestartFastSync(status cs.NewStatus) error {
	atomic.StoreUint32(&bcR.inFastSync, 1)
	bcR.Logger.Info("RestartFastSync trigger by conR.SwitchToFastSync")
	bcR.initialStatus = status
	bcR.pool.MockCaughtUp(false)
	bcR.pool.height = bcR.appmgr.Height() + 1
	if bcR.fastSync {
		err := bcR.pool.Reset()
		if err != nil {
			bcR.Logger.Info("bcR.pool.Reset failed")
			return err
		}
	} else {
		bcR.Logger.Info("RestartFastSync: set bcR.fastSync")
		bcR.fastSync = true
	}
	err := bcR.pool.Start()
	if err != nil {
		bcR.Logger.Info("bcR.pool.Start failed")
		return err
	}
	go bcR.poolRoutine()
	return nil
}

// OnStop implements cmn.Service.
func (bcR *BlockchainReactor) OnStop() {
	bcR.BaseReactor.OnStop()
	bcR.pool.Stop()
}

// GetChannels implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  1024 * 1024,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our status to peer.
func (bcR *BlockchainReactor) AddPeer(peer p2p.Peer) {
	msgBytes := encodeMsg(&bcStatusResponseMessage{bcR.appmgr.Height()}) //ser.MustEncodeToBytes()
	if !peer.Send(BlockchainChannel, msgBytes) {
		// doing nothing, will try later in `poolRoutine`
	}
	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.SetPeerHeight
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcR.pool.RemovePeer(peer.ID())
}

// respondToPeer loads a block and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// if all nodes are honest, no node should be requesting for a block
// that's non-existent.
func (bcR *BlockchainReactor) respondToPeer(msg *bcBlockRequestMessage,
	src p2p.Peer) (queued bool) {

	block := bcR.appmgr.LoadBlock(msg.Height)
	if block != nil {
		msgBytes := encodeMsg(&bcBlockResponseMessage{Block: block})
		return src.TrySend(BlockchainChannel, msgBytes)
	}

	bcR.Logger.Info("Peer asking for a block we don't have", "src", src, "height", msg.Height)

	msgBytes := encodeMsg(&bcNoBlockResponseMessage{Height: msg.Height})
	return src.TrySend(BlockchainChannel, msgBytes)
}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcR *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		bcR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		bcR.sw.StopPeerForError(src, err)
		return
	}

	bcR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg, "size", len(msgBytes))

	switch resp := msg.(type) {
	case *bcBlockRequestMessage:
		if queued := bcR.respondToPeer(resp, src); !queued {
			// Unfortunately not queued since the queue is full.
		}
	case *bcBlockResponseMessage:
		// Got a block.
		block := resp.Block
		resp.Block = nil
		bcR.Logger.Info("Receive", "src", src, "chID", chID, "height", block.Height, "size", len(msgBytes))
		bcR.pool.AddBlock(src.ID(), block, len(msgBytes))
	case *bcStatusRequestMessage:
		// Send peer our status.
		msgBytes := encodeMsg(&bcStatusResponseMessage{bcR.appmgr.Height()})
		queued := src.TrySend(BlockchainChannel, msgBytes)
		if !queued {
			// sorry
		}
	case *bcStatusResponseMessage:
		// Got a peer status. Unverified.
		bcR.pool.SetPeerHeight(src.ID(), resp.Height)
		bcR.Logger.Debug("recv bcStatusResponseMessage", "resp.Height", resp.Height, " appmgr.Height()", bcR.appmgr.Height(), "maxLaggingBlocks", maxLaggingBlocks,
			"canFastSync", bcR.canFastSync)
		if resp.Height > bcR.appmgr.Height()+maxLaggingBlocks && atomic.CompareAndSwapUint32(&bcR.canFastSync, 1, 0) {
			bcR.Logger.Info("Receive", "src", src, "chID", chID, "msg.Height", resp.Height)
			conR := bcR.sw.Reactor("CONSENSUS").(consensusReactor)
			conR.SwitchToFastSync()
		}
	default:
		bcR.Logger.Error(cmn.Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
// (Except for the SYNC_LOOP, which is the primary purpose and must be synchronous.)
func (bcR *BlockchainReactor) poolRoutine() {

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	switchToConsensusTicker := time.NewTicker(switchToConsensusIntervalSeconds * time.Second)

	blocksSynced := 0

	chainID := bcR.initialStatus.ChainID
	status := bcR.initialStatus

	lastHundred := time.Now()
	lastRate := 0.0

FOR_LOOP:
	for {
		select {
		case request := <-bcR.requestsCh:
			peer := bcR.sw.Peers().GetByID(request.PeerID)
			if peer == nil {
				continue FOR_LOOP // Peer has since been disconnected.
			}
			bcR.Logger.Info("sendRequest", "peer", peer, "height", request.Height)
			msgBytes := encodeMsg(&bcBlockRequestMessage{request.Height})
			queued := peer.TrySend(BlockchainChannel, msgBytes)
			if !queued {
				// We couldn't make the request, send-queue full.
				// The pool handles timeouts, just let it go.
				continue FOR_LOOP
			}
		case err := <-bcR.errorsCh:
			peer := bcR.sw.Peers().GetByID(err.peerID)
			if peer != nil {
				bcR.sw.StopPeerForError(peer, fmt.Errorf("poolRoutine: %v", err))
			}
		case <-switchToConsensusTicker.C:
			height, numPending, lenRequesters := bcR.pool.GetStatus()
			outbound, inbound, _ := bcR.sw.NumPeers()
			bcR.Logger.Debug("Consensus ticker", "numPending", numPending, "total", lenRequesters,
				"outbound", outbound, "inbound", inbound)
			if bcR.pool.IsCaughtUp() && atomic.CompareAndSwapUint32(&bcR.inFastSync, 1, 0) {
				bcR.Logger.Info("Time to switch to consensus reactor!", "height", height)
				bcR.pool.Stop()

				conR := bcR.sw.Reactor("CONSENSUS").(consensusReactor)
				conR.SwitchToConsensus(status, blocksSynced)

				if !bcR.pool.MockCaughtUp(false) {
					atomic.StoreUint32(&bcR.canFastSync, 1)
				} else {
					bcR.Logger.Info("MockCaughtUp for StopTheWorld")
				}

				break FOR_LOOP
			}
		case <-trySyncTicker.C: // chan time
			// This loop can be slow as long as it's doing syncing work.
		SYNC_LOOP:
			for i := 0; i < 10; i++ {
				// See if there are any blocks to sync.
				first, second := bcR.pool.PeekTwoBlocks()
				//bcR.Logger.Info("TrySync peeked", "first", first, "second", second)
				if first == nil || second == nil {
					// We need both to sync the first block.
					break SYNC_LOOP
				}
				firstParts := first.MakePartSet(status.ConsensusParams.BlockPartSizeBytes)
				firstPartsHeader := firstParts.Header()
				firstID := types.BlockID{first.Hash(), firstPartsHeader}

				if first.Recover > 0 {
					status.Validators = types.NewValidatorSet(bcR.appmgr.GetRecoverValidators(first.Height - 1))
				}

				err := status.Validators.VerifyCommit(chainID, firstID, first.Height, second.LastCommit)
				if err != nil {
					bcR.Logger.Error("Error in validation", "err", err)
					bcR.Logger.Report("fast sync block", "logID", types.LogIdSyncBlockCheckError, "height", first.Height, "err", err)
					//close first peer
					peerID := bcR.pool.RedoRequest(first.Height)
					peer := bcR.sw.Peers().GetByID(peerID)
					if peer != nil {
						bcR.sw.StopPeerForError(peer, fmt.Errorf("BlockchainReactor validation error: %v", err))
					}
					//close second peer
					peerID = bcR.pool.RedoRequest(second.Height)
					peer = bcR.sw.Peers().GetByID(peerID)
					if peer != nil {
						bcR.sw.StopPeerForError(peer, fmt.Errorf("BlockchainReactor validation error: %v", err))
					}
					break SYNC_LOOP
				}

				if !bcR.appmgr.CheckBlock(first) {
					bcR.Logger.Warn("BlockchainReactor CheckBlock failed", "height", first.Height, "blockHash", first.Hash())
					bcR.Logger.Report("fast sync block", "logID", types.LogIdSyncBlockCheckError, "height", first.Height, "err", "check block failed")
					peerID := bcR.pool.RedoRequest(first.Height)
					peer := bcR.sw.Peers().GetByID(peerID)
					if peer != nil {
						bcR.sw.MarkBadNode(peer.NodeInfo())
						bcR.sw.StopPeerForError(peer, fmt.Errorf("BlockchainReactor CheckBlock failed"))
					}
					break SYNC_LOOP
				}

				// XXX NoUsed Now.
				// verify the second block using the first's commit
				// txsResult := &types.TxsResult{
				// 	GasUsed:     second.Header.GasUsed,
				// 	StateHash:   second.Header.StateHash,
				// 	ReceiptHash: second.Header.ReceiptHash,
				// 	LogsBloom:   second.Bloom(),
				// }
				// if !bcR.appmgr.CheckProcessResult(first.Hash(), txsResult) {
				// 	bcR.Logger.Warn("BlockchainReactor CheckProcessResult of second`s failed", "height", second.Height, "blockHash", second.Hash())
				// 	bcR.Logger.Report("fast sync block", "logID", types.LogIdSyncBlockCheckError, "height", first.Height, "err", "check block result failed")
				// 	peerID := bcR.pool.RedoRequest(second.Height)
				// 	peer := bcR.sw.Peers().GetByID(peerID)
				// 	if peer != nil {
				// 		bcR.sw.MarkBadNode(peer.NodeInfo())
				// 		bcR.sw.StopPeerForError(peer, fmt.Errorf("BlockchainReactor CheckProcessResult failed"))
				// 	}
				// 	break SYNC_LOOP
				// }

				// verify the first block success
				bcR.pool.PopRequest()
				// TODO: batch saves so we dont persist to disk every block
				validators, err := bcR.appmgr.CommitBlock(first, firstParts, second.LastCommit, bcR.fastSync)
				if err != nil {
					bcR.Logger.Warn("poolRoutine: CommitBlock failed", "height", first.Height, "blockHash", first.Hash())
					bcR.Logger.Report("fast sync block commit failed", "logID", types.LogIdCommitBlockFail, "height", first.Height, "err", err)
					cmn.PanicQ(cmn.Fmt("+2/3 committed an invalid block, err:%v", err))
				}

				// TODO: same thing for app - but we would need a way to
				// get the hash without persisting the status
				oldHeight := status.LastHeightValidatorsChanged
				status, err = bcR.blockExec.ApplyBlock(status, firstID, first, validators)
				if err != nil {
					// TODO This is bad, are we zombie?
					cmn.PanicQ(cmn.Fmt("Failed to process committed block (%d:%X): %v",
						first.Height, first.Hash(), err))
				}
				blocksSynced++

				if status.LastHeightValidatorsChanged > oldHeight {
					bcR.appmgr.SetLastChangedVals(status.LastHeightValidatorsChanged, status.Validators.Copy().Validators)
				}

				if first.Recover > 0 {
					bcR.Logger.Warn("BlockchainReactor fast sync recover block")
					status.LastRecover = true
				}

				if blocksSynced%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					bcR.Logger.Info("Fast Sync Rate", "height", bcR.pool.height,
						"max_peer_height", bcR.pool.MaxPeerHeight(), "blocks/s", lastRate)
					lastHundred = time.Now()
				}
			}
			continue FOR_LOOP
		case <-bcR.Quit():
			break FOR_LOOP
		}
	}
}

func (bcR *BlockchainReactor) statusUpdateRoutine() {
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)

	for {
		select {
		case <-statusUpdateTicker.C:
			// ask for status updates
			go bcR.BroadcastStatusRequest() // nolint: errcheck
		case <-bcR.Quit():
			return
		}
	}
}

// BroadcastStatusRequest broadcasts current height.
func (bcR *BlockchainReactor) BroadcastStatusRequest() error {
	msgBytes := encodeMsg(&bcStatusRequestMessage{bcR.appmgr.Height()})
	bcR.sw.Broadcast(BlockchainChannel, msgBytes)
	return nil
}

//-----------------------------------------------------------------------------
// Messages

// BlockchainMessage is a generic message for this reactor.
type BlockchainMessage interface{}

func RegisterBlockchainMessages() {
	ser.RegisterInterface((*BlockchainMessage)(nil), nil)
	ser.RegisterConcrete(&bcBlockRequestMessage{}, "blockchain/BlockRequest", nil)
	ser.RegisterConcrete(&bcBlockResponseMessage{}, "blockchain/BlockResponse", nil)
	ser.RegisterConcrete(&bcNoBlockResponseMessage{}, "blockchain/NoBlockResponse", nil)
	ser.RegisterConcrete(&bcStatusResponseMessage{}, "blockchainl/StatusResponse", nil)
	ser.RegisterConcrete(&bcStatusRequestMessage{}, "blockchain/StatusRequest", nil)
}

// decodeMsg decodes BlockchainMessage.
// TODO: ensure that bz is completely read.
func decodeMsg(bz []byte) (msg BlockchainMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)",
			len(bz), maxMsgSize)
	}
	err = ser.DecodeBytesWithType(bz, &msg)
	return
}

func encodeMsg(msg BlockchainMessage) []byte {
	return ser.MustEncodeToBytesWithType(msg)
}

//-------------------------------------

type bcBlockRequestMessage struct {
	Height uint64
}

func (m *bcBlockRequestMessage) String() string {
	return cmn.Fmt("[bcBlockRequestMessage %v]", m.Height)
}

type bcNoBlockResponseMessage struct {
	Height uint64
}

func (brm *bcNoBlockResponseMessage) String() string {
	return cmn.Fmt("[bcNoBlockResponseMessage %d]", brm.Height)
}

//-------------------------------------

type bcBlockResponseMessage struct {
	Block *types.Block
}

func (m *bcBlockResponseMessage) String() string {
	return cmn.Fmt("[bcBlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type bcStatusRequestMessage struct {
	Height uint64
}

func (m *bcStatusRequestMessage) String() string {
	return cmn.Fmt("[bcStatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcStatusResponseMessage struct {
	Height uint64
}

func (m *bcStatusResponseMessage) String() string {
	return cmn.Fmt("[bcStatusResponseMessage %v]", m.Height)
}
