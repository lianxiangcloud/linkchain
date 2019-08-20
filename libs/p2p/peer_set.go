package p2p

import (
	"sync"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

// IPeerSet has a (immutable) subset of the methods of PeerSet.
type IPeerSet interface {
	HasID(id string) bool
	HasIP(ip string) bool
	GetByID(id string) Peer
	GetByIP(ip string) Peer
	List() []Peer
	Size() int
}

//-----------------------------------------------------------------------------

// PeerSet is a special structure for keeping a table of peers.
// Iteration over the peers is super fast and thread-safe.
type PeerSet struct {
	mtx     sync.Mutex
	lookup  map[string]*peerSetItem
	connIPs sync.Map //map[string]string   //key:ip:port  value:id
	list    []Peer
}

type peerSetItem struct {
	peer  *peer
	index int
}

// NewPeerSet creates a new peerSet with a list of initial capacity of 256 items.
func NewPeerSet() *PeerSet {
	return &PeerSet{
		lookup: make(map[string]*peerSetItem),
		list:   make([]Peer, 0, 256),
	}
}

// Add adds the peer to the PeerSet.
// It returns an error carrying the reason, if the peer is already present.
func (ps *PeerSet) Add(peer *peer) error {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	id := peer.ID()
	if ps.lookup[id] != nil {
		return ErrSwitchDuplicatePeerID{id}
	}

	index := len(ps.list)
	// Appending is safe even with other goroutines
	// iterating over the ps.list slice.
	ps.list = append(ps.list, peer)
	ps.lookup[id] = &peerSetItem{peer, index}

	nodeinfo := peer.NodeInfo()
	if nodeinfo.ListenAddr != "" {
		//TODO: fix me here, this will add 127.0.0.1 into this
		ps.connIPs.Store(nodeinfo.ListenAddr, id)
	}
	log.Info("Add peer", "id", peer.ID(), "peer", nodeinfo)
	for _, ip := range nodeinfo.LocalAddrs {
		ps.connIPs.Store(ip, id)
	}

	return nil
}

// AddOurAddress add oneself node address.
func (ps *PeerSet) AddOurAddress(listenaddr string, otheraddr []string) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	log.Info("AddOurAddress", "listenerAddr", listenaddr, "otherAddr", otheraddr)

	if listenaddr != "" {
		ps.connIPs.Store(listenaddr, listenaddr)
	}
	for _, ip := range otheraddr {
		ps.connIPs.Store(ip, ip)
	}

	return
}

// HasID returns true if the set contains the peer referred to by this
// id, otherwise false.
func (ps *PeerSet) HasID(id string) bool {
	ps.mtx.Lock()
	_, ok := ps.lookup[id]
	ps.mtx.Unlock()
	return ok
}

// HasIP returns true if the set contains the peer referred to by this
// ip, otherwise false.
func (ps *PeerSet) HasIP(ip string) bool {
	ps.mtx.Lock()
	_, ok := ps.connIPs.Load(ip)
	ps.mtx.Unlock()
	return ok
}

// GetByID looks up a peer by the provided id. Returns nil if peer is not found.
func (ps *PeerSet) GetByID(id string) Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	item, ok := ps.lookup[id]
	if ok {
		return item.peer
	}
	return nil
}

// GetByIP looks up a peer by the provided ip. Returns nil if peer is not found.
func (ps *PeerSet) GetByIP(ip string) Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	id, ok := ps.connIPs.Load(ip)
	if !ok {
		return nil
	}

	item, ok := ps.lookup[id.(string)]
	if ok {
		return item.peer
	}
	return nil
}

// Remove discards peer by its Key, if the peer was previously memoized.
func (ps *PeerSet) Remove(peer Peer) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	item := ps.lookup[peer.ID()]
	if item == nil {
		return
	}

	index := item.index
	// Create a new copy of the list but with one less item.
	// (we must copy because we'll be mutating the list).
	newList := make([]Peer, len(ps.list)-1)
	copy(newList, ps.list)

	if index != len(ps.list)-1 {
		// Replace the popped item with the last item in the old list.
		lastPeer := ps.list[len(ps.list)-1]
		lastPeerKey := lastPeer.ID()
		lastPeerItem := ps.lookup[lastPeerKey]
		newList[index] = lastPeer
		lastPeerItem.index = index
	}

	ps.list = newList
	delete(ps.lookup, peer.ID())

	nodeinfo := peer.NodeInfo()
	if nodeinfo.ListenAddr != "" {
		val, ok := ps.connIPs.Load(nodeinfo.ListenAddr)
		if ok && val.(string) == peer.ID() {
			ps.connIPs.Delete(nodeinfo.ListenAddr)
		}
	}
	for _, ip := range nodeinfo.LocalAddrs {
		val, ok := ps.connIPs.Load(ip)
		if ok && val.(string) == peer.ID() {
			ps.connIPs.Delete(ip)
		}
	}
	ips := make([]string, 0)
	ps.connIPs.Range(func(ip interface{}, id interface{}) bool {
		if id.(string) == peer.ID() {
			ips = append(ips, ip.(string))
			return true
		}
		return true
	})
	for _, ip := range ips {
		ps.connIPs.Delete(ip)
	}
}

// Size returns the number of unique items in the peerSet.
func (ps *PeerSet) Size() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return len(ps.list)
}

// List returns the threadsafe list of peers.
func (ps *PeerSet) List() []Peer {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.list
}
