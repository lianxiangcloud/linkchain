// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package discover implements the Node Discovery Protocol.
//
// The Node Discovery protocol provides a way to find RLPx nodes that
// can be connected to. It uses a Kademlia-like protocol to maintain a
// distributed database of the IDs and endpoints of all listening
// nodes.
package discover

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/lianxiangcloud/linkchain/bootcli"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/p2p/netutil"
	"github.com/lianxiangcloud/linkchain/types"

	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
)

const (
	alpha           = 3  // Kademlia concurrency factor
	bucketSize      = 16 // Kademlia bucket size
	maxReplacements = 10 // Size of per-bucket replacement list

	// We keep buckets for the upper 1/15 of distances because
	// it's very unlikely we'll ever encounter a node that's closer.
	hashBits          = len(common.Hash{}) * 8
	nBuckets          = hashBits / 15       // Number of buckets(17)
	bucketMinDistance = hashBits - nBuckets // Log distance of closest bucket

	// IP address limits.
	bucketIPLimit, bucketSubnet = 2, 24 // at most 2 addresses from the same /24
	tableIPLimit, tableSubnet   = 10, 24

	refreshInterval    = 30 * time.Minute
	revalidateInterval = 10 * time.Second
	copyNodesInterval  = 45 * time.Minute
	seedMinTableTime   = 5 * time.Minute
	seedCount          = 30
	seedMaxAge         = 5 * 24 * time.Hour
)

// DhtTable is the 'node table', a Kademlia-like index of neighbor nodes. The table keeps
// itself up-to-date by verifying the liveness of neighbors and requesting their node
// records when announcements of a new record version are received.
type DhtTable struct {
	maxDhtDialOutNums int
	bootSvr           string            //addr of bootnode server
	nodeType          types.NodeType    //NodePeer
	mutex             sync.Mutex        // protects buckets, bucket content, seeds, rand
	buckets           [nBuckets]*bucket // index of known nodes by distance
	seeds             []*node           // bootstrap nodes
	seedsNum          int
	rand              *mrand.Rand // source of randomness, periodically reseeded
	ips               netutil.DistinctNetSet

	log           log.Logger
	db            common.P2pDBManager // database of known nodes
	refreshReq    chan chan struct{}
	initDone      chan struct{}
	closeReq      chan struct{}
	closed        chan struct{}
	closeOnce     sync.Once
	priv          crypto.PrivKey
	localNode     *common.Node
	udpCon        *udp
	nodeAddedHook func(*node) // for testing
}

// bucket contains nodes, ordered by their last activity. the entry
// that was most recently active is the first element in entries.
type bucket struct {
	entries      []*node // live entries, sorted by time of last contact
	replacements []*node // recently seen nodes to be used if revalidation fails
	ips          netutil.DistinctNetSet
}

//SlefInfo is myself node info
type SlefInfo struct {
	NodeType  types.NodeType
	Self      *common.Node
	ListenCon common.UDPConn
}

// NewDhtTable starts listening for discovery packets on the given UDP socket.
func NewDhtTable(maxDhtDialOutNums int, bootSvr string, self *SlefInfo, db common.P2pDBManager, cfg common.Config, log log.Logger) (*DhtTable, error) {
	log.Info("NewDhtTable")
	err := dhtPrecheck(maxDhtDialOutNums, self, db)
	if err != nil {
		return nil, err
	}
	tab, err := newDhtTable(maxDhtDialOutNums, bootSvr, self, db, cfg, log)
	if err != nil {
		return nil, err
	}
	return tab, nil
}

func dhtPrecheck(maxDhtDialOutNums int, self *SlefInfo, db common.P2pDBManager) error {
	if maxDhtDialOutNums <= 0 {
		return fmt.Errorf("maxDhtDialOutNums:%v <=0", maxDhtDialOutNums)
	}
	if self == nil || db == nil {
		return fmt.Errorf("self or db is nil")
	}
	if self.Self == nil {
		return fmt.Errorf("self.Self is nil")
	}

	return nil
}

