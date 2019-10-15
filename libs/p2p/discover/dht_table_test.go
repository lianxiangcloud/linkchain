package discover

import (
	"fmt"
	"net"
	"testing"

	"time"

	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/stretchr/testify/assert"
)

var seeds = []*common.Node{
	&common.Node{IP: net.ParseIP("127.0.0.1"), UDP_Port: 2500, ID: common.NodeID{1, 2, 3}},
	&common.Node{IP: net.ParseIP("127.0.0.1"), UDP_Port: 2501, TCP_Port: 2501, ID: common.NodeID{1, 2, 4}},
	&common.Node{IP: net.ParseIP("127.0.0.1"), UDP_Port: 2502, TCP_Port: 2502, ID: common.NodeID{1, 2, 5}},
	&common.Node{IP: net.ParseIP("127.0.0.1"), UDP_Port: 2503, TCP_Port: 2503, ID: common.NodeID{1, 2, 6}},
	&common.Node{IP: net.ParseIP("127.0.0.1"), UDP_Port: 2504, TCP_Port: 2504, ID: common.NodeID{1, 2, 7}},
}

func defaultDBProvider(id string, dbtype dbm.DBBackendType, dbdir string, counts uint64) (dbm.DB, error) {
	return dbm.NewDB(id, dbtype, dbdir, counts), nil
}

func generateSelfInfo(myPrivKey crypto.PrivKey) *SlefInfo {
	var startPort = 25000
	myID := common.NodeID(crypto.Keccak256Hash(myPrivKey.PubKey().Bytes()))

	for bindPort := startPort; bindPort < 65535; bindPort++ {
		myNode := &common.Node{ID: myID, UDP_Port: uint16(bindPort), TCP_Port: uint16(bindPort)}
		bindAddr := fmt.Sprintf("%s:%d", "127.0.0.1", bindPort)
		udpAddr, err := net.ResolveUDPAddr("udp", bindAddr)
		if err != nil {
			logger.Error("NewDefaultListener", "ResolveUDPAddr err", err, "bindAddr", bindAddr)
			continue
		}
		udpConn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			logger.Error("NewDefaultListener", "ListenUDP err", err, "addr", udpAddr)
			continue
		}
		selfinfo := &SlefInfo{
			Self:      myNode,
			ListenCon: udpConn,
		}
		return selfinfo
	}
	return nil
}

func TestDHTReadRandomNodes(t *testing.T) {
	var cfg common.Config
	db, err := defaultDBProvider("dht", dbm.LevelDBBackend, "/tmp/dhtdb0", 0)
	if err != nil {
		t.Fatalf("DefaultDBProvider failed,err:%s", err)
		return
	}
	db2 := NewDBManager(db, logger)
	cfg.PrivateKey = crypto.GenPrivKeyEd25519()
	cfg.SeedNodes = seeds
	self := generateSelfInfo(cfg.PrivateKey)
	if self == nil {
		t.Fatalf("generateSelfInfo self==nil")
		return
	}
	ntab, err := NewDhtTable(10, self, db2, cfg, logger)
	if err != nil {
		t.Fatalf("NewDhtTable err:%s", err)
		return
	}
	cacheSize := 3
	randomNodesFromCache := make([]*common.Node, cacheSize)
	num := ntab.ReadRandomNodes(randomNodesFromCache, nil)
	assert.Equal(t, cacheSize, num)
	//
	cacheSize = 7
	randomNodesFromCache = make([]*common.Node, cacheSize)
	num = ntab.ReadRandomNodes(randomNodesFromCache, nil)
	assert.Equal(t, len(seeds), num)
	//
	alreadyConnect := make(map[string]bool)
	alreadyConnect[common.TransNodeIDToString(common.NodeID{1, 2, 4})] = true
	alreadyConnect[common.TransNodeIDToString(common.NodeID{1, 2, 5})] = true
	num = ntab.ReadRandomNodes(randomNodesFromCache, alreadyConnect)
	assert.Equal(t, len(seeds)-2, num)
}

func TestDHTPing(t *testing.T) {
	var cfg common.Config
	db, err := defaultDBProvider("dht", dbm.LevelDBBackend, "/tmp/dhtdb2", 0)
	if err != nil {
		t.Fatalf("DefaultDBProvider failed,err:%s", err)
		return
	}
	db2 := NewDBManager(db, logger)
	cfg.PrivateKey = crypto.GenPrivKeyEd25519()
	self := generateSelfInfo(cfg.PrivateKey)
	if self == nil {
		t.Fatalf("generateSelfInfo self==nil")
		return
	}
	ntab, err := NewDhtTable(10, self, db2, cfg, logger)
	if err != nil {
		t.Fatalf("NewDhtTable err:%s", err)
		return
	}
	ntab.Start()
	time.Sleep(time.Second * 1)
	testNode := &common.Node{IP: net.ParseIP("192.168.3.59"), UDP_Port: 120, TCP_Port: 120, ID: common.NodeID{1, 2, 3}}
	_, err = ntab.ping(testNode)
	fmt.Printf("ping1 err:%s\n", err)
	time.Sleep(time.Second * 1)
	var flag = false
	if err != nil {
		fmt.Printf("ping err:%s\n", err)
		flag = true
	}
	assert.Equal(t, true, flag)
}
