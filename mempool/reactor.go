package mempool

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/clist"
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/ser"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/types"
)

const (
	//MempoolChannel ID
	MempoolChannel = byte(0x30)

	maxMsgSize                 = 1048576 // 1MB TODO make it configurable
	peerCatchupSleepIntervalMS = 100     // If peer is behind, sleep this amount
	ReceiveCacheMaxLength      = 10000   // Max Length of Receive
)

var (
	HandleReceiveMsgFunc = defaultHandReceiveMsg
)

// MempoolReactor handles mempool tx broadcasting amongst peers.
type MempoolReactor struct {
	p2p.BaseReactor
	config           *cfg.MempoolConfig
	Mempool          *Mempool
	cacheRev         *clist.CList //tx
	mutisignCacheRev *clist.CList //mutisign tx

	cacheHash txCache
	txReqCh   chan txRequest
}

// NewMempoolReactor returns a new MempoolReactor with the given config and mempool.
func NewMempoolReactor(config *cfg.MempoolConfig, mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		config:           config,
		Mempool:          mempool,
		cacheRev:         clist.New(),
		mutisignCacheRev: clist.New(),
		cacheHash:        newTxHeap(15),
		txReqCh:          make(chan txRequest, 20000),
	}
	memR.BaseReactor = *p2p.NewBaseReactor("MempoolReactor", memR)
	return memR
}

func (memR *MempoolReactor) GetRecvCache() *clist.CList {
	return memR.cacheRev
}

func (memR *MempoolReactor) GetMutilSignCache() *clist.CList {
	return memR.mutisignCacheRev
}

// SetLogger sets the Logger on the reactor and the underlying Mempool.
func (memR *MempoolReactor) SetLogger(l log.Logger) {
	memR.Logger = l
	//memR.Mempool.SetLogger(l)
}

// OnStart implements p2p.BaseReactor.
func (memR *MempoolReactor) OnStart() error {
	if !memR.config.Broadcast {
		memR.Logger.Info("Tx broadcasting is disabled")
	}

	go memR.receiveTxRoutine()
	go memR.handleMutilsignTx()
	go memR.handleTxReqRoutine()
	return nil
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (memR *MempoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                MempoolChannel,
			Priority:          5,
			SendQueueCapacity: 2000,
		},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (memR *MempoolReactor) AddPeer(peer p2p.Peer) {
	// go memR.broadcastTxRoutine(peer)
	go memR.broadcastTxToPeer(peer)
}

// RemovePeer implements Reactor.
func (memR *MempoolReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	// broadcast routine checks if peer is gone and returns
}

func defaultHandReceiveMsg(memR *MempoolReactor, msg MempoolMessage, src p2p.Peer) {
	switch msg := msg.(type) {
	case TxMessage:
		if !memR.config.ReceiveP2pTx {
			memR.Logger.Debug("MempoolReactor Receive return", "ReceiveP2pTx", memR.config.ReceiveP2pTx)
			return
		}
		memR.Logger.Debug("Receive", "src", src.ID(), "hash", msg.Tx.Hash())
		tx := msg.Tx
		msg.Tx = nil
		if tx.TypeName() == types.TxMultiSignAccount {
			memR.Logger.Debug("Receive TxMultiSignTx")
			if memR.mutisignCacheRev.Len() >= ReceiveCacheMaxLength { // mutisignCacheRev Reach ReceiveCacheMaxLength Limit
				memR.Logger.Info("mutisignCacheRev Reach Limit, Drop Tx", "src", src.ID(), "hash", tx.Hash())
				return
			}
			memR.mutisignCacheRev.PushBack(&RecieveMessage{src.ID(), tx})
		} else {
			if memR.cacheRev.Len() >= ReceiveCacheMaxLength { // cacheRev Reach ReceiveCacheMaxLength Limit
				memR.Logger.Info("cacheRev Reach Limit, Drop Tx", "src", src.ID(), "hash", tx.Hash())
				return
			}
			memR.cacheRev.PushBack(&RecieveMessage{src.ID(), tx})
		}
	case TxHashMessage:
		memR.Logger.Debug("Receive", "src", src.ID(), "msg", msg.String())
		if msg.Kind == TxHashNotify {
			for _, hash := range msg.Hashs {
				if memR.Mempool.cache.Exists(hash) { // mempool already has it
					continue
				}
				if memR.cacheHash.Put(makeNopTx(hash)) { // put to cache ok
					memR.txReqCh <- txRequest{hash: hash, src: src}
				} // already cache
			}
		}
		if msg.Kind == TxHashRequest {
			for _, hash := range msg.Hashs {
				if cacheTx := memR.Mempool.cache.Get(hash); cacheTx != nil {
					memR.txReqCh <- txRequest{tx: cacheTx, dst: src}
				}
			}
		}

	default:
		memR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Receive implements Reactor.
// It adds any received transactions to the cache.
func (memR *MempoolReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		memR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		memR.Mempool.sw.StopPeerForError(src, err)
		return
	}
	HandleReceiveMsgFunc(memR, msg, src)
}

func (memR *MempoolReactor) handleReceiveTx(isMultisignTx bool) {
	var revTxList *clist.CList
	if isMultisignTx {
		revTxList = memR.mutisignCacheRev
	} else {
		revTxList = memR.cacheRev
	}
	var (
		next       *clist.CElement
		removed    *clist.CElement
		addTxChLen = (runtime.NumCPU() + 3) >> 2
		addTxCh    = make(chan struct{}, addTxChLen)
	)

	for i := 0; i < addTxChLen; i++ {
		addTxCh <- struct{}{}
	}

	for {
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-revTxList.WaitChan(): // Wait until a tx is available
				if next = revTxList.Front(); next == nil {
					continue
				}
			case <-memR.Quit():
				return
			}
		}

		v := next.Value

		removed = next
		next = next.Next()
		revTxList.Remove(removed)
		removed.Value = nil
		removed.DetachPrev()

		<-addTxCh
		addTx := func(v interface{}) {
			switch tmpTx := v.(type) {
			case *RecieveMessage:
				memR.Logger.Debug("start add", "hash", tmpTx.Tx.Hash())
			default:
				memR.Logger.Info("error msg type")
			}
			// transaction not successfully added to mempool will be dropped instead of re-entering
			if err := memR.Mempool.add(v); err != nil {
				if err != types.ErrTxDuplicate && err != types.ErrMempoolIsFull {
					memR.Logger.Error("mempool add data from peers failed", "err", err, "hash", v.(*RecieveMessage).Tx.Hash(), "cacheLen", revTxList.Len())
				}
			}
			addTxCh <- struct{}{}
		}
		if isMultisignTx {
			addTx(v)
		} else {
			go addTx(v)
		}
	}
}