func newDhtTable(maxDialOutNums int, bootSvr string, self *SlefInfo, db common.P2pDBManager, cfg common.Config, log log.Logger) (*DhtTable, error) {
	log.Info("newDhtTable")
	if self.ListenCon == nil {
		log.Info("self.ListenCon == nil")
		return nil, nil
	}
	tab := &DhtTable{
		maxDhtDialOutNums: maxDialOutNums,
		bootSvr:           bootSvr,
		nodeType:          self.NodeType,
		db:                db,
		refreshReq:        make(chan chan struct{}),
		initDone:          make(chan struct{}),
		closeReq:          make(chan struct{}),
		closed:            make(chan struct{}),
		rand:              mrand.New(mrand.NewSource(0)),
		ips:               netutil.DistinctNetSet{Subnet: tableSubnet, Limit: tableIPLimit},
		log:               log,
		localNode:         self.Self,
		priv:              cfg.PrivateKey,
	}
	tab.udpCon = newUDP(tab, self.ListenCon, db, self.Self, cfg, log)
	if err := tab.setFallbackNodes(cfg.SeedNodes); err != nil {
		return nil, err
	}
	for i := range tab.buckets {
		tab.buckets[i] = &bucket{
			ips: netutil.DistinctNetSet{Subnet: bucketSubnet, Limit: bucketIPLimit},
		}
	}
	tab.seedRand()
	return tab, nil
}

//Start start dht service
func (tab *DhtTable) Start() {
	if tab == nil {
		return
	}
	go tab.loop()
	tab.udpCon.start()
}

// Stop shuts down the socket and aborts any running queries.
func (tab *DhtTable) Stop() {
	tab.closeOnce.Do(func() {
		tab.udpCon.close()
		tab.close()
	})
}

//GetMaxDialOutNum return the max dialout num
func (tab *DhtTable) GetMaxDialOutNum() int {
	if tab.nodeType == types.NodePeer {
		return tab.maxDhtDialOutNums
	}
	if tab.seedsNum > 0 {
		return tab.seedsNum
	}
	return defaultSeeds
}

//GetMaxConNumFromCache return the max node's num from local cache
func (tab *DhtTable) GetMaxConNumFromCache() int {
	if tab.nodeType == types.NodePeer {
		return tab.maxDhtDialOutNums / 2
	}
	return len(tab.seeds)
}

func (tab *DhtTable) self() *common.Node {
	return tab.localNode
}

func (tab *DhtTable) seedRand() {
	var b [8]byte
	crand.Read(b[:])

	tab.mutex.Lock()
	tab.rand.Seed(int64(binary.BigEndian.Uint64(b[:])))
	tab.mutex.Unlock()
}

// LookupRandom finds random nodes in the network.
func (tab *DhtTable) LookupRandom() []*common.Node {
	if tab.nodeType == types.NodePeer {
		if tab.len() == 0 {
			// All nodes were dropped, refresh. The very first query will hit this
			// case and run the bootstrapping logic.
			<-tab.Refresh()
		}
		return tab.lookupRandom()
	} else {
		return tab.getRandSeedsFromBootSvr()
	}
}

func (tab *DhtTable) getRandSeedsFromBootSvr() []*common.Node {
	var splitedNodes []*common.Node
	seedNodes, _, _ := bootcli.GetSeeds(tab.bootSvr, tab.priv, tab.log)
	if len(seedNodes) > 0 {
		seedsNum := 0
		seedsMap := make(map[string]bool)
		myID := common.TransPubKeyToNodeID(tab.priv.PubKey())
		for i := 0; i < len(seedNodes); i++ {
			if seedNodes[i].ID == myID { //it is my self,skip
				continue
			}
			splitedNodes = append(splitedNodes, seedNodes[i])
			_, ok := seedsMap[seedNodes[i].ID.String()]
			if ok {
				continue
			} else {
				seedsMap[seedNodes[i].ID.String()] = true
				seedsNum++
			}
		}
		tab.seedsNum = seedsNum
		tab.seeds = wrapNodes(splitedNodes)
	}
	// Shuffle the buckets.
	for i := len(tab.seeds) - 1; i > 0; i-- {
		j := tab.rand.Intn(len(tab.seeds))
		tab.seeds[i], tab.seeds[j] = tab.seeds[j], tab.seeds[i]
	}
	return splitedNodes
}

