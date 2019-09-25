package p2p

import (
	"fmt"
	"net"
	"sync"
	"time"

	"encoding/hex"

	"github.com/lianxiangcloud/linkchain/bootnode"
	"github.com/lianxiangcloud/linkchain/config"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/p2p/conn"
	disc "github.com/lianxiangcloud/linkchain/libs/p2p/discover"
	"github.com/lianxiangcloud/linkchain/libs/p2p/netutil"
	"github.com/lianxiangcloud/linkchain/types"
)

const (
	// wait a random amount of time from this interval
	// before dialing peers or reconnecting to help prevent DoS
	dialRandomizerIntervalMilliseconds = 3000

	// repeatedly try to reconnect for a few minutes
	// ie. 5 * 20 = 100s
	reconnectAttempts = 20
	reconnectInterval = 5 * time.Second

	// then move into exponential backoff mode for ~1day
	// ie. 3**10 = 16hrs
	reconnectBackOffAttempts          = 10
	reconnectBackOffBaseSeconds       = 3
	flushThrottleTimeout              = 73             // Time to wait before flushing messages out on the connection, in ms
	sendRate                    int64 = int64(5120000) // 5mB/s
	recvRate                    int64 = int64(5120000) // 5mB/s
	// This time limits inbound connection attempts per source IP.
	inboundThrottleTime      = 30 * time.Second
	maxInboundNumForSingleIp = 10
)

const (
	blackListTimeout = (600 * time.Second)
	defaultDialRatio = 2
)

var (
	DefaultNewTableFunc = defaultNewTable
)

//-----------------------------------------------------------------------------

// Switch handles peer connections and exposes an API to receive incoming messages
// on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
// or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
// incoming messages are received on the reactor.
type Switch struct {
	cmn.BaseService
	config        *config.P2PConfig
	listeners     []Listener
	reactors      map[string]Reactor
	chDescs       []*conn.ChannelDescriptor
	reactorsByCh  map[byte]Reactor //key:channelID
	peers         *PeerSet
	dialing       *cmn.CMap
	localNodeInfo NodeInfo // our node info
	blackListLock sync.Mutex
	blackListMap  map[string]bool //record bad node  key:node id
	nodeKey       crypto.PrivKey

	filterConnByAddr func(net.Addr) error

	mConfig conn.MConnConfig

	rng            *cmn.Rand            // seed for randomizing dial times and orders
	ntab           common.DiscoverTable //cache node from nodeserver or dht network
	manager        *ConManager          //manager the all connections with myself
	db             dbm.DB
	dm             common.P2pDBManager
	udpCon         *net.UDPConn
	upnpFlag       bool
	inboundHistory expHeap //record inbound ip in inboundThrottleTime
	inboundLock    sync.Mutex
	inboundMap     map[string]int //record connection num for single ip,only record public ip  key:ip
	whitelist      *netutil.Netlist
	blacklist      *netutil.Netlist
}

//TransNodeToEndpoint translate nodes to array of ip:port
func TransNodeToEndpoint(nodes []*common.Node) []string {
	endpoints := make([]string, len(nodes))
	for i := 0; i < len(endpoints); i++ {
		endpoints[i] = cmn.Fmt("%v:%v", nodes[i].IP, nodes[i].TCP_Port)
	}
	return endpoints
}

