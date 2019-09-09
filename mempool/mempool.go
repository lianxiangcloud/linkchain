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
	types.TxUTXO:            struct{}{},
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
	types.TxUTXO:            struct{}{},
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
	utxoTxs              *clist.CList // for utxo input purelly
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
	kImageCache map[lktypes.Key]struct{}
}

// MemFunc sets an optional parameter on the Mempool.
type MemFunc func(*Mempool)

// NewMempool returns a new Mempool with the given configuration and height.
func NewMempool(config *cfg.MempoolConfig, height uint64, sw p2p.P2PManager, options ...MemFunc) *Mempool {
	sigWorkers := int64((runtime.NumCPU() + 3) >> 2)
	sem := semaphore.NewWeighted(sigWorkers)
	mempool := &Mempool{
		config:          config,
		utxoTxs:         clist.New(),
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
		// mempool.cache = newMapTxCache(config.CacheSize)
		mempool.cache = newTxHeapManager(4, 30) // @Todo: make it configurable??
	} else {
		mempool.cache = nopTxCache{}
	}

	mempool.kImageCache = make(map[lktypes.Key]struct{})

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
	pending += mem.utxoTxs.Len()
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
// func (mem *Mempool) VerifyTxFromCache(hash common.Hash) (*common.Address, bool) {
func (mem *Mempool) GetTxFromCache(hash common.Hash) types.Tx {
	return mem.cache.Get(hash)
}

// UTXOTxsSize return pure utxoTxs list length.
func (mem *Mempool) UTXOTxsSize() int {
	return mem.utxoTxs.Len()
}

// GoodTxsSize returns goodTxs list length.
func (mem *Mempool) GoodTxsSize() int {
	return mem.goodTxs.Len()
}

// SpecGoodTxsSize returns specGoodList length.
func (mem *Mempool) SpecGoodTxsSize() int {
	return mem.specGoodTxs.Len()
}

func (mem *Mempool) utxoTxsFront() *clist.CElement {
	return mem.utxoTxs.Front()
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
	isOnlyUtxoInput := false
	from, _ := tx.From()
	if from == common.EmptyAddress { // pure utxo input
		isOnlyUtxoInput = true
	}

	if isOnlyUtxoInput {
		if mem.utxoTxs.Len() >= mem.config.Size {
			return types.ErrMempoolIsFull
		}
	}

	// @Note: we have already add nonce at tx.CheckState;
	if err := mem.app.CheckTx(tx, StateCheck); err != nil {
		if err == types.ErrNonceTooHigh { // has account input
			if mem.futureTxsCount >= mem.config.FutureSize {
				return types.ErrMempoolIsFull
			}
			if from == common.EmptyAddress {
				panic("addUTXOTx: from is empty, should not happen")
			}
			err = mem.addFutureTx(tx)
			log.Debug("add UTXOTx to Future", "tx", tx.Hash, "from", from, "err", err)
		}
		return err
	}

	if isOnlyUtxoInput {
		mem.addPureUtxoTx(tx)
	} else {
		mem.addGoodTx(tx, true)
	}
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
			mem.cache.Delete(tx.Hash())
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
	if !mem.cache.Put(tx) {
		return types.ErrTxDuplicate
	}
	// END CACHE

	if _, exist := canAddTxType[tx.TypeName()]; !exist {
		mem.cache.Delete(tx.Hash())
		return types.ErrParams
	}

	if err = mem.app.CheckTx(tx, BasicCheck); err != nil {
		mem.cache.Delete(tx.Hash())
		return
	}

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
					if types.BlacklistInstance().IsBlackAddress(common.EmptyAddress, aOutput.To, txUtxo.TokenID) {
						mem.cache.Delete(tx.Hash())
						return types.ErrBlacklistAddress
					}
				}
			}
		}
		if (txUtxo.UTXOKind() & types.Ain) == types.Ain {
			fromAddr, _ := txUtxo.From()
			if types.BlacklistInstance().IsBlackAddress(fromAddr, common.EmptyAddress, txUtxo.TokenID) {
				mem.cache.Delete(tx.Hash())
				return types.ErrBlacklistAddress
			}
		}
		err = mem.addUTXOTx(tx)
	case *types.Transaction, *types.TokenTransaction, *types.ContractCreateTx, *types.ContractUpgradeTx:
		// blacklist check
		var fromAddr, toAddr common.Address
		fromAddr, _ = tx.From()
		if tx.To() != nil {
			toAddr = *tx.To()
		} else {
			toAddr = common.EmptyAddress
		}
		if types.BlacklistInstance().IsBlackAddress(fromAddr, toAddr, tx.(types.RegularTx).TokenAddress()) {
			mem.cache.Delete(tx.Hash())
			return types.ErrBlacklistAddress
		}
		err = mem.addLocalTx(tx)
	case *types.MultiSignAccountTx:
		err = mem.addLocalSpecTx(tx)
	default:
		mem.logger.Error("AddTx fail,Invaild tx type")
		mem.cache.Delete(tx.Hash())
		return types.ErrParams
	}

	if err != nil {
		mem.cache.Delete(tx.Hash())
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
	tick := time.NewTicker(time.Duration(3) * time.Second)
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
	msg := TxHashMessage{Hashs: []common.Hash{tx.Hash()}, Kind: TxHashNotify}
	data, err := ser.EncodeToBytesWithType(&msg)
	if err != nil {
		logger.Error("BroadcastTx: mempool marshal tx fail", "hash", tx.Hash())
		return
	}
	sw.BroadcastE(MempoolChannel, peerID, data)
}