func (tab *DhtTable) IsDhtTable() bool {
	return true
}

// ReadRandomNodes fills the given slice with random nodes from the table. The results
// are guaranteed to be unique for a single invocation, no node will appear twice.
func (tab *DhtTable) ReadRandomNodes(buf []*common.Node) (nodeNum int) {
	if !tab.isInitDone() {
		tab.log.Info("isInitDone not done")
		return 0
	}
	if tab.nodeType == types.NodePeer {
		nodeNum = tab.readNodesFromBucket(buf)
	} else {
		var i = 0
		for ; i < len(buf) && i < len(tab.seeds); i++ {
			buf[i] = unwrapNode(tab.seeds[i])
		}
		nodeNum = i
		// Shuffle the buf.
		for i := nodeNum - 1; i > 0; i-- {
			j := tab.rand.Intn(nodeNum)
			tab.seeds[i], tab.seeds[j] = tab.seeds[j], tab.seeds[i]
		}
	}

	return
}

func (tab *DhtTable) ReadNodesFromKbucket(buf []*common.Node) (nodeNum int) {
	if !tab.isInitDone() {
		tab.log.Info("isInitDone not done")
		return 0
	}
	nodeNum = tab.readNodesFromBucket(buf)
	return
}

func (tab *DhtTable) readNodesFromBucket(buf []*common.Node) (n int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	// Find all non-empty buckets and get a fresh slice of their entries.
	var buckets [][]*node
	for _, b := range &tab.buckets {
		if len(b.entries) > 0 {
			buckets = append(buckets, b.entries)
		}
	}
	if len(buckets) == 0 {
		tab.log.Info("DhtTable len(buckets) = 0")
		return 0
	}
	// Shuffle the buckets.
	for i := len(buckets) - 1; i > 0; i-- {
		j := tab.rand.Intn(len(buckets))
		buckets[i], buckets[j] = buckets[j], buckets[i]
	}
	// Move head of each bucket into buf, removing buckets that become empty.
	var i, j int
	for ; i < len(buf); i, j = i+1, (j+1)%len(buckets) {
		b := buckets[j]
		buf[i] = unwrapNode(b[0])
		buckets[j] = b[1:]
		if len(b) == 1 {
			buckets = append(buckets[:j], buckets[j+1:]...)
		}
		if len(buckets) == 0 {
			break
		}
	}
	return i + 1
}

// getNode returns the node with the given ID or nil if it isn't in the table.
func (tab *DhtTable) getNode(id common.NodeID) *common.Node {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	b := tab.bucket(id)
	for _, e := range b.entries {
		if e.ID == id {
			return unwrapNode(e)
		}
	}
	return nil
}

// close terminates the network listener and flushes the node database.
func (tab *DhtTable) close() {
	close(tab.closeReq)
	<-tab.closed
}

// setFallbackNodes sets the initial points of contact. These nodes
// are used to connect to the network if the table is empty and there
// are no known nodes in the database.
func (tab *DhtTable) setFallbackNodes(nodes []*common.Node) error {
	var splitedNodes []*common.Node
	seedsNum := 0
	seedsMap := make(map[string]bool)
	myID := common.TransPubKeyToNodeID(tab.priv.PubKey())
	for _, n := range nodes {
		if err := n.ValidateComplete(); err != nil {
			tab.log.Debug("bad bootstrap node", "err", err, "n.ip", n.IP.String(), "n.ID", n.ID, "n.UDP_Port", n.UDP_Port)
			continue
		}

		if n.ID == myID { //it is my self,skip
			//if seedNodes[i].ID == common.NodeID(crypto.Keccak256Hash(tab.priv.PubKey().Bytes()))
			tab.log.Debug("is is myself", "n.ID", n.ID, "myID", myID)
			continue
		}
		splitedNodes = append(splitedNodes, n)
		_, ok := seedsMap[n.ID.String()]
		if ok {
			continue
		} else {
			seedsMap[n.ID.String()] = true
			seedsNum++
		}
	}
	tab.seedsNum = seedsNum
	tab.seeds = wrapNodes(splitedNodes)
	tab.log.Info("DhtTable setFallbackNodes", "seedsNum", seedsNum)
	return nil
}

