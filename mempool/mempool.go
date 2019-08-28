package mempool

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"

	cfg "github.com/lianxiangcloud/linkchain/config"
	"github.com/lianxiangcloud/linkchain/libs/clist"
	"github.com/lianxiangcloud/linkchain/libs/common"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	lktypes "github.com/lianxiangcloud/linkchain/libs/cryptonote/types"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

/*

The mempool pushes new txs onto the proxyAppConn.
It gets a stream of (req, res) tuples from the proxy.
The mempool stores good txs in a concurrent linked-list.

Multiple concurrent go-routines can traverse this linked-list
safely by calling .NextWait() on each element.

So we have several go-routines:
1. Consensus calling Update() and Reap() synchronously
2. Many mempool reactor's peer routines calling CheckTx()
3. Many mempool reactor's peer routines traversing the txs linked list
4. Another goroutine calling GarbageCollectTxs() periodically

To manage these goroutines, there are three methods of locking.
1. Mutations to the linked-list is protected by an internal mtx (CList is goroutine-safe)
2. Mutations to the linked-list elements are atomic
3. CheckTx() calls can be paused upon Update() and Reap(), protected by .proxyMtx

Garbage collection of old elements from mempool.txs is handlde via
the DetachPrev() call, which makes old elements not reachable by
peer broadcastTxRoutine() automatically garbage collected.

TODO: Better handle abci client errors. (make it automatically handle connection errors)

*/

var (
	evictionInterval    = 10 * time.Second // Time interval to check for evictable transactions
	statsReportInterval = 5 * time.Second  // Time interval to report transaction pool stats
)

var canPromoteTxType = map[string]struct{}{
	types.TxNormal:          struct{}{},
	types.TxToken:           struct{}{},
	types.TxContractCreate:  struct{}{},
	types.TxContractUpgrade: struct{}{},
}

var canAddTxType = map[string]struct{}{
	types.TxNormal:           struct{}{},
	types.TxToken:            struct{}{},
	types.TxMultiSignAccount: struct{}{},
	types.TxContractCreate:   struct{}{},
	types.TxContractUpgrade:  struct{}{},
	types.TxUTXO:             struct{}{},
}

var canAddFutureTxType = map[string]struct{}{
	types.TxNormal:          struct{}{},
	types.TxToken:           struct{}{},
	types.TxContractCreate:  struct{}{},
	types.TxContractUpgrade: struct{}{},
}

var (
	BroadcastTxFunc = defaultBroadcastTx
)

// Mempool is an ordered in-memory pool for transactions before they are proposed in a consensus
// round. Transaction validity is checked using the CheckTx abci message before the transaction is
// added to the pool. The Mempool uses a concurrent list structure for storing transactions that
// can be efficiently accessed by multiple concurrent readers.
type Mempool struct {
	config *cfg.MempoolConfig

	app App

	proxyMtx             sync.Mutex
	goodTxs              *clist.CList // concurrent linked-list of good txs
	specGoodTxs          *clist.CList //for updatavalidators Tx and MultiSignAccount Tx
	futureTxs            map[common.Address]*txList
	futureTxsCount       int
	beats                map[common.Address]time.Time // Last heartbeat from each known account
	height               uint64                       // the last block Update()'d to
	rechecking           int32                        // for re-checking filtered txs on Update()
	notifiedTxsAvailable bool
	txsAvailable         chan struct{} // fires once for each height, when the mempool is not empty
	sw                   p2p.P2PManager
	//broadcastTxChan      chan types.Tx
	broadcastTxChan chan *RecieveMessage

	// Keep a cache of already-seen txs.
	// This reduces the pressure on the proxyApp.
	cache txCache

	logger log.Logger

	metrics *Metrics

	quit chan bool

	sem *semaphore.Weighted
	//keyimage cache
	kImageMtx   sync.RWMutex
	kImageCache map[lktypes.Key]bool
}

// MemFunc sets an optional parameter on the Mempool.
type MemFunc func(*Mempool)

