package discover

import (
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"time"

	"bytes"

	"encoding/binary"

	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Keys in the node database.
const (
	// These fields are stored per ID and IP, the full key is "n:<ID>:v4:<IP>:findfail".
	// Use nodeItemKey to create those keys.
	dbNodePrefix    = "n:" // Identifier to prefix node entries with
	dbDiscoverRoot  = "v4"
	dbNodeFindFails = "findfail"
	dbNodePing      = "lastping"
	dbNodePong      = "lastpong"
)

const (
	dbNodeExpiration = 24 * time.Hour // Time after which an unseen node should be dropped.
	dbCleanupCycle   = time.Hour      // Time period for running the expiration task.
)

type dbManager struct {
	logger    log.Logger
	db        dbm.DB
	runner    sync.Once
	quit      chan struct{}
	closeOnce sync.Once
}

// nodeItemKey returns the database key for a node metadata field.
func nodeItemKey(id common.NodeID, ip net.IP, field string) []byte {
	ip16 := ip.To16()
	if ip16 == nil {
		panic(fmt.Errorf("invalid IP (length %d)", len(ip)))
	}
	return bytes.Join([][]byte{nodeKey(id), ip16, []byte(field)}, []byte{':'})
}

// nodeKey returns the database key for a node record.
func nodeKey(id common.NodeID) []byte {
	key := append([]byte(dbNodePrefix), id[:]...)
	key = append(key, ':')
	key = append(key, dbDiscoverRoot...)
	return key
}

func NewDBManager(rawDB dbm.DB, logger log.Logger) *dbManager {
	p2pDB := &dbManager{db: rawDB, logger: logger, quit: make(chan struct{})}
	return p2pDB
}

func mustDecodeNode(id, data []byte) *common.Node {
	node := new(common.Node)
	if err := ser.DecodeBytes(data, node); err != nil {
		panic(fmt.Errorf("p2p/enode: can't decode node %x in DB: %v", id, err))
	}
	// Restore node id cache.
	copy(node.ID[:], id)
	return node
}

// reads the next node record from the iterator, skipping over other
// database entries and pong timeout node.
func (dm *dbManager) findBestNode(it dbm.Iterator, nodes []*common.Node, now time.Time, maxAge time.Duration) *common.Node {
seek:
	for ; it.Valid(); it.Next() {
		id, rest := splitNodeKey(it.Key())
		if string(rest) != dbDiscoverRoot {
			continue
		}
		node := mustDecodeNode(id[:], it.Value())
		subTime := now.Sub(dm.LastPongReceived(node.ID, node.IP))
		if subTime > maxAge {
			continue
		}
		for i := range nodes {
			if nodes[i].ID == node.ID {
				continue seek // duplicate
			}
		}
		return node
	}
	return nil
}

func (dm *dbManager) QuerySeeds(n int, maxAge time.Duration) []*common.Node {
	var (
		now   = time.Now()
		nodes = make([]*common.Node, 0, n)

		it = dm.db.Iterator(nil, nil)
		id common.NodeID
	)
	defer it.Close()
	//Ensure that the initial id is random
	rand.Read(id[:])
	for seeks := 0; len(nodes) < n && seeks < n*5; seeks++ {
		// Seek to a random entry. The first byte is incremented by a
		// random amount each time in order to increase the likelihood
		// of hitting all existing nodes in very small databases.
		ctr := id[0]
		rand.Read(id[:])
		id[0] = ctr + id[0]%16
		it.Seek(nodeKey(id))
		//var node *common.Node
		node := dm.findBestNode(it, nodes, now, maxAge)
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (dm *dbManager) printfIt() {
	var (
		it = dm.db.Iterator(nil, nil)
	)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		id, rest := splitNodeKey(it.Key())
		if string(rest) != dbDiscoverRoot {
			dm.logger.Debug("continue node", "id", id, "rest", rest)
			continue
		}
		node := mustDecodeNode(id[:], it.Value())
		dm.logger.Debug("node", "id", node.ID)
	}
}

func (dm *dbManager) LastPingReceived(id common.NodeID, ip net.IP) time.Time {
	return time.Unix(dm.fetchInt64(nodeItemKey(id, ip, dbNodePing)), 0)
}

func (dm *dbManager) ensureExpirer() {
	dm.runner.Do(func() { go dm.expirer() })
}

// expirer should be started in a go routine, and is responsible for looping ad
// infinitum and dropping stale data from the database.
func (dm *dbManager) expirer() {
	tick := time.NewTicker(dbCleanupCycle)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			dm.expireNodes()
		case <-dm.quit:
			return
		}
	}
}