// NewP2pManager creates a new Switch with the given config.
func NewP2pManager(logger log.Logger, myPrivKey crypto.PrivKey, cfg *config.P2PConfig,
	localNodeInfo NodeInfo, seeds []*common.Node, db dbm.DB) (*Switch, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cfg is nil")
	}
	sw := &Switch{
		config:       cfg,
		reactors:     make(map[string]Reactor),
		chDescs:      make([]*conn.ChannelDescriptor, 0),
		reactorsByCh: make(map[byte]Reactor),
		peers:        NewPeerSet(),
		dialing:      cmn.NewCMap(),
		blackListMap: make(map[string]bool),
		db:           db,
		rng:          cmn.NewRand(), // Ensure we have a completely undeterministic PRNG.
		inboundMap:   make(map[string]int),
	}

	mConfig := conn.DefaultMConnConfig()
	mConfig.FlushThrottle = time.Duration(flushThrottleTimeout) * time.Millisecond
	mConfig.SendRate = sendRate
	mConfig.RecvRate = recvRate
	mConfig.MaxPacketMsgPayloadSize = cfg.MaxPacketMsgPayloadSize

	sw.mConfig = mConfig

	sw.BaseService = *cmn.NewBaseService(nil, "P2P Switch", sw)
	sw.nodeKey = myPrivKey
	var listener Listener
	listener, sw.udpCon, sw.upnpFlag = NewDefaultListener(localNodeInfo.Type, cfg.ListenAddress, cfg.ExternalAddress, logger)
	if listener != nil {
		sw.AddListener(listener)
		p2pHost := listener.ExternalAddressHost()
		p2pPort := listener.ExternalAddress().Port
		localNodeInfo.ListenAddr = cmn.Fmt("%v:%v", p2pHost, p2pPort)
		localip := GetLocalAllAddress()
		for _, lIP := range localip {
			localNodeInfo.LocalAddrs = append(localNodeInfo.LocalAddrs, cmn.Fmt("%v:%v", lIP, p2pPort))
		}
	}
	sw.SetNodeInfo(localNodeInfo)
	sw.SetLogger(logger)
	var err error
	err = DefaultNewTableFunc(sw, seeds)
	if err != nil {
		return nil, err
	}
	sw.newConManager()
	return sw, nil
}

func (sw *Switch) GetConManager() *ConManager {
	return sw.manager
}

func (sw *Switch) MarkBadNode(nodeInfo NodeInfo) {
	sw.Logger.Info("MarkBadNode", "nodeInfo.PubKey", nodeInfo.PubKey.String(), "id", nodeInfo.ID())
	sw.blackListLock.Lock()
	sw.blackListMap[nodeInfo.ID()] = true
	sw.blackListLock.Unlock()
	go func(id string) {
		d := time.Duration(blackListTimeout)
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
			sw.Logger.Info("MarkBadNode time to clean blacklist", "id", id)
			sw.blackListLock.Lock()
			delete(sw.blackListMap, id)
			sw.blackListLock.Unlock()
		}
	}(nodeInfo.ID())
}

func (sw *Switch) blackListHasID(nodeid string) bool {
	sw.blackListLock.Lock()
	defer sw.blackListLock.Unlock()
	_, ok := sw.blackListMap[nodeid]
	if ok {
		return true
	}
	return false
}

func (sw *Switch) newDm(db dbm.DB, needDht bool) {
	if sw.dm == nil && needDht {
		dbLogger := sw.Logger.With("module", "P2pDBManager")
		sw.dm = disc.NewDBManager(db, dbLogger)
	}
}

func (sw *Switch) NodeKey() crypto.PrivKey {
	return sw.nodeKey
}

func (sw *Switch) DBManager() common.P2pDBManager {
	return sw.dm
}

func (sw *Switch) GetTable() common.DiscoverTable {
	return sw.ntab
}

func (sw *Switch) BootNodeAddr() string {
	return bootnode.GetBestBootNode()
}

func (sw *Switch) UdpCon() *net.UDPConn {
	return sw.udpCon
}

func defaultNewTable(sw *Switch, seeds []*common.Node) error {
	needDht := false
	if bootnode.GetLocalNodeType() == types.NodePeer {
		needDht = true
	}
	return sw.DefaultNewTable(seeds, needDht, false)
}

func (sw *Switch) DefaultNewTable(seeds []*common.Node, needDht bool, needReNewUdpCon bool) error {
	var err error
	cfg := common.Config{PrivateKey: sw.nodeKey, SeedNodes: make([]*common.Node, len(seeds))}
	copy(cfg.SeedNodes, seeds)
	httpLogger := sw.Logger.With("module", "httpTable")
	sw.ntab, err = disc.NewHTTPTable(cfg, sw.BootNodeAddr(), bootnode.GetLocalNodeType(), httpLogger)
	return err
}