func (mem *Mempool) addSpecGoodTx(tx types.Tx) {
	addtime := time.Now()
	memTx := &mempoolTx{tx: tx, addtime: &addtime}
	mem.specGoodTxs.PushBack(memTx)
	mem.logger.Debug("Added Specgood transaction", "tx", tx.Hash(), "type", tx.TypeName())
	mem.notifyTxsAvailable()
}

func (mem *Mempool) addPureUtxoTx(tx types.Tx) {
	memTx := &mempoolTx{tx: tx}
	mem.utxoTxs.PushBack(memTx)
	mem.logger.Debug("Added pure utxo transaction", "tx", tx.Hash(), "type", tx.TypeName())
	mem.notifyTxsAvailable()
}

// addGoodTx add a transaction to goodTxs
func (mem *Mempool) addGoodTx(tx types.Tx, notPromote bool) {
	memTx := &mempoolTx{tx: tx}
	mem.goodTxs.PushBack(memTx)
	mem.logger.Debug("Added good transaction", "tx", tx.Hash(), "type", tx.TypeName())
	mem.metrics.Size.Set(float64(mem.GoodTxsSize()))
	mem.notifyTxsAvailable()

	if _, exist := canPromoteTxType[tx.TypeName()]; !exist {
		return
	}
	if notPromote {
		from, _ := tx.From()
		mem.promoteExecutables([]common.Address{from})
	}
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
			mem.cache.Delete(tx.Hash())
			mem.futureTxsCount--
			mem.logger.Trace("Removed old futureTx", "tx", tx.Hash(), "nonce", tx.Nonce())
		}

		// Gather all executable transactions and promote them
		if need := mem.config.Size - mem.GoodTxsSize(); need > 0 {
			startNonce := mem.app.GetNonce(addr)
			endNonce := startNonce + uint64(need)
			promoting = list.Ready(startNonce, endNonce)
			goodSize := mem.GoodTxsSize()
			for _, tx := range promoting {
				if goodSize > mem.config.Size { // GoodTx is full
					break
				}

				if err := mem.app.CheckTx(tx, StateCheck); err != nil {
					mem.cache.Delete(tx.Hash())
					mem.logger.Error("promoting transaction", "tx", tx.Hash().Hex(), "err", err)
				} else {
					mem.addGoodTx(tx, false)
					mem.logger.Trace("move futureTx to goodTxs", "tx", tx.Hash().Hex(), "nonce", tx.Nonce())
					goodSize++
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
				mem.cache.Delete(tx.Hash())
				mem.futureTxsCount--
				mem.logger.Trace("Removed cap-exceeding futureTx", "tx", tx.Hash(), "nonce", tx.Nonce())
			}
		}

		// Deleteete the entire queue entry if it became empty.
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
		mem.cache.Delete(etx.Hash())
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
	if mem.GoodTxsSize() == 0 && mem.SpecGoodTxsSize() == 0 && mem.UTXOTxsSize() == 0 {
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

	mem.logger.Info("Reap start", "maxTxs", maxTxs, "utxoTxs", mem.UTXOTxsSize(), "goodTxs", mem.GoodTxsSize(), "specTx", mem.SpecGoodTxsSize())
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

	utxoTxs := mem.collectTxs(mem.utxoTxs, mem.config.UTXOSize)     // get all pure utxo txs
	specTxs := mem.collectTxs(mem.specGoodTxs, mem.config.SpecSize) //get all special Txs

	maxTxs = maxTxs - len(specTxs) - len(utxoTxs)
	txs := mem.collectTxs(mem.goodTxs, maxTxs)
	mem.logger.Info("Reap end", "utxoTxs", len(utxoTxs), "specTxs", len(specTxs), "txsLen", len(txs), "maxTxs", maxTxs)
	txs = append(txs, utxoTxs...)
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

	if mem.UTXOTxsSize() > 0 {
		mem.recheckUtxoTxs()
	}
	atomic.StoreInt32(&mem.rechecking, 0)
	// Gather all executable transactions and promote them
	mem.promoteExecutables(nil)

	if mem.GoodTxsSize() > 0 || mem.SpecGoodTxsSize() > 0 || mem.UTXOTxsSize() > 0 {
		mem.logger.Info("mem.notifyTxsAvailable start")
		mem.notifyTxsAvailable()
		mem.logger.Info("mem.notifyTxsAvailable end")
	}
	mem.metrics.Size.Set(float64(mem.GoodTxsSize()))
	return nil
}

func (mem *Mempool) filterTxs(blockTxsMap map[string]struct{}) {
	filterList := []*clist.CList{mem.goodTxs, mem.utxoTxs, mem.specGoodTxs}
	for _, txsList := range filterList {
		for e := txsList.Front(); e != nil; e = e.Next() {
			memTx := e.Value.(*mempoolTx)
			// Remove the tx if it's alredy in a block.
			if _, ok := blockTxsMap[memTx.tx.Hash().String()]; ok {
				txsList.Remove(e)
				mem.cache.DelayDelete(memTx.tx.Hash())
				e.DetachPrev()
			}
		}
	}
}

func (mem *Mempool) recheckUtxoTxs() {
	var err error
	mem.logger.Info("recheckUtxoTxs start", "len", mem.UTXOTxsSize())
	for e := mem.utxoTxs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		if err = mem.app.CheckTx(memTx.tx, StateCheck); err == nil {
			continue
		}
		mem.utxoTxs.Remove(e)
		e.DetachPrev()
		mem.cache.Delete(memTx.tx.Hash())
		mem.logger.Debug("removeUtxoTx when recheck", "hash", memTx.tx.Hash(), "err", err)
	}
	mem.logger.Info("recheckUtxoTxs end", "len", mem.UTXOTxsSize())
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
				mem.cache.Delete(memTx.tx.Hash())
			}
			mem.logger.Debug("removeGoodTx when recheck", "hash", memTx.tx.Hash(), "err", err)
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
		mem.cache.Delete(memTx.tx.Hash())
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
	Put(tx types.Tx) bool
	Get(common.Hash) types.Tx
	Delete(common.Hash)
	DelayDelete(common.Hash)
	Exists(common.Hash) bool
	Size() int
}

