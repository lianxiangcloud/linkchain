package mempool

import (
	"fmt"
	"reflect"
	"runtime"
	"time"

	"github.com/lianxiangcloud/linkchain/libs/clist"
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
}

// NewMempoolReactor returns a new MempoolReactor with the given config and mempool.
func NewMempoolReactor(config *cfg.MempoolConfig, mempool *Mempool) *MempoolReactor {
	memR := &MempoolReactor{
		config:           config,
		Mempool:          mempool,
		cacheRev:         clist.New(),
		mutisignCacheRev: clist.New(),
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
	return nil
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (memR *MempoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:       MempoolChannel,
			Priority: 5,
		},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (memR *MempoolReactor) AddPeer(peer p2p.Peer) {
	go memR.broadcastTxRoutine(peer)
}

// RemovePeer implements Reactor.
func (memR *MempoolReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	// broadcast routine checks if peer is gone and returns
}

func defaultHandReceiveMsg(memR *MempoolReactor, msg MempoolMessage, src p2p.Peer) {
	switch msg := msg.(type) {
	case TxMessage:
		memR.Logger.Debug("Receive", "src", src.ID(), "hash", msg.Tx.Hash())
		tx := msg.Tx
		msg.Tx = nil
		if tx.TypeName() == types.TxMultiSignAccount {
			memR.Logger.Debug("Receive TxMultiSignTx")
			memR.mutisignCacheRev.PushBack(&RecieveMessage{src.ID(), tx})
		} else {
			memR.cacheRev.PushBack(&RecieveMessage{src.ID(), tx})
		}
	default:
		memR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Receive implements Reactor.
// It adds any received transactions to the cache.
func (memR *MempoolReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	if !memR.config.ReceiveP2pTx {
		//memR.Logger.Debug("MempoolReactor Receive return", "ReceiveP2pTx", memR.config.ReceiveP2pTx)
		return
	}
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

			err := memR.Mempool.add(v)
			if err == types.ErrMempoolIsFull {
				revTxList.PushBack(v)
				time.Sleep(1 * time.Second)
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
func (memR *MempoolReactor) broadcastTxRoutine(peer p2p.Peer) {
	if !memR.config.Broadcast {
		return
	}

	nextTx := memR.Mempool.txsFront()
	nextSpecTx := memR.Mempool.specTxsFront()
	const (
		sendSpecTxs = iota
		sendNormalTxs
		sendFinish
	)
	next, turn := nextSpecTx, sendSpecTxs

	for {
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.

		//Send specTxs txs
		if next == nil {
			turn++
			if turn == sendNormalTxs {
				next = nextTx
			}
		}
		if turn == sendFinish {
			memR.Logger.Info("memR broadcast all txs  finished")
			return
		}

		if next == nil {
			continue
		}

		select {
		case <-peer.Quit():
			return
		case <-memR.Quit():
			return
		default:
		}

		var sendData []byte
		switch v := next.Value.(type) {
		case *mempoolTx:
			msg := &TxMessage{Tx: v.tx}
			data, err := ser.EncodeToBytesWithType(msg)
			if err != nil {
				memR.Logger.Error("failed to marshal tx", "hash", v.tx.Hash().String())
			}
			sendData = data
		}
		success := peer.Send(MempoolChannel, sendData)
		if !success {
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}
		next = next.Next()
	}
}

//-----------------------------------------------------------------------------
// Messages

// MempoolMessage is a message sent or received by the MempoolReactor.
type MempoolMessage interface{}

func RegisterMempoolMessages() {
	ser.RegisterInterface((*MempoolMessage)(nil), nil)
	ser.RegisterConcrete(TxMessage{}, "mempool/TxMessage", nil)
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

type RecieveMessage struct {
	PeerID string
	Tx     types.Tx
}