func (sw *Switch) newConManager() {
	conManagerLogger := sw.Logger.With("module", "conManager")
	sw.manager = NewConManager(sw, conManagerLogger)
}

//GetByID retrun the peer con by id
func (sw *Switch) GetByID(id string) Peer {
	if sw.peers != nil {
		return sw.peers.GetByID(id)
	}
	return nil
}

//GetConfig retrun p2p config
func (sw *Switch) GetConfig() *config.P2PConfig {
	return sw.config
}

//---------------------------------------------------------------------
// Switch setup

// AddReactor adds the given reactor to the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) AddReactor(name string, reactor Reactor) Reactor {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chID := chDesc.ID
		if sw.reactorsByCh[chID] != nil {
			cmn.PanicSanity(fmt.Sprintf("Channel %X has multiple reactors %v & %v", chID, sw.reactorsByCh[chID], reactor))
		}
		sw.chDescs = append(sw.chDescs, chDesc)
		sw.reactorsByCh[chID] = reactor
	}
	sw.reactors[name] = reactor
	return reactor
}

// Reactors returns a map of reactors registered on the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactors() map[string]Reactor {
	return sw.reactors
}

// Reactor returns the reactor with the given name.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactor(name string) Reactor {
	return sw.reactors[name]
}

// AddListener adds the given listener to the switch for listening to incoming peer connections.
// NOTE: Not goroutine safe.
func (sw *Switch) AddListener(l Listener) {
	sw.listeners = append(sw.listeners, l)
}

// Listeners returns the list of listeners the switch listens on.
// NOTE: Not goroutine safe.
func (sw *Switch) Listeners() []Listener {
	return sw.listeners
}

// IsListening returns true if the switch has at least one listener.
// NOTE: Not goroutine safe.
func (sw *Switch) IsListening() bool {
	return len(sw.listeners) > 0
}

// SetNodeInfo sets the switch's NodeInfo for checking compatibility and handshaking with other nodes.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodeInfo(nodeInfo NodeInfo) {
	sw.localNodeInfo = nodeInfo
	sw.localNodeInfo.PubKey = sw.nodeKey.PubKey().(crypto.PubKeyEd25519)
	sw.peers.AddOurAddress(nodeInfo.ListenAddr, nodeInfo.LocalAddrs)
}

// NodeInfo returns the switch's NodeInfo.
// NOTE: Not goroutine safe.
func (sw *Switch) NodeInfo() NodeInfo {
	return sw.localNodeInfo
}

// GetNumPeersByRole return the num of validators、peers、listeners
func (sw *Switch) GetNumPeersByRole() (int, int, int) {
	var validators, peers, listeners = 0, 0, 0
	swPeers := sw.peers.List()
	for _, peer := range swPeers {
		nodeType := peer.NodeInfo().Type
		if nodeType == types.NodeValidator {
			validators++
		} else if nodeType == types.NodePeer {
			peers++
		} else {
			// WARN: Wrong nodeType
		}
	}
	return validators, peers, listeners
}

//---------------------------------------------------------------------
// Service start/stop

// OnStart implements BaseService. It starts all the reactors, peers, and listeners.
func (sw *Switch) OnStart() error {
	sw.Logger.Info("Switch OnStart", "local pubkey", sw.localNodeInfo.PubKey)
	// Start reactors
	for _, reactor := range sw.reactors {
		err := reactor.Start()
		if err != nil {
			return cmn.ErrorWrap(err, "failed to start %v", reactor)
		}
	}
	// Start listeners
	for _, listener := range sw.listeners {
		go sw.listenerRoutine(listener)
	}
	if sw.ntab != nil {
		sw.ntab.Start()
	}
	if sw.manager != nil {
		sw.manager.Start()
	}
	return nil
}