func (memR *MempoolReactor) handleMutilsignTx() {
	memR.handleReceiveTx(true)
}

// receive tx from cache.
func (memR *MempoolReactor) receiveTxRoutine() {
	memR.handleReceiveTx(false)
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() uint64
}

// Send new mempool txs to peer.
func (memR *MempoolReactor) broadcastTxToPeer(peer p2p.Peer) {
	if !memR.config.Broadcast {
		return
	}

	total := 0
	_start := time.Now()
	send := func(next *clist.CElement) error {
		// @Note: should be < 1000
		maxCount := 256 + int(rand.Int31n(256))

		for next != nil {
			count := 0
			msg := TxHashMessage{Kind: TxHashNotify}
			hashs := make([]common.Hash, 0, maxCount)

			for (next != nil) && (count < maxCount) {
				switch v := next.Value.(type) {
				case *mempoolTx:
					hashs = append(hashs, v.tx.Hash())
					next = next.Next()
					count++
				}
			}

			select {
			case <-peer.Quit():
				return fmt.Errorf("peer Quit")
			case <-memR.Quit():
				return fmt.Errorf("MemReactor Quit")
			default:
			}

			msg.Hashs = hashs
			data, err := ser.EncodeToBytesWithType(&msg)
			if err != nil {
				memR.Logger.Error("broadcastTxToPeer: marshal fail", "err", err)
				return err
			}

			if !peer.Send(MempoolChannel, data) {
				memR.Logger.Warn("broadcastTxToPeer: send timeout")
			}
			total += len(hashs)
		}
		return nil
	}

	if err := send(memR.Mempool.utxoTxsFront()); err != nil {
		return
	}
	if err := send(memR.Mempool.txsFront()); err != nil {
		return
	}
	if err := send(memR.Mempool.specTxsFront()); err != nil {
		return
	}
	memR.Logger.Info("broadcastTxToPeer: done", "total", total, "used", time.Since(_start).String())
}