// isInitDone returns whether the table's initial seeding procedure has completed.
func (tab *DhtTable) isInitDone() bool {
	select {
	case <-tab.initDone:
		return true
	default:
		return false
	}
}

func (tab *DhtTable) Refresh() <-chan struct{} {
	done := make(chan struct{})
	select {
	case tab.refreshReq <- done:
	case <-tab.closeReq:
		close(done)
	}
	return done
}

func (tab *DhtTable) PrinttAllnodes() {
	for i, b := range tab.buckets {
		tab.log.Debug("PrinttAllnodes", "i", i, "len(b.entries)", len(b.entries))
		for _, n := range b.entries {
			tab.log.Debug("PrinttAllnodes", "ID", n.ID, "ip", n.IP.String(), "udp_port", n.UDP_Port, "tcp_port", n.TCP_Port)
		}
	}
}

// loop schedules runs of doRefresh, doRevalidate and copyLiveNodes.
func (tab *DhtTable) loop() {
	tab.log.Debug("DhtTable loop")
	var (
		revalidate     = time.NewTimer(tab.nextRevalidateTime())
		refresh        = time.NewTicker(refreshInterval)
		copyNodes      = time.NewTicker(copyNodesInterval)
		refreshDone    = make(chan struct{})           // where doRefresh reports completion
		revalidateDone chan struct{}                   // where doRevalidate reports completion
		waiting        = []chan struct{}{tab.initDone} // holds waiting callers while doRefresh runs
	)
	defer refresh.Stop()
	defer revalidate.Stop()
	defer copyNodes.Stop()

	// Start initial refresh.
	go tab.doRefresh(refreshDone)
loop:
	for {
		select {
		case <-refresh.C:
			tab.seedRand()
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				go tab.doRefresh(refreshDone)
			}
		case req := <-tab.refreshReq:
			waiting = append(waiting, req)
			if refreshDone == nil {
				refreshDone = make(chan struct{})
				go tab.doRefresh(refreshDone)
			}
		case <-refreshDone:
			for _, ch := range waiting {
				close(ch)
			}
			waiting, refreshDone = nil, nil
		case <-revalidate.C:
			revalidateDone = make(chan struct{})
			go tab.doRevalidate(revalidateDone)
		case <-revalidateDone:
			revalidate.Reset(tab.nextRevalidateTime())
			revalidateDone = nil
		case <-copyNodes.C:
			go tab.copyLiveNodes()
		case <-tab.closeReq:
			break loop
		}
	}

	if refreshDone != nil {
		<-refreshDone
	}
	for _, ch := range waiting {
		close(ch)
	}
	if revalidateDone != nil {
		<-revalidateDone
	}
	close(tab.closed)
}