// OnStop implements BaseService. It stops all listeners, peers, and reactors.
func (sw *Switch) OnStop() {
	// Stop listeners
	for _, listener := range sw.listeners {
		listener.Stop()
	}
	sw.listeners = nil
	// Stop peers
	for _, peer := range sw.peers.List() {
		peer.Stop()
		sw.peers.Remove(peer)
	}
	// Stop reactors
	sw.Logger.Debug("Switch: Stopping reactors")
	for _, reactor := range sw.reactors {
		reactor.Stop()
	}
	if sw.ntab != nil {
		sw.ntab.Stop()
	}
	if sw.manager != nil {
		sw.manager.Stop()
	}
	if sw.dm != nil {
		sw.dm.Close()
	}
}

//---------------------------------------------------------------------
// Peers

// Broadcast runs a go routine for each attempted send, which will block trying
// to send for defaultSendTimeoutSeconds. Returns a channel which receives
// success values for each attempted send (false if times out). Channel will be
// closed once msg bytes are sent to all peers (or time out).
//
// NOTE: Broadcast uses goroutines, so order of broadcast may not be preserved.
func (sw *Switch) Broadcast(chID byte, msgBytes []byte) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	var wg sync.WaitGroup
	for _, peer := range sw.peers.List() {
		wg.Add(1)
		go func(peer Peer) {
			defer wg.Done()
			success := peer.Send(chID, msgBytes)
			successChan <- success
		}(peer)
	}
	go func() {
		wg.Wait()
		close(successChan)
	}()
	return successChan
}

// BroadcastE runs a go routine for each attempted send expept peerID,which will block trying
// to send for defaultSendTimeoutSeconds. Returns a channel which receives
// success values for each attempted send (false if times out). Channel will be
// closed once msg bytes are sent to all peers (or time out).
func (sw *Switch) BroadcastE(chID byte, peerID string, msgBytes []byte) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	var wg sync.WaitGroup
	for _, peer := range sw.peers.List() {
		if peer.ID() == peerID { //peerID already have msgBytes
			continue
		}
		wg.Add(1)
		go func(peer Peer) {
			defer wg.Done()
			success := peer.Send(chID, msgBytes)
			successChan <- success
		}(peer)
	}
	go func() {
		wg.Wait()
		close(successChan)
	}()
	return successChan
}

func (sw *Switch) CloseAllConnection() {
	// Stop peers
	for _, peer := range sw.peers.List() {
		sw.stopAndRemovePeer(peer, "network change,CloseAllConnection")
	}
}

// NumPeers returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.IsOutbound() {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

// Peers returns the set of peers that are connected to the switch.
func (sw *Switch) Peers() IPeerSet {
	return sw.peers
}

func (sw *Switch) DHTPeers() []*common.Node {
	if sw.ntab.IsDhtTable() {
		buffer := make([]*common.Node, 17*16)
		table, ok := sw.ntab.(*disc.DhtTable)
		if ok {
			n := table.ReadNodesFromKbucket(buffer)
			return buffer[:n]
		}
		return nil
	} else {
		return nil
	}
}

//LocalNodeInfo return the localnode info
func (sw *Switch) LocalNodeInfo() NodeInfo {
	return sw.localNodeInfo
}

// StopPeerForError disconnects from a peer due to external error.
// If the peer is persistent, it will attempt to reconnect.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer Peer, reason interface{}) {
	sw.Logger.Error("Stopping peer for error", "peer", peer, "err", reason)
	sw.stopAndRemovePeer(peer, reason)
}

func (sw *Switch) stopAndRemovePeer(peer Peer, reason interface{}) {
	sw.peers.Remove(peer)
	err := peer.Stop()
	if err != nil {
		return
	}
	if !peer.IsOutbound() {
		remoteIP, _ := netutil.AddrIP(peer.RemoteAddr())
		sw.subInboundCon(remoteIP)
	}
	for _, reactor := range sw.reactors {
		reactor.RemovePeer(peer, reason)
	}
}

//---------------------------------------------------------------------
// Dialing

// IsDialing returns true if the switch is currently dialing the given address.
func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