type nopTxCache struct{}

var _ txCache = (*nopTxCache)(nil)

func (nopTxCache) Size() int {
	return 0
}

func (nopTxCache) Get(common.Hash) types.Tx { return nil }
func (nopTxCache) Exists(common.Hash) bool  { return false }
func (nopTxCache) Put(types.Tx) bool        { return true }
func (nopTxCache) Delete(common.Hash)       {}
func (nopTxCache) DelayDelete(common.Hash)  {}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addresssByHeartbeat []addressByHeartbeat

func (a addresssByHeartbeat) Len() int           { return len(a) }
func (a addresssByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addresssByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

//--------------------------------

type expireHash struct {
	hash   common.Hash
	expire int64 // in seconds
}

type expireHashHeap []expireHash

func (h expireHashHeap) Len() int            { return len(h) }
func (h expireHashHeap) Less(i, j int) bool  { return h[i].expire < h[j].expire }
func (h expireHashHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *expireHashHeap) Push(x interface{}) { *h = append(*h, x.(expireHash)) }
func (h *expireHashHeap) Pop() interface{} {
	res := (*h)[len(*h)-1]
	*h = (*h)[:len(*h)-1]
	return res
}

type txHeap struct {
	sync.RWMutex
	items  *expireHashHeap
	txMap  map[common.Hash]types.Tx
	expire int64 // in seconds
}

func newTxHeap(expire int64) *txHeap {
	items := make([]expireHash, 0, 100000)
	h := &txHeap{
		items:  (*expireHashHeap)(&items),
		txMap:  make(map[common.Hash]types.Tx, 100000),
		expire: expire,
	}

	go h.loop()
	return h
}

func (h *txHeap) Put(tx types.Tx) bool {
	hash := tx.Hash()

	flag := false
	h.Lock()
	if _, ok := h.txMap[hash]; !ok {
		h.txMap[hash] = tx
		flag = true
	}
	h.Unlock()
	return flag
}

func (h *txHeap) Get(hash common.Hash) types.Tx {
	h.RLock()
	tx := h.txMap[hash]
	h.RUnlock()
	return tx
}

func (h *txHeap) DelayDelete(hash common.Hash) {
	now := time.Now().Unix()
	h.Lock()
	h.items.Push(expireHash{hash: hash, expire: now + h.expire})
	h.Unlock()
}

func (h *txHeap) Delete(hash common.Hash) {
	h.Lock()
	delete(h.txMap, hash)
	h.Unlock()
}

func (h *txHeap) Exists(hash common.Hash) bool {
	h.RLock()
	_, ok := h.txMap[hash]
	h.RUnlock()
	return ok
}

func (h *txHeap) Len() int {
	h.RLock()
	n := h.items.Len()
	h.RUnlock()
	return n
}

func (h *txHeap) Size() int {
	h.RLock()
	n := len(h.txMap)
	h.RUnlock()
	return n
}

func (h *txHeap) loop() {
	for {
		size := h.Len()
		if size == 0 {
			time.Sleep(time.Millisecond * 1500)
			continue
		}

		h.Lock()
		item := h.items.Pop().(expireHash)
		h.Unlock()

		now := time.Now().Unix()
		delta := item.expire - now
		if delta > 0 {
			time.Sleep(time.Second * time.Duration(delta))
		}

		h.Delete(item.hash)
		log.Debug("txHeap delete", "hash", item.hash)
	}
}

type txHeapManager struct {
	h []*txHeap
}

func newTxHeapManager(nums int, expire int64) *txHeapManager {
	h := make([]*txHeap, 0, nums)
	for i := 0; i < nums; i++ {
		h = append(h, newTxHeap(expire))
	}

	return &txHeapManager{
		h: h,
	}
}

func (m *txHeapManager) Size() int {
	size := 0
	for _, h := range m.h {
		size += h.Size()
	}
	return size
}

func (m *txHeapManager) Put(tx types.Tx) bool {
	hash := tx.Hash()
	index := int(hash[0]) % len(m.h)
	return m.h[index].Put(tx)
}

func (m *txHeapManager) Get(hash common.Hash) types.Tx {
	index := int(hash[0]) % len(m.h)
	return m.h[index].Get(hash)
}

func (m *txHeapManager) DelayDelete(hash common.Hash) {
	index := int(hash[0]) % len(m.h)
	m.h[index].DelayDelete(hash)
}

func (m *txHeapManager) Delete(hash common.Hash) {
	index := int(hash[0]) % len(m.h)
	m.h[index].Delete(hash)
}

func (m *txHeapManager) Exists(hash common.Hash) bool {
	index := int(hash[0]) % len(m.h)
	return m.h[index].Exists(hash)
}

//-------UTXO----------
func (m *Mempool) KeyImageReset() {
	m.kImageMtx.Lock()
	m.kImageCache = make(map[lktypes.Key]struct{})
	m.kImageMtx.Unlock()
}

func (m *Mempool) KeyImageExists(key lktypes.Key) bool {
	ok := false

	m.kImageMtx.RLock()
	_, ok = m.kImageCache[key]
	m.kImageMtx.RUnlock()

	return ok
}

func (m *Mempool) KeyImagePush(key lktypes.Key) bool {
	m.kImageMtx.Lock()
	if _, ok := m.kImageCache[key]; ok {
		return false
	}
	m.kImageCache[key] = struct{}{}
	m.kImageMtx.Unlock()

	log.Debug("KeyImagePush push image cache", "key", key)
	return true
}

func (m *Mempool) KeyImageRemove(key lktypes.Key) {
	m.kImageMtx.Lock()
	delete(m.kImageCache, key)
	m.kImageMtx.Unlock()
}

func (m *Mempool) KeyImageRemoveKeys(keys []*lktypes.Key) {
	m.kImageMtx.Lock()
	for _, key := range keys {
		delete(m.kImageCache, *key)
		log.Debug("KeyImageRemoveKeys delete image cache", "key", *key)
	}
	m.kImageMtx.Unlock()
}