// lookup performs a network search for nodes close to the given target. It approaches the
// target by querying nodes that are closer to it on each iteration. The given target does
// not need to be an actual node identifier.
func (tab *DhtTable) lookup(target common.NodeID) []*node {
	var (
		asked          = make(map[common.NodeID]bool)
		seen           = make(map[common.NodeID]bool)
		reply          = make(chan []*node, alpha)
		pendingQueries = 0
		result         *nodesByDistance
	)
	// Don't query further if we hit ourself.
	// Unlikely to happen often in practice.
	asked[tab.self().ID] = true

	// Generate the initial result set.
	tab.mutex.Lock()
	result = tab.closest(target, bucketSize, false)
	tab.mutex.Unlock()

	for i := 0; i < len(result.entries); i++ {
		seen[result.entries[i].ID] = true
	}

	for {
		// ask the alpha closest nodes that we haven't asked yet
		for i := 0; i < len(result.entries) && pendingQueries < alpha; i++ {
			n := result.entries[i]
			if !asked[n.ID] {
				asked[n.ID] = true
				pendingQueries++
				go tab.lookupWorker(n, target, reply) //Find the 16 nodes closest to the targetKey
			}
		}
		if pendingQueries == 0 {
			// we have asked all closest nodes, stop the search
			break
		}
		select {
		case nodes := <-reply:
			for _, n := range nodes {
				if n != nil && !seen[n.ID] && (tab.self().ID != n.ID) {
					seen[n.ID] = true
					result.push(n, bucketSize)
				}
			}
		case <-tab.closeReq:
			return nil // shutdown, no need to continue.
		}
		pendingQueries--
	}
	return result.entries
}

func (tab *DhtTable) lookupWorker(n *node, targetKey common.NodeID, reply chan<- []*node) {
	fails := tab.db.FindFails(n.ID, n.IP)
	r, err := tab.udpCon.findnode(n.ID, n.addr(), targetKey)
	if err == errClosed {
		// Avoid recording failures on shutdown.
		reply <- nil
		return
	} else if len(r) == 0 {
		fails++
		tab.db.UpdateFindFails(n.ID, n.IP, fails)
		tab.log.Debug("Findnode failed", "id", n.ID, "ip", n.IP.String(),
			"TCP_Port", n.TCP_Port, "UDP_Port", n.UDP_Port, "failcount", fails, "err", err)
		if fails >= maxFindnodeFailures {
			tab.log.Info("Too many findnode failures, dropping", "id", n.ID, "ip", n.IP.String(),
				"TCP_Port", n.TCP_Port, "UDP_Port", n.UDP_Port, "failcount", fails)
			tab.delete(n) //Delete records in the cache
		}
	} else if fails > 0 {
		// Reset failure counter because it counts _consecutive_ failures.
		tab.db.UpdateFindFails(n.ID, n.IP, 0)
	}

	// Grab as many nodes as possible. Some of them might not be alive anymore, but we'll
	// just remove those again during revalidation.
	for _, n := range r {
		tab.addSeenNode(n) //Add the latest 16 node found to kbucket
	}
	reply <- r
}

func (tab *DhtTable) lookupRandom() []*common.Node {
	var target [32]byte
	crand.Read(target[:])
	targetID := common.NodeID(crypto.Keccak256Hash(target[:]))
	return unwrapNodes(tab.lookup(targetID))
}

func (tab *DhtTable) lookupSelf() []*common.Node {
	targetID := common.TransPubKeyToNodeID(tab.priv.PubKey())
	return unwrapNodes(tab.lookup(targetID))
}

// doRefresh performs a lookup for a random target to keep buckets full. seed nodes are
// inserted if the table is empty (initial bootstrap or discarded faulty peers).
func (tab *DhtTable) doRefresh(done chan struct{}) {
	defer close(done)

	// Load nodes from the database and insert
	// them. This should yield a few previously seen nodes that are
	// (hopefully) still alive.
	tab.loadSeedNodes()

	// Run self lookup to discover new neighbor nodes.
	tab.lookupSelf()
	// The Kademlia paper specifies that the bucket refresh should
	// perform a lookup in the least recently used bucket. We cannot
	// adhere to this because the findnode target is a 512bit value
	// (not hash-sized) and it is not easily possible to generate a
	// sha3 preimage that falls into a chosen bucket.
	// We perform a few lookups with a random target instead.
	for i := 0; i < 3; i++ {
		tab.lookupRandom()
	}
}