// DialPeersAsync dials a list of peers asynchronously in random order (optionally, making them persistent).
// Used to dial peers from config on startup or from unsafe-RPC (trusted sources).
func (sw *Switch) DialPeersAsync(peers []string, persistent bool) error {
	var addrs []string
	for _, r := range peers {
		if !sw.peers.HasIP(r) {
			addrs = append(addrs, r)
		}
	}

	netAddrs, errs := NewNetAddressStrings(addrs)
	// only log errors, dial correct addresses
	for _, err := range errs {
		sw.Logger.Error("Error in peer's address", "err", err)
	}
	// permute the list, dial them in random order.
	perm := sw.rng.Perm(len(netAddrs))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			j := perm[i]

			addr := netAddrs[j]
			sw.randomSleep(0)
			err := sw.DialPeerWithAddress(addr, persistent)
			if err != nil {
				switch err.(type) {
				case ErrSwitchConnectToSelf, ErrSwitchDuplicatePeerID:
					sw.Logger.Debug("Error dialing peer", "err", err)
				default:
					sw.Logger.Error("Error dialing peer", "err", err)
				}
			}
		}(i)
	}
	return nil
}

// DialPeerWithAddress dials the given peer and runs sw.addPeer if it connects and authenticates successfully.
// If `persistent == true`, the switch will always try to reconnect to this peer if the connection ever fails.
func (sw *Switch) DialPeerWithAddress(addr *NetAddress, persistent bool) error {
	sw.dialing.Set(addr.IP.String(), addr)
	defer sw.dialing.Delete(addr.IP.String())
	err := sw.addOutboundPeerWithConfig(addr, sw.config, persistent)
	return err
}

func (sw *Switch) AddDial(node *common.Node) bool {
	if node == nil {
		return false
	}
	try := &NetAddress{IP: node.IP, Port: node.TCP_Port}
	if sw.whitelist != nil && !sw.whitelist.Contains(node.IP) {
		sw.Logger.Debug("addDial", "dial ip", node.IP.String(), "is in whitelist", sw.whitelist.MarshalTOML())
		return false
	}
	if sw.blacklist != nil && sw.blacklist.Contains(node.IP) {
		sw.Logger.Debug("addDial", "dial ip", node.IP.String(), "is in blacklist", sw.blacklist.MarshalTOML())
		return false
	}
	if dialling := sw.IsDialing(try); dialling {
		sw.Logger.Trace("IsDialing", "id", node.ID.String())
		return true
	}

	connected := sw.Peers().HasIP(try.String()) || sw.Peers().HasID(common.TransNodeIDToString(node.ID))
	if connected {
		peer := sw.Peers().GetByID(hex.EncodeToString(node.ID.Bytes()))
		if peer != nil {
			if peer.IsOutbound() { //Indicates that you have actively connected, skipped this node, and tried to select another node
				return false
			}
		}
		return true //Indicates that it is active connection node
	}
	err := sw.dial(try)
	if err != nil {
		return false
	}
	return true
}

func (sw *Switch) dial(address *NetAddress) error {
	err := sw.DialPeerWithAddress(address, false)
	if err != nil {
		switch err.(type) {
		case ErrSwitchConnectToSelf, ErrSwitchDuplicatePeerID:
			sw.Logger.Debug("Error dialing peer", "err", err)
		default:
			sw.Logger.Error("Error dialing peer", "err", err)
		}
	}
	return err
}

// sleep for interval plus some random amount of ms on [0, dialRandomizerIntervalMilliseconds]
func (sw *Switch) randomSleep(interval time.Duration) {
	r := time.Duration(sw.rng.Int63n(dialRandomizerIntervalMilliseconds)) * time.Millisecond
	time.Sleep(r + interval)
}

//------------------------------------------------------------------------------------
// Connection filtering

// FilterConnByAddr returns an error if connecting to the given address is forbidden.
func (sw *Switch) FilterConnByAddr(addr net.Addr) error {
	if sw.filterConnByAddr != nil {
		return sw.filterConnByAddr(addr)
	}
	return nil
}