func (memR *MempoolReactor) handleTxReqRoutine() {
	for {
		select {
		case <-memR.Quit():
			memR.Logger.Info("handleTxReqRoutine exit...")
			return
		case req := <-memR.txReqCh:
			var (
				peer p2p.Peer
				data []byte
				err  error
				wait bool
			)

			if req.src != nil { // send tx request to peer
				memR.cacheHash.DelayDelete(req.hash)
				peer = req.src
				msg := TxHashMessage{Hashs: []common.Hash{req.hash}, Kind: TxHashRequest}
				data, err = ser.EncodeToBytesWithType(&msg)
				if err != nil {
					memR.Logger.Error("handleTxReqRoutine: marshal tx fail", "hash", req.hash)
					continue
				}
			} else { // send tx data to peer
				peer = req.dst
				msg := TxMessage{Tx: req.tx}
				data, err = ser.EncodeToBytesWithType(&msg)
				if err != nil {
					memR.Logger.Error("handleTxReqRoutine: marshal tx fail", "hash", req.tx.Hash())
					continue
				}
				if req.tx.TypeName() == types.TxMultiSignAccount {
					wait = true
				}
			}

			if !wait { // no wait
				if !peer.TrySend(MempoolChannel, data) {
					go peer.Send(MempoolChannel, data)
				}
				continue
			}
			peer.Send(MempoolChannel, data)
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

// MempoolMessage is a message sent or received by the MempoolReactor.
type MempoolMessage interface{}

func RegisterMempoolMessages() {
	ser.RegisterInterface((*MempoolMessage)(nil), nil)
	ser.RegisterConcrete(TxMessage{}, "mempool/TxMessage", nil)
	ser.RegisterConcrete(TxHashMessage{}, "mempool/TxHashMessage", nil)
}

// decodeMsg decodes a byte-array into a MempoolMessage.
func decodeMsg(bz []byte) (msg MempoolMessage, err error) {
	/*
		if len(bz) > maxMsgSize {
			return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
		}
	*/
	err = ser.DecodeBytesWithType(bz, &msg)
	return
}

//-------------------------------------

// TxMessage is a MempoolMessage containing a transaction.
type TxMessage struct {
	Tx types.Tx
}

// String returns a string representation of the TxMessage.
func (m TxMessage) String() string {
	return fmt.Sprintf("[TxMessage %v]", m.Tx)
}

type TxHashMessageKind int

const (
	_ TxHashMessageKind = iota
	TxHashNotify
	TxHashRequest
)

// TxHashMessage --
type TxHashMessage struct {
	Hashs []common.Hash
	Kind  TxHashMessageKind
}

func (m TxHashMessage) String() string {
	buf := bytes.NewBufferString("[TxHashMessage K:L:V ")
	buf.WriteString(fmt.Sprintf("%d:%d:[", m.Kind, len(m.Hashs)))

	for i := 0; i < len(m.Hashs); i++ {
		buf.WriteString(hex.EncodeToString(m.Hashs[i][:3]))
		buf.WriteString("..., ")
	}
	buf.WriteString("] ]")
	return buf.String()

	// return fmt.Sprintf("[TxHashMessage %d:%v]", m.Kind, m.Hashs)
}

type RecieveMessage struct {
	PeerID string
	Tx     types.Tx
}

// ---------------------------
type nopTx common.Hash

func makeNopTx(hash common.Hash) *nopTx {
	tx := new(common.Hash)
	copy(tx[:], hash[:])
	return (*nopTx)(tx)
}

func (tx *nopTx) Hash() common.Hash               { return common.Hash(*tx) }
func (tx *nopTx) From() (common.Address, error)   { panic("nopTx Not Support") }
func (tx *nopTx) To() *common.Address             { panic("nopTx Not Support") }
func (tx *nopTx) TypeName() string                { return "nopTx" }
func (tx *nopTx) CheckBasic(types.TxCensor) error { panic("nopTx Not Support") }
func (tx *nopTx) CheckState(types.TxCensor) error { panic("nopTx Not Support") }

// ------------------------
type txRequest struct {
	src  p2p.Peer
	hash common.Hash

	tx  types.Tx
	dst p2p.Peer
}