// expireNodes iterates over the database and deletes all nodes that have not
// been seen (i.e. received a pong from) for some time.
func (dm *dbManager) expireNodes() {
	prefix := []byte(dbNodePrefix)
	r := util.BytesPrefix(prefix)
	it := dm.db.Iterator(r.Start, r.Limit)
	defer it.Close()
	if !it.Next() {
		return
	}

	var (
		threshold    = time.Now().Add(-dbNodeExpiration).Unix()
		youngestPong int64
		atEnd        = false
	)
	for !atEnd || it.Valid() {
		id, ip, field := splitNodeItemKey(it.Key())
		if field == dbNodePong {
			time, _ := binary.Varint(it.Value())
			if time > youngestPong {
				youngestPong = time
			}
			if time < threshold {
				// Last pong from this IP older than threshold, remove fields belonging to it.
				dm.logger.Info("expireNodes", "id", id, "ip", ip)
				deleteRange(dm.db, nodeItemKey(id, ip, ""))
			}
		}
		atEnd = !it.Next()
		if it.Valid() {
			nextID, _ := splitNodeKey(it.Key())
			if atEnd || nextID != id {
				// We've moved beyond the last entry of the current ID.
				// Remove everything if there was no recent enough pong.
				if youngestPong > 0 && youngestPong < threshold {
					dm.logger.Info("expireNodes", "atEnd", atEnd, "nextID", nextID, "id", id)
					deleteRange(dm.db, nodeKey(id))
				}
				youngestPong = 0
			}
		}
	}
}

func deleteRange(db dbm.DB, prefix []byte) {
	tmpPrefix := []byte(prefix)
	r := util.BytesPrefix(tmpPrefix)
	itr := db.Iterator(r.Start, r.Limit)
	defer itr.Close()
	for itr.Next() {
		db.Delete(itr.Key())
	}
}

// splitNodeItemKey returns the components of a key created by nodeItemKey.
func splitNodeItemKey(key []byte) (id common.NodeID, ip net.IP, field string) {
	id, key = splitNodeKey(key)
	// Skip discover root.
	if string(key) == dbDiscoverRoot {
		return id, nil, ""
	}
	key = key[len(dbDiscoverRoot)+1:]
	// Split out the IP.
	ip = net.IP(key[:16])
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	key = key[16+1:]
	// Field is the remainder of key.
	field = string(key)
	return id, ip, field
}

// splitNodeKey returns the node ID of a key created by nodeKey.
func splitNodeKey(key []byte) (id common.NodeID, rest []byte) {
	if !bytes.HasPrefix(key, []byte(dbNodePrefix)) {
		return common.NodeID{}, nil
	}
	item := key[len(dbNodePrefix):]
	copy(id[:], item[:len(id)])
	return id, item[len(id)+1:]
}

func (dm *dbManager) UpdateNode(node *common.Node) {
	blob, err := ser.EncodeToBytes(node)
	if err != nil {
		dm.logger.Error("UpdateNode", "EncodeToBytes err", err)
		return
	}
	dm.db.Set(nodeKey(node.ID), blob)
	return
}

func (dm *dbManager) storeInt64(key []byte, n int64) {
	blob := make([]byte, binary.MaxVarintLen64)
	blob = blob[:binary.PutVarint(blob, n)]
	dm.db.Set(key, blob)
}

func (dm *dbManager) UpdateLastPingReceived(id common.NodeID, ip net.IP, instance time.Time) {
	dm.storeInt64(nodeItemKey(id, ip, dbNodePing), instance.Unix())
}

func (dm *dbManager) UpdateLastPongReceived(id common.NodeID, ip net.IP, instance time.Time) {
	dm.storeInt64(nodeItemKey(id, ip, dbNodePong), instance.Unix())
}

func (dm *dbManager) UpdateFindFails(id common.NodeID, ip net.IP, fails int) {
	dm.storeInt64(nodeItemKey(id, ip, dbNodeFindFails), int64(fails))
}

func (dm *dbManager) LastPongReceived(id common.NodeID, ip net.IP) time.Time {
	// Launch expirer
	dm.ensureExpirer()
	return time.Unix(dm.fetchInt64(nodeItemKey(id, ip, dbNodePong)), 0)
}

// fetchInt64 retrieves an integer associated with a particular key.
func (dm *dbManager) fetchInt64(key []byte) int64 {
	blob := dm.db.Get(key)
	if len(blob) == 0 {
		return 0
	}
	val, read := binary.Varint(blob)
	if read <= 0 {
		return 0
	}
	return val
}

func (dm *dbManager) FindFails(id common.NodeID, ip net.IP) int {
	return int(dm.fetchInt64(nodeItemKey(id, ip, dbNodeFindFails)))
}

func (dm *dbManager) Close() {
	dm.closeOnce.Do(func() {
		if dm.quit != nil {
			close(dm.quit)
		}
	})
}