// SetAddrFilter sets the function for filtering connections by address.
func (sw *Switch) SetAddrFilter(f func(net.Addr) error) {
	sw.filterConnByAddr = f
}

//------------------------------------------------------------------------------------

func (srv *Switch) checkInboundConn(remoteIP net.IP) error {
	if remoteIP != nil {
		// Reject Internet peers that try too often.
		srv.inboundHistory.expire(time.Now())
		num := srv.inboundConNum(remoteIP)
		if !netutil.IsLAN(remoteIP) && num >= maxInboundNumForSingleIp {
			return fmt.Errorf("remoteIP:%v inboundConNum:%d too many connections", remoteIP.String(), num)
		}
		if !netutil.IsLAN(remoteIP) && srv.inboundHistory.contains(remoteIP.String()) {
			return fmt.Errorf("remoteIP:%v too many attempts", remoteIP.String())
		}
		srv.inboundHistory.add(remoteIP.String(), time.Now().Add(inboundThrottleTime))
	}
	return nil
}

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}
		sw.Logger.Info("listenerRoutine", "recv new con from", inConn.RemoteAddr())
		//check income connection
		remoteIP, _ := netutil.AddrIP(inConn.RemoteAddr())
		err := sw.checkInboundConn(remoteIP)
		if err != nil {
			sw.Logger.Info("Ignoring inbound connection: already have enough peers", "err", err)
			inConn.Close()
			continue
		}

		// ignore connection if we already have enough
		// leave room for MinNumOutboundPeers
		var maxInPeers int
		var OutboundPeers int
		if netutil.IsLAN(remoteIP) {
			sw.Logger.Debug("it is local network", "remoteIP", remoteIP.String())
			maxInPeers = sw.config.MaxNumPeers / 2
		} else {
			if sw.ntab != nil {
				OutboundPeers = sw.ntab.GetMaxDialOutNum()
			}
			maxInPeers = sw.config.MaxNumPeers - OutboundPeers
		}
		if maxInPeers <= (sw.peers.Size() - OutboundPeers) {
			sw.Logger.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(),
				"numPeers", sw.peers.Size(), "OutboundPeers", OutboundPeers, "maxin", maxInPeers)
			inConn.Close()
			continue
		}

		// New inbound connection!
		err = sw.addInboundPeerWithConfig(inConn, sw.config)
		if err != nil {
			sw.Logger.Info("Ignoring inbound connection: error while adding peer", "address", inConn.RemoteAddr().String(), "err", err)
			continue
		}
	}

	// cleanup
}

// closes conn if err is returned
func (sw *Switch) addInboundPeerWithConfig(
	conn net.Conn,
	config *config.P2PConfig,
) error {
	sw.Logger.Info("addInboundPeerWithConfig", "conn.addr", conn.RemoteAddr())
	peerConn, err := newInboundPeerConn(conn, config, sw.nodeKey)
	if err != nil {
		conn.Close() // peer is nil
		return err
	}
	if err = sw.addPeer(peerConn, true); err != nil {
		peerConn.CloseConn()
		return err
	}

	return nil
}

// dial the peer; make secret connection; authenticate against the dialed ID;
// add the peer.
// if dialing fails, start the reconnect loop. If handhsake fails, its over.
// If peer is started succesffuly, reconnectLoop will start when
// StopPeerForError is called
func (sw *Switch) addOutboundPeerWithConfig(
	addr *NetAddress,
	config *config.P2PConfig,
	persistent bool,
) error {
	sw.Logger.Info("Dialing peer", "address", addr)
	peerConn, err := newOutboundPeerConn(
		addr,
		config,
		persistent,
		sw.nodeKey,
	)
	if err != nil {
		return err
	}

	if err := sw.addPeer(peerConn, false); err != nil {
		sw.Logger.Info("addPeer", "failed", err)
		peerConn.CloseConn()
		return err
	}
	return nil
}

func (sw *Switch) inboundConNum(remoteIP net.IP) int {
	sw.Logger.Debug("inboundConNum", "remoteIP", remoteIP.String())
	sw.inboundLock.Lock()
	defer sw.inboundLock.Unlock()
	num, _ := sw.inboundMap[remoteIP.String()]
	return num
}