func (tab *DhtTable) loadSeedNodes() {
	seeds := wrapNodes(tab.db.QuerySeeds(seedCount, seedMaxAge))
	if tab.nodeType == types.NodePeer {
		seeds = append(seeds, tab.seeds...)
	}
	for i := range seeds { //There may be multiple ip of the same ID in seeds
		seed := seeds[i]
		age := log.Lazy{Fn: func() interface{} { return time.Since(tab.db.LastPongReceived(seed.ID, seed.IP)) }}
		tab.log.Debug("Found seed node in database", "id", seed.ID, "addr", seed.addr(), "tcpPort", seed.TCP_Port, "udpPort", seed.UDP_Port, "age", age)
		if tab.isInbucket(seed) {
			continue
		}
		err, _ := tab.ping(&seed.Node)
		if err != nil {
			tab.log.Debug("ping failed", "err", err, "id", seed.ID, "addr", seed.addr(), "tcpPort", seed.TCP_Port, "udpPort", seed.UDP_Port)
			continue
		}
		tab.log.Trace("ping success", "id", seed.ID, "addr", seed.addr(), "tcpPort", seed.TCP_Port, "udpPort", seed.UDP_Port)
		tab.addSeenNode(seed) //only add the seeds that we can ping success
	}
}

func (tab *DhtTable) ping(n *common.Node) (error, *pong) {
	err, pong := tab.udpCon.ping(n)
	return err, pong
}

// doRevalidate checks that the last node in a random bucket is still live and replaces or
// deletes the node if it isn't.
func (tab *DhtTable) doRevalidate(done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	last, bi := tab.nodeToRevalidate()
	if last == nil {
		// No non-empty bucket found.
		return
	}

	// Ping the selected node and wait for a pong.
	err, pong := tab.ping(unwrapNode(last))
	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.buckets[bi]
	if err == nil {
		// The node responded, move it to the front.
		if pong != nil {
			if last.ID != pong.MyNodeInfo.ID {
				tab.log.Info("doRevalidate id changed", "last.ID", last.ID, "pong id", pong.MyNodeInfo.ID)
				last.Node.ID = pong.MyNodeInfo.ID
			}
		}
		last.livenessChecks++
		tab.log.Trace("Revalidated node", "b", bi, "myID", tab.localNode.ID, "remoteID", last.ID, "checks", last.livenessChecks)
		tab.bumpInBucket(b, last, false)
		return
	}
	// No reply received, pick a replacement or delete the node if there aren't
	// any replacements.
	if r := tab.replace(b, last); r != nil {
		tab.log.Info("Replaced dead node", "b", bi, "myID", tab.localNode.ID, "id", last.ID, "ip", last.IP, "checks", last.livenessChecks, "r", r.ID, "rip", r.IP)
	} else {
		tab.log.Info("Removed dead node", "b", bi, "myID", tab.localNode.ID, "id", last.ID, "ip", last.IP, "udpport", last.UDP_Port, "checks", last.livenessChecks)
	}
}

// nodeToRevalidate returns the last node in a random, non-empty bucket.
func (tab *DhtTable) nodeToRevalidate() (n *node, bi int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	for _, bi = range tab.rand.Perm(len(tab.buckets)) {
		b := tab.buckets[bi]
		if len(b.entries) > 0 {
			last := b.entries[len(b.entries)-1]
			return last, bi
		}
	}
	return nil, 0
}

func (tab *DhtTable) nextRevalidateTime() time.Duration {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	return time.Duration(tab.rand.Int63n(int64(revalidateInterval)))
}

// copyLiveNodes adds nodes from the table to the database if they have been in the table
// longer then minTableTime.
func (tab *DhtTable) copyLiveNodes() {
	tab.log.Debug("copyLiveNodes")
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	now := time.Now()
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			if n.livenessChecks > 0 && now.Sub(n.addedAt) >= seedMinTableTime {
				tab.db.UpdateNode(unwrapNode(n))
			}
		}
	}
}

// closest returns the n nodes in the table that are closest to the
// given id. The caller must hold tab.mutex.
func (tab *DhtTable) closest(target common.NodeID, nresults int, checklive bool) *nodesByDistance {
	// This is a very wasteful way to find the closest nodes but
	// obviously correct. I believe that tree-based buckets would make
	// this easier to implement efficiently.
	close := &nodesByDistance{target: target}
	for _, b := range &tab.buckets {
		for _, n := range b.entries {
			if checklive && n.livenessChecks == 0 {
				continue
			}
			close.push(n, nresults)
		}
	}
	return close
}