// NewMempool returns a new Mempool with the given configuration and height.
func NewMempool(config *cfg.MempoolConfig, height uint64, sw p2p.P2PManager, options ...MemFunc) *Mempool {
	sigWorkers := int64((runtime.NumCPU() + 3) >> 2)
	sem := semaphore.NewWeighted(sigWorkers)
	mempool := &Mempool{
		config:          config,
		goodTxs:         clist.New(),
		specGoodTxs:     clist.New(),
		futureTxs:       make(map[common.Address]*txList),
		beats:           make(map[common.Address]time.Time),
		height:          height,
		rechecking:      0,
		sw:              sw,
		broadcastTxChan: make(chan *RecieveMessage, config.BroadcastChanSize),
		logger:          log.NewNopLogger(),
		metrics:         NopMetrics(),
		quit:            make(chan bool),
		sem:             sem,
	}

	if config.CacheSize > 0 {
		mempool.cache = newMapTxCache(config.CacheSize)
	} else {
		mempool.cache = nopTxCache{}
	}

	mempool.kImageCache = make(map[lktypes.Key]bool)

	for _, option := range options {
		option(mempool)
	}

	go mempool.loop()
	go mempool.broadcastTxRoutine()

	return mempool
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (mem *Mempool) loop() {
	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	// Keep waiting for and reacting to the various events
	for {
		select {
		case <-mem.quit:
			mem.logger.Info("mempool quit")
			return

		// Handle stats reporting ticks
		case <-report.C:
			mem.proxyMtx.Lock()
			specSpending, pending, queued := mem.stats()
			mem.futureTxsCount = queued
			mem.proxyMtx.Unlock()
			mem.logger.Debug("status report", "specGoodTxs", specSpending, "goodTxs", pending, "futureTxs", queued)

		// Handle inactive account transaction eviction
		case <-evict.C:
			if !mem.config.RemoveFutureTx {
				continue
			}
			mem.proxyMtx.Lock()
			for addr := range mem.futureTxs {
				addr := addr
				// Any non-locals old enough should be removed
				if time.Since(mem.beats[addr]) >= mem.config.Lifetime {
					for _, tx := range mem.futureTxs[addr].Flatten() {
						mem.removeFutureTx(addr, tx)
					}
				}
			}
			mem.proxyMtx.Unlock()
		}
	}
}

//Stop ...
func (mem *Mempool) Stop() {
	select {
	case mem.quit <- true:
	default:
	}
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (mem *Mempool) Stats() (int, int, int) {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	return mem.stats()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (mem *Mempool) stats() (specPending int, pending int, queued int) {
	specPending = mem.specGoodTxs.Len()
	pending = mem.goodTxs.Len()
	for _, list := range mem.futureTxs {
		queued += list.Len()
	}
	return
}

// EnableTxsAvailable initializes the TxsAvailable channel,
// ensuring it will trigger once every height when transactions are available.
// NOTE: not thread safe - should only be called once, on startup
func (mem *Mempool) EnableTxsAvailable() {
	mem.txsAvailable = make(chan struct{}, 1)
}

//SetApp set application handle
func (mem *Mempool) SetApp(a App) {
	mem.app = a
}

// SetLogger sets the Logger.
func (mem *Mempool) SetLogger(l log.Logger) {
	mem.logger = l
}

// WithMetrics sets the metrics.
func WithMetrics(metrics *Metrics) MemFunc {
	return func(mem *Mempool) { mem.metrics = metrics }
}

// Lock locks the mempool. The consensus must be able to hold lock to safely update.
func (mem *Mempool) Lock() {
	mem.proxyMtx.Lock()
}

// Unlock unlocks the mempool.
func (mem *Mempool) Unlock() {
	mem.proxyMtx.Unlock()
}

//SetReceiveP2pTx ...
func (mem *Mempool) SetReceiveP2pTx(on bool) {
	mem.config.ReceiveP2pTx = on
}

//VerifyTxFromCache is used to check whether tx has been verify in mempool by APP
func (mem *Mempool) VerifyTxFromCache(hash common.Hash) (*common.Address, bool) {
	return mem.cache.VerifyTxFromCache(hash)
}

// GoodTxsSize returns goodTxs list length.
func (mem *Mempool) GoodTxsSize() int {
	return mem.goodTxs.Len()
}

// SpecGoodTxsSize returns specGoodList length.
func (mem *Mempool) SpecGoodTxsSize() int {
	return mem.specGoodTxs.Len()
}

// txsFront returns the first transaction in the ordered list for peer
// goroutines to call .NextWait() on.
func (mem *Mempool) txsFront() *clist.CElement {
	return mem.goodTxs.Front()
}

func (mem *Mempool) specTxsFront() *clist.CElement {
	return mem.specGoodTxs.Front()
}

func (mem *Mempool) addUTXOTx(tx types.Tx) (err error) {
	if mem.goodTxs.Len() >= mem.config.Size {
		return types.ErrMempoolIsFull
	}

	if err := mem.app.CheckTx(tx, StateCheck); err != nil {
		return err
	}

	mem.addGoodTx(tx, true)
	return nil
}

func (mem *Mempool) addLocalTx(tx types.Tx) (err error) {
	log.Debug("addLocalTx", "hash", tx.Hash())
	if err = mem.app.CheckTx(tx, StateCheck); err == nil {
		if mem.goodTxs.Len() < mem.config.Size {
			mem.addGoodTx(tx, true)
		} else {
			err = mem.addFutureTx(tx)
		}
	} else if err == types.ErrNonceTooHigh {
		if mem.futureTxsCount >= mem.config.FutureSize {
			mem.cache.Remove(tx)
			return types.ErrMempoolIsFull
		}
		err = mem.addFutureTx(tx)
	} else {
		mem.logger.Warn("addLocalTx", "CheckTx failed", err, "txHash", tx.Hash().Hex())
	}
	return err
}

func (mem *Mempool) addLocalSpecTx(tx types.Tx) (err error) {
	if err = mem.app.CheckTx(tx, StateCheck); err == nil {
		if mem.specGoodTxs.Len() < mem.config.SpecSize {
			mem.addSpecGoodTx(tx)
		} else {
			err = types.ErrMempoolIsFull
		}
	} else {
		if err != types.ErrNonceTooHigh {
			mem.logger.Warn("addLocalSpecTx", "CheckSpecTx failed", err, "tx", tx.TypeName(), "txHash", tx.Hash())
		}
	}
	return err
}

//AddTx add good txs in a concurrent linked-list
func (mem *Mempool) AddTx(peerID string, tx types.Tx) (err error) {
	// CACHE
	if !mem.cache.Push(tx) {
		return types.ErrTxDuplicate
	}
	// END CACHE

	if _, exist := canAddTxType[tx.TypeName()]; !exist {
		mem.cache.Remove(tx)
		return types.ErrParams
	}

	if err = mem.app.CheckTx(tx, BasicCheck); err != nil {
		mem.cache.Remove(tx)
		return
	}

	var from common.Address
	if from, err = tx.From(); err != nil {
		mem.cache.Remove(tx)
		return
	}
	mem.cache.SetTxFrom(tx.Hash(), &from)

	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	switch tx.(type) {
	case *types.UTXOTransaction:
		// blacklist check
		txUtxo := tx.(*types.UTXOTransaction)
		if (txUtxo.UTXOKind() & types.Aout) == types.Aout {
			for _, out := range txUtxo.Outputs {
				switch aOutput := out.(type) {
				case *types.AccountOutput:
					if types.BlacklistInstance().IsBlackAddress(common.EmptyAddress, aOutput.To) {
						mem.cache.Remove(tx)
						return types.ErrBlacklistAddress
					}
				}
			}
		}
		if (txUtxo.UTXOKind() & types.Ain) == types.Ain {
			fromAddr, err := txUtxo.From()
			if err != nil {
				fromAddr = common.EmptyAddress
			}
			if types.BlacklistInstance().IsBlackAddress(fromAddr, common.EmptyAddress) {
				mem.cache.Remove(tx)
				return types.ErrBlacklistAddress
			}
		}
		err = mem.addUTXOTx(tx)
	case *types.Transaction, *types.TokenTransaction, *types.ContractCreateTx, *types.ContractUpgradeTx:
		// blacklist check
		var fromAddr, toAddr common.Address
		fromAddr, err := tx.From()
		if err != nil {
			fromAddr = common.EmptyAddress
		}
		if tx.To() != nil {
			toAddr = *tx.To()
		} else {
			toAddr = common.EmptyAddress
		}
		if types.BlacklistInstance().IsBlackAddress(fromAddr, toAddr) {
			mem.cache.Remove(tx)
			return types.ErrBlacklistAddress
		}
		err = mem.addLocalTx(tx)
	case *types.MultiSignAccountTx:
		// blacklist check
		var fromAddr, toAddr common.Address
		fromAddr, err := tx.From()
		if err != nil {
			fromAddr = common.EmptyAddress
		}
		if tx.To() != nil {
			toAddr = *tx.To()
		} else {
			toAddr = common.EmptyAddress
		}
		if types.BlacklistInstance().IsBlackAddress(fromAddr, toAddr) {
			mem.cache.Remove(tx)
			return types.ErrBlacklistAddress
		}
		err = mem.addLocalSpecTx(tx)
	default:
		mem.logger.Error("AddTx fail,Invaild tx type")
		mem.cache.Remove(tx)
		return types.ErrParams
	}

	if err != nil {
		mem.cache.Remove(tx)
	} else if mem.config.Broadcast {
		select {
		case mem.broadcastTxChan <- &RecieveMessage{PeerID: peerID, Tx: tx}:
		default:
			mem.logger.Info("broadcastTxChan is full", "size", mem.config.BroadcastChanSize, "hash", tx.Hash().Hex())
		}
	}
	return err
}

func (mem *Mempool) add(data interface{}) (err error) {
	switch v := data.(type) {
	case *RecieveMessage:
		err = mem.AddTx(v.PeerID, v.Tx)
		if err != nil && err != types.ErrTxDuplicate && err != types.ErrMempoolIsFull {
			mem.logger.Error("mempool add data from peers failed", "err", err, "v", v.Tx.Hash())
		}
	}
	return err
}

func waitBroadcast(resultChain chan bool) {
	tick := time.NewTicker(time.Duration(2) * time.Second)
	defer tick.Stop()
	for {
		select {
		case _, ok := <-resultChain:
			if !ok { //all peer send done
				log.Debug("all peer send done")
				return
			}
		case <-tick.C:
			log.Info("wait resultChain timeout")
			return
		}
	}
}

func (mem *Mempool) broadcastTxRoutine() {
	if !mem.config.Broadcast {
		return
	}

	for {
		select {
		case txMsg, ok := <-mem.broadcastTxChan:
			if ok && mem.sw != nil {
				BroadcastTxFunc(txMsg.PeerID, txMsg.Tx, mem.sw, mem.logger)
			}
		case <-mem.quit:
			return
		}
	}
}

func defaultBroadcastTx(peerID string, tx types.Tx, sw p2p.P2PManager, logger log.Logger) {
	msg := &TxMessage{Tx: tx}
	logger.Debug("mempool broadcast tx to peers", "hash", tx.Hash().String(), "peers", sw.Peers().List())
	data, err := ser.EncodeToBytesWithType(msg)
	if err != nil {
		logger.Error("mempool failed to marshal tx", "hash", tx.Hash().String())
	}
	if tx.TypeName() == types.TxMultiSignAccount {
		logger.Debug("TxMultiSignAccount")

		resultChain := sw.BroadcastE(MempoolChannel, peerID, data)
		waitBroadcast(resultChain)
	} else {
		sw.BroadcastE(MempoolChannel, peerID, data)
	}
}

func (mem *Mempool) addSpecGoodTx(tx types.Tx) {
	addtime := time.Now()
	memTx := &mempoolTx{tx: tx, addtime: &addtime}
	mem.specGoodTxs.PushBack(memTx)
	mem.logger.Debug("Added Specgood transaction", "tx", tx.Hash().Hex(), "type", tx.TypeName())
	mem.notifyTxsAvailable()
}

// addGoodTx add a transaction to goodTxs
func (mem *Mempool) addGoodTx(tx types.Tx, promote bool) {
	memTx := &mempoolTx{tx: tx}
	mem.goodTxs.PushBack(memTx)
	mem.logger.Debug("Added good transaction", "tx", tx.Hash().Hex(), "type", tx.TypeName())
	mem.metrics.Size.Set(float64(mem.GoodTxsSize()))
	mem.notifyTxsAvailable()

	if _, exist := canPromoteTxType[tx.TypeName()]; !exist {
		return
	}
	from, _ := tx.From()
	mem.promoteExecutables([]common.Address{from})
}

func (mem *Mempool) addTofutureTxs(from common.Address, tx types.RegularTx) error {
	mem.beats[from] = time.Now()
	if mem.futureTxs[from] == nil {
		mem.futureTxs[from] = newTxList(false)
		mem.beats[from] = time.Now()
	}

	inserted, _ := mem.futureTxs[from].Add(tx)
	if !inserted {
		mem.logger.Warn("futureTxs Add tx duplicate cached", "txNonce", tx.Nonce(), "txHash", tx.Hash())
		return types.ErrTxDuplicate
	}
	mem.logger.Debug("Added future transaction", "tx", tx.Hash().Hex(), "type", tx.TypeName(), "from", from.Hex(), "nonce", tx.Nonce())
	if mem.config.RemoveFutureTx {
		mem.futureTxsCount = mem.removeFutureTxs()
	}
	return nil
}

// addFutureTx add a transaction to futureTxs
func (mem *Mempool) addFutureTx(tx types.Tx) error {
	if _, exist := canAddFutureTxType[tx.TypeName()]; !exist {
		mem.logger.Error("tx is not a Transaction", "tx.Type", tx.TypeName(), "tx", tx)
		return types.ErrParams
	}
	rtx, ok := tx.(types.RegularTx)
	if !ok {
		mem.logger.Error("tx is not a Transaction", "tx.Type", tx.TypeName(), "tx", tx)
		return types.ErrParams
	}
	from, _ := tx.From()
	return mem.addTofutureTxs(from, rtx)
}

// promoteExecutables moves transactions that have become processable from the
// futureTxs to the goodTxs. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
func (mem *Mempool) promoteExecutables(accounts []common.Address) {
	// Track the promoted transactions to broadcast them at once
	var promoting types.Transactions

	// Gather all the accounts potentially needing updates
	if accounts == nil {
		accounts = make([]common.Address, 0, len(mem.futureTxs))
		for addr := range mem.futureTxs {
			accounts = append(accounts, addr)
		}
	}

	// Iterate over all accounts and promote any executable transactions
	for _, addr := range accounts {
		list := mem.futureTxs[addr]
		if list == nil {
			continue
		}

		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(mem.app.GetNonce(addr)) {
			//remove from cache
			mem.cache.Remove(tx)
			mem.futureTxsCount--
			mem.logger.Trace("Removed old futureTx", "tx", tx.Hash(), "nonce", tx.Nonce())
		}

		// Gather all executable transactions and promote them
		if need := mem.config.Size - mem.GoodTxsSize(); need > 0 {
			startNonce := mem.app.GetNonce(addr)
			endNonce := startNonce + uint64(need)
			promoting = list.Ready(startNonce, endNonce)
			for _, tx := range promoting {
				if err := mem.app.CheckTx(tx, StateCheck); err != nil {
					mem.cache.Remove(tx)
					mem.logger.Error("promoting transaction", "tx", tx.Hash().Hex(), "err", err)
				} else {
					mem.addGoodTx(tx, false)
					mem.logger.Trace("move futureTx to goodTxs", "tx", tx.Hash().Hex(), "nonce", tx.Nonce())
				}
				mem.futureTxsCount--
			}

			if promoting.Len() > 0 {
				mem.beats[addr] = time.Now()
			}
		}

		if mem.config.RemoveFutureTx {
			// Drop all transactions over the allowed limit
			for _, tx := range list.Cap(mem.config.AccountQueue) {
				mem.cache.Remove(tx)
				mem.futureTxsCount--
				mem.logger.Trace("Removed cap-exceeding futureTx", "tx", tx.Hash(), "nonce", tx.Nonce())
			}
		}

		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(mem.futureTxs, addr)
			delete(mem.beats, addr)
		}
	}
}

func (mem *Mempool) removeFutureTxs() int {
	// If we've queued more transactions than the hard limit, drop oldest ones
	futureSize := mem.config.FutureSize
	queued := 0
	for _, list := range mem.futureTxs {
		queued += list.Len()
	}
	count := queued

	if queued > futureSize {
		// Sort all accounts with queued transactions by heartbeat
		addresses := make(addresssByHeartbeat, 0, len(mem.futureTxs))
		for addr := range mem.futureTxs {
			addresses = append(addresses, addressByHeartbeat{addr, mem.beats[addr]})
		}
		sort.Sort(addresses)

		// Drop transactions until the total is below the limit or only locals remain
		for drop := queued - futureSize; drop > 0 && len(addresses) > 0; {
			addr := addresses[0]
			list := mem.futureTxs[addr.address]

			addresses = addresses[1:]

			// Drop all transactions if they are less than the overflow
			if size := list.Len(); size <= drop {
				for _, tx := range list.Flatten() {
					mem.removeFutureTx(addr.address, tx)
				}
				drop -= size
				count -= size
				continue
			}
			// Otherwise drop only last few transactions
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				mem.removeFutureTx(addr.address, txs[i])
				drop--
				count--
			}
		}
	}
	return count
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
func (mem *Mempool) removeFutureTx(addr common.Address, etx types.RegularTx) {
	// Transaction is in the future queue
	if future := mem.futureTxs[addr]; future != nil {
		future.Remove(etx)
		if future.Empty() {
			delete(mem.futureTxs, addr)
			delete(mem.beats, addr)
		}
		mem.cache.Remove(etx)
		mem.futureTxsCount--
		mem.logger.Debug("removeFutureTx", "tx", etx.Hash().Hex(), "nonce", etx.Nonce())
	}
}

// TxsAvailable returns a channel which fires once for every height,
// and only when transactions are available in the mempool.
// NOTE: the returned channel may be nil if EnableTxsAvailable was not called.
func (mem *Mempool) TxsAvailable() <-chan struct{} {
	return mem.txsAvailable
}

func (mem *Mempool) notifyTxsAvailable() {
	if mem.GoodTxsSize() == 0 && mem.SpecGoodTxsSize() == 0 {
		panic("notified txs available but mempool is empty!")
	}
	if mem.txsAvailable != nil && !mem.notifiedTxsAvailable {
		// channel cap is 1, so this will send once
		mem.notifiedTxsAvailable = true
		select {
		case mem.txsAvailable <- struct{}{}:
		default:
		}
	}
}

// Reap returns a list of transactions currently in the mempool.
// If maxTxs is -1, there is no cap on the number of returned transactions.
func (mem *Mempool) Reap(maxTxs int) types.Txs {
	mem.proxyMtx.Lock()
	defer mem.proxyMtx.Unlock()

	mem.logger.Debug("Reap start", "maxTxs", maxTxs, "goodTxs", mem.GoodTxsSize(), "specTx", mem.SpecGoodTxsSize())
	if maxTxs <= 0 {
		return make([]types.Tx, 0)
	}
	for atomic.LoadInt32(&mem.rechecking) > 0 {
		// TODO: Something better?
		time.Sleep(time.Millisecond * 10)
	}

	if maxTxs > mem.config.MaxReapSize {
		maxTxs = mem.config.MaxReapSize
	}

	specTxs := mem.collectTxs(mem.specGoodTxs, mem.config.SpecSize) //get all special Txs

	maxTxs = maxTxs - len(specTxs)
	txs := mem.collectTxs(mem.goodTxs, maxTxs)
	mem.logger.Debug("Reap end", "specTxs", len(specTxs), "txsLen", len(txs), "maxTxs", maxTxs)
	txs = append(txs, specTxs...)
	return txs
}

// maxTxs: -1 means uncapped, 0 means none
func (mem *Mempool) collectTxs(txList *clist.CList, maxTxs int) types.Txs {
	if maxTxs <= 0 {
		return make([]types.Tx, 0)
	}
	utxoTxCount := 0
	txs := make([]types.Tx, 0, cmn.MinInt(txList.Len(), maxTxs))
	for e := txList.Front(); e != nil && len(txs) < maxTxs; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		if memTx.tx.TypeName() == types.TxUTXO {
			utxoTxCount++
		}
		txs = append(txs, memTx.tx)
		if utxoTxCount >= mem.config.UTXOSize {
			break
		}
	}
	return txs
}

// Update informs the mempool that the given txs were committed and can be discarded.
// NOTE: this should be called *after* block is committed by consensus.
// NOTE: unsafe; Lock/Unlock must be managed by caller
func (mem *Mempool) Update(height uint64, txs types.Txs) error {
	// First, create a lookup map of txns in new txs.
	txsMap := make(map[string]struct{})
	for _, tx := range txs {
		txsMap[tx.Hash().String()] = struct{}{}
	}

	// Set height
	mem.height = height
	mem.notifiedTxsAvailable = false

	// Remove transactions that are already in txs.
	mem.filterTxs(txsMap)

	atomic.StoreInt32(&mem.rechecking, 1)
	// Recheck transactions that are already in goodTxs.
	mem.recheckTxs()

	if mem.SpecGoodTxsSize() > 0 {
		mem.recheckSpecTxs()
	}
	atomic.StoreInt32(&mem.rechecking, 0)
	// Gather all executable transactions and promote them
	mem.promoteExecutables(nil)

	if mem.GoodTxsSize() > 0 || mem.SpecGoodTxsSize() > 0 {
		mem.logger.Info("mem.notifyTxsAvailable start")
		mem.notifyTxsAvailable()
		mem.logger.Info("mem.notifyTxsAvailable end")
	}
	mem.metrics.Size.Set(float64(mem.GoodTxsSize()))
	return nil
}

func (mem *Mempool) filterTxs(blockTxsMap map[string]struct{}) {
	filterList := []*clist.CList{mem.goodTxs, mem.specGoodTxs}
	for _, txsList := range filterList {
		for e := txsList.Front(); e != nil; e = e.Next() {
			memTx := e.Value.(*mempoolTx)
			// Remove the tx if it's alredy in a block.
			if _, ok := blockTxsMap[memTx.tx.Hash().String()]; ok {
				txsList.Remove(e)
				mem.cache.Remove(memTx.tx)
				e.DetachPrev()
			}
		}
	}
}

// NOTE: pass in goodTxs because mem.txs can mutate concurrently.
func (mem *Mempool) recheckTxs() {
	var err error
	filterList := []*clist.CList{}
	if mem.goodTxs.Len() > 0 {
		filterList = append(filterList, mem.goodTxs)
	}

	if len(filterList) <= 0 {
		return
	}

	for i, txsList := range filterList {
		mem.logger.Info("recheckTxs start", "i", i, "len", txsList.Len())
		for e := txsList.Front(); e != nil; e = e.Next() {
			memTx := e.Value.(*mempoolTx)
			if err = mem.app.CheckTx(memTx.tx, StateCheck); err == nil {
				// goodTx, do nothing
				continue
			} else if err == types.ErrNonceTooHigh {
				// nonce too high, move goodTxs to futureTxs
				err = mem.addFutureTx(memTx.tx)
			}
			txsList.Remove(e)
			e.DetachPrev()
			if err != nil {
				mem.cache.Remove(memTx.tx)
			}
			mem.logger.Debug("removeGoodTx when recheck", "hash", memTx.tx.Hash().String(), "err", err)
		}
		mem.logger.Info("recheckTxs end", "i", i, "len", txsList.Len())
	}
}

func (mem *Mempool) recheckSpecTxs() {
	var err error
	mem.logger.Info("recheckSpecTxs start", "len", mem.SpecGoodTxsSize())
	for e := mem.specGoodTxs.Front(); e != nil; e = e.Next() {
		timeout := true
		memTx := e.Value.(*mempoolTx)
		if time.Since(*memTx.addtime) < mem.config.Lifetime {
			timeout = false
			if err = mem.app.CheckTx(memTx.tx, StateCheck); err == nil {
				continue
			}
		}
		mem.specGoodTxs.Remove(e)
		e.DetachPrev()
		mem.cache.Remove(memTx.tx)
		name, hash := memTx.tx.TypeName, memTx.tx.Hash().String()
		mem.logger.Debug("remove SpecGoodTx when recheck", "type", name, "hash", hash, "err", err, "timeout", timeout)
	}
	mem.logger.Info("recheckSpecTxs end", "len", mem.SpecGoodTxsSize())
}

//--------------------------------------------------------------------------------

// mempoolTx is a transaction that successfully ran
type mempoolTx struct {
	tx      types.Tx
	addtime *time.Time
}

//--------------------------------------------------------------------------------

type txCache interface {
	Reset()
	VerifyTxFromCache(hash common.Hash) (*common.Address, bool)
	Exists(tx types.Tx) bool
	Push(tx types.Tx) bool
	SetTxFrom(hash common.Hash, from *common.Address)
	Remove(tx types.Tx)
	Size() int
}

// mapTxCache maintains a cache of transactions.
type mapTxCache struct {
	mtx      sync.RWMutex
	size     int
	mapCache map[common.Hash]*common.Address
}

var _ txCache = (*mapTxCache)(nil)

// newMapTxCache returns a new mapTxCache.
func newMapTxCache(cacheSize int) *mapTxCache {
	return &mapTxCache{
		size:     cacheSize,
		mapCache: make(map[common.Hash]*common.Address, cacheSize),
	}
}

func (cache *mapTxCache) Size() int {
	cache.mtx.Lock()
	size := len(cache.mapCache)
	cache.mtx.Unlock()
	return size
}

// Reset resets the cache to an empty state.
func (cache *mapTxCache) Reset() {
	cache.mtx.Lock()
	cache.mapCache = make(map[common.Hash]*common.Address, cache.size)
	cache.mtx.Unlock()
}

// Push adds the given tx to the cache and returns true. It returns false if tx
// is already in the cache.
func (cache *mapTxCache) Push(tx types.Tx) bool {
	cache.mtx.Lock()
	if _, exists := cache.mapCache[tx.Hash()]; exists {
		cache.mtx.Unlock()
		return false
	}
	cache.mapCache[tx.Hash()] = nil
	cache.mtx.Unlock()
	return true
}

// Exists returns true if tx is already in the cache.
func (cache *mapTxCache) Exists(tx types.Tx) bool {
	cache.mtx.RLock()
	_, exists := cache.mapCache[tx.Hash()]
	cache.mtx.RUnlock()
	return exists
}

func (cache *mapTxCache) VerifyTxFromCache(hash common.Hash) (*common.Address, bool) {
	cache.mtx.RLock()
	v, exists := cache.mapCache[hash]
	cache.mtx.RUnlock()
	if exists {
		return v, true
	}
	return nil, false
}

func (cache *mapTxCache) SetTxFrom(hash common.Hash, from *common.Address) {
	cache.mtx.Lock()
	cache.mapCache[hash] = from
	cache.mtx.Unlock()
}

// Remove removes the given tx from the cache.
func (cache *mapTxCache) Remove(tx types.Tx) {
	cache.mtx.Lock()
	txHash := tx.Hash()
	delete(cache.mapCache, txHash)

	cache.mtx.Unlock()
}

type nopTxCache struct{}

var _ txCache = (*nopTxCache)(nil)

func (nopTxCache) Size() int {
	return 0
}

func (nopTxCache) VerifyTxFromCache(hash common.Hash) (*common.Address, bool) { return nil, false }
func (nopTxCache) Exists(types.Tx) bool                                       { return false }
func (nopTxCache) Reset()                                                     {}
func (nopTxCache) Push(types.Tx) bool                                         { return true }
func (nopTxCache) SetTxFrom(hash common.Hash, from *common.Address)           {}
func (nopTxCache) Remove(types.Tx)                                            {}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addresssByHeartbeat []addressByHeartbeat

func (a addresssByHeartbeat) Len() int           { return len(a) }
func (a addresssByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addresssByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

//-------UTXO----------
func (m *Mempool) KeyImageReset() {
	m.kImageMtx.Lock()
	defer m.kImageMtx.Unlock()
	m.kImageCache = make(map[lktypes.Key]bool)
}

func (m *Mempool) KeyImageExists(key lktypes.Key) bool {
	m.kImageMtx.RLock()
	defer m.kImageMtx.RUnlock()
	return m.kImageCache[key]
}

func (m *Mempool) KeyImagePush(key lktypes.Key) bool {
	m.kImageMtx.Lock()
	defer m.kImageMtx.Unlock()
	if m.kImageCache[key] {
		return false
	}
	m.kImageCache[key] = true
	log.Debug("KeyImagePush push image cache", "key", key)
	return true
}

func (m *Mempool) KeyImageRemove(key lktypes.Key) {
	m.kImageMtx.Lock()
	defer m.kImageMtx.Unlock()
	delete(m.kImageCache, key)
}

func (m *Mempool) KeyImageRemoveKeys(keys []*lktypes.Key) {
	m.kImageMtx.Lock()
	defer m.kImageMtx.Unlock()
	for _, key := range keys {
		delete(m.kImageCache, *key)
		log.Debug("KeyImageRemoveKeys delete image cache", "key", *key)
	}
}