func (sw *Switch) addInboundCon(remoteIP net.IP) {
	sw.Logger.Info("addInboundCon", "remoteIP", remoteIP.String())
	sw.inboundLock.Lock()
	defer sw.inboundLock.Unlock()
	num, _ := sw.inboundMap[remoteIP.String()]
	num++
	sw.inboundMap[remoteIP.String()] = num
}

func (sw *Switch) subInboundCon(remoteIP net.IP) {
	sw.Logger.Info("subInboundCon", "remoteIP", remoteIP.String())
	sw.inboundLock.Lock()
	defer sw.inboundLock.Unlock()
	num, _ := sw.inboundMap[remoteIP.String()]
	num--
	if num < 0 {
		log.Info("subInboundCon unexpect", "ip", remoteIP.String(), "num", num)
		return
	}
	sw.inboundMap[remoteIP.String()] = num
}

// addPeer performs the P2P handshake with a peer
// that already has a SecretConnection. If all goes well,
// it starts the peer and adds it to the switch.
// NOTE: This performs a blocking handshake before the peer is added.
// NOTE: If error is returned, caller is responsible for calling
// peer.CloseConn()
func (sw *Switch) addPeer(pc peerConn, isInCon bool) error {
	addr := pc.conn.RemoteAddr()
	if err := sw.FilterConnByAddr(addr); err != nil {
		return err
	}

	// Exchange NodeInfo on the conn
	peerNodeInfo, err := HandShakeFunc(pc.conn, sw.localNodeInfo, time.Duration(sw.config.HandshakeTimeout), isInCon)
	if err != nil {
		return err
	}
	//drop peer record in blacklist
	if sw.blackListHasID(peerNodeInfo.ID()) {
		return fmt.Errorf("peer id:%v is in blacklist", peerNodeInfo.ID())
	}
	// Validate the peers nodeInfo
	if err := peerNodeInfo.Validate(); err != nil {
		return err
	}

	// Avoid self
	if sw.localNodeInfo.PubKey.Equals(peerNodeInfo.PubKey) {
		return ErrSwitchConnectToSelf{peerNodeInfo.NetAddress()}
	}

	// Avoid duplicate
	if sw.peers.HasID(peerNodeInfo.ID()) {
		sw.peers.connIPs.Store(addr.String(), peerNodeInfo.ID())
		return ErrSwitchDuplicatePeerID{peerNodeInfo.ID()}
	}

	// Check version, chain id
	if err := sw.localNodeInfo.CompatibleWith(peerNodeInfo); err != nil {
		return err
	}
	peerNodeInfo.CachePeerID = "" //reset CachePeerID
	peer := newPeer(pc, sw.mConfig, &peerNodeInfo, sw.reactorsByCh, sw.chDescs, sw.StopPeerForError)
	peer.SetLogger(sw.Logger.With("peer", addr))

	peer.Logger.Info("Successful handshake with peer", "peerNodeInfo", peerNodeInfo)

	// Add the peer to .peers.
	// It should not err since we already checked peers.Has().
	if err := sw.peers.Add(peer); err != nil {
		return err
	}
	if isInCon {
		remoteIP, _ := netutil.AddrIP(pc.conn.RemoteAddr())
		sw.addInboundCon(remoteIP)
	}

	// All good. Start peer
	isrunning := sw.IsRunning()
	if isrunning {
		if err = sw.startInitPeer(peer); err != nil {
			return err
		}
	}

	sw.Logger.Info("Added peer success", "peer", peer, "isrunning", isrunning)
	return nil
}

func (sw *Switch) startInitPeer(peer *peer) error {
	err := peer.Start() // spawn send/recv routines
	if err != nil {
		// Should never happen
		sw.Logger.Error("Error starting peer", "peer", peer, "err", err)
		return err
	}

	for _, reactor := range sw.reactors {
		reactor.AddPeer(peer)
	}

	return nil
}