// len returns the number of nodes in the table.
func (tab *DhtTable) len() (n int) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	for _, b := range &tab.buckets {
		n += len(b.entries)
	}
	return n
}

// bucket returns the bucket for the given node ID hash.
func (tab *DhtTable) bucket(id common.NodeID) *bucket {
	d := LogDist(tab.self().ID, id)
	if d <= bucketMinDistance {
		return tab.buckets[0]
	}
	return tab.buckets[d-bucketMinDistance-1]
}

// addSeenNode adds a node which may or may not be live to the end of a bucket. If the
// bucket has space available, adding the node succeeds immediately. Otherwise, the node is
// added to the replacements list.
//
// The caller must not hold tab.mutex.
func (tab *DhtTable) addSeenNode(n *node) {
	if n.ID == tab.self().ID {
		return
	}

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.bucket(n.ID)
	if contains(b.entries, n.ID) {
		// Already in bucket, don't add.
		return
	}
	if len(b.entries) >= bucketSize {
		tab.log.Debug("len(b.entries) >= bucketSize", "len(b.entries)", len(b.entries))
		// Bucket full, maybe add as replacement.
		tab.addReplacement(b, n)
		return
	}
	if !tab.addIP(b, n.IP) {
		tab.log.Info("Can't add: IP limit reached", "n.IP", n.IP.String())
		// Can't add: IP limit reached.
		return
	}
	// Add to end of bucket:
	b.entries = append(b.entries, n)
	b.replacements = deleteNode(b.replacements, n)
	n.addedAt = time.Now()
	if tab.nodeAddedHook != nil {
		tab.nodeAddedHook(n)
	}
}

func (tab *DhtTable) isInbucket(n *node) bool {
	if n.ID == tab.self().ID {
		return true
	}

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.bucket(n.ID)
	if contains(b.entries, n.ID) {
		// Already in bucket, don't add.
		return true
	}
	return false
}

// addVerifiedNode adds a node whose existence has been verified recently to the front of a
// bucket. If the node is already in the bucket, it is moved to the front. If the bucket
// has no space, the node is added to the replacements list.
//
// There is an additional safety measure: if the table is still initializing the node
// is not added. This prevents an attack where the table could be filled by just sending
// ping repeatedly.
//
// The caller must not hold tab.mutex.
func (tab *DhtTable) addVerifiedNode(n *node) {
	if !tab.isInitDone() {
		return
	}
	if n.ID == tab.self().ID {
		return
	}
	ip16 := n.IP.To16()
	if ip16 == nil {
		return
	}

	tab.mutex.Lock()
	defer tab.mutex.Unlock()
	b := tab.bucket(n.ID)
	if tab.bumpInBucket(b, n, true) {
		// Already in bucket, moved to front.
		return
	}
	if len(b.entries) >= bucketSize {
		// Bucket full, maybe add as replacement.
		tab.addReplacement(b, n)
		return
	}
	if !tab.addIP(b, n.IP) {
		// Can't add: IP limit reached.
		return
	}
	// Add to front of bucket.
	b.entries, _ = pushNode(b.entries, n, bucketSize)
	b.replacements = deleteNode(b.replacements, n)
	n.addedAt = time.Now()
	if tab.nodeAddedHook != nil {
		tab.nodeAddedHook(n)
	}
}

// delete removes an entry from the node table. It is used to evacuate dead nodes.
func (tab *DhtTable) delete(node *node) {
	tab.mutex.Lock()
	defer tab.mutex.Unlock()

	tab.deleteInBucket(tab.bucket(node.ID), node)
}

func (tab *DhtTable) addIP(b *bucket, ip net.IP) bool {
	if netutil.IsLAN(ip) {
		return true
	}
	if !tab.ips.Add(ip) {
		tab.log.Debug("IP exceeds table limit", "ip", ip.String())
		return false
	}
	if !b.ips.Add(ip) {
		tab.log.Debug("IP exceeds bucket limit", "ip", ip.String())
		tab.ips.Remove(ip)
		return false
	}
	return true
}

func (tab *DhtTable) removeIP(b *bucket, ip net.IP) {
	if netutil.IsLAN(ip) {
		return
	}
	tab.ips.Remove(ip)
	b.ips.Remove(ip)
}

func (tab *DhtTable) addReplacement(b *bucket, n *node) {
	for _, e := range b.replacements {
		if e.ID == n.ID {
			return // already in list
		}
	}
	if !tab.addIP(b, n.IP) {
		return
	}
	var removed *node
	b.replacements, removed = pushNode(b.replacements, n, maxReplacements)
	if removed != nil {
		tab.removeIP(b, removed.IP)
	}
}

// replace removes n from the replacement list and replaces 'last' with it if it is the
// last entry in the bucket. If 'last' isn't the last entry, it has either been replaced
// with someone else or became active.
func (tab *DhtTable) replace(b *bucket, last *node) *node {
	if len(b.entries) == 0 || b.entries[len(b.entries)-1].ID != last.ID {
		// Entry has moved, don't replace it.
		return nil
	}
	// Still the last entry.
	if len(b.replacements) == 0 {
		tab.deleteInBucket(b, last)
		return nil
	}
	r := b.replacements[tab.rand.Intn(len(b.replacements))]
	b.replacements = deleteNode(b.replacements, r)
	b.entries[len(b.entries)-1] = r
	tab.removeIP(b, last.IP)
	return r
}

// bumpInBucket moves the given node to the front of the bucket entry list
// if it is contained in that list.
func (tab *DhtTable) bumpInBucket(b *bucket, n *node, replaceLive bool) bool {
	for i := range b.entries {
		if b.entries[i].ID == n.ID {
			if !n.IP.Equal(b.entries[i].IP) {
				// Endpoint has changed, ensure that the new IP fits into table limits.
				tab.removeIP(b, b.entries[i].IP)
				if !tab.addIP(b, n.IP) {
					// It doesn't, put the previous one back.
					tab.addIP(b, b.entries[i].IP)
					return false
				}
			}
			// Move it to the front.
			copy(b.entries[1:], b.entries[:i])
			if replaceLive {
				n.livenessChecks = b.entries[i].livenessChecks
			}
			b.entries[0] = n
			return true
		}
	}
	return false
}

func (tab *DhtTable) deleteInBucket(b *bucket, n *node) {
	b.entries = deleteNode(b.entries, n)
	tab.removeIP(b, n.IP)
}

func contains(ns []*node, id common.NodeID) bool {
	for _, n := range ns {
		if n.ID == id {
			return true
		}
	}
	return false
}

// pushNode adds n to the front of list, keeping at most max items.
func pushNode(list []*node, n *node, max int) ([]*node, *node) {
	if len(list) < max {
		list = append(list, nil)
	}
	removed := list[len(list)-1]
	copy(list[1:], list)
	list[0] = n
	return list, removed
}

// deleteNode removes n from list.
func deleteNode(list []*node, n *node) []*node {
	for i := range list {
		if list[i].ID == n.ID {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// nodesByDistance is a list of nodes, ordered by distance to target.
type nodesByDistance struct {
	entries []*node
	target  common.NodeID
}

// push adds the given node to the list, keeping the total size below maxElems.
func (h *nodesByDistance) push(n *node, maxElems int) {
	ix := sort.Search(len(h.entries), func(i int) bool {
		return DistCmp(h.target, h.entries[i].ID, n.ID) > 0
	})
	if len(h.entries) < maxElems {
		h.entries = append(h.entries, n)
	}
	if ix == len(h.entries) {
		// farther away than all nodes we already have.
		// if there was room for it, the node is now the last element.
	} else {
		// slide existing entries down to make room
		// this will overwrite the entry we just appended.
		copy(h.entries[ix+1:], h.entries[ix:])
		h.entries[ix] = n
	}
}
