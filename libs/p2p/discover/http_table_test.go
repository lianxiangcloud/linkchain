package discover

import (
	"net"
	"testing"

	"os"

	"encoding/json"

	"github.com/lianxiangcloud/linkchain/bootnode"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/stretchr/testify/assert"
)

var (
	logger = log.Root()
)

func init() {
	logger.SetHandler(log.StdoutHandler)
}

type addr struct {
	Network string `json:"network"` //tcp or udp
	Addr    string `json:"addr"`
}

func savevalSeedsToFile(privKeys []crypto.PrivKey, valSeeds []*common.Node, valSeedsFiles string, t *testing.T) {
	// Create a file
	var f *os.File
	f, err := os.OpenFile(valSeedsFiles, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		t.Fatalf("creating test file failed: %s", err)
	}
	var jsonData bootnode.GetSeedsResp
	jsonData.Seeds = make([]bootnode.Rnode, len(valSeeds))
	for i := 0; i < len(jsonData.Seeds); i++ {
		jsonData.Seeds[i].ID = common.TransPubKeyToNodeID(privKeys[i].PubKey())
		tmpEnd := &bootnode.Endpoint{IP: []string{valSeeds[i].IP.String()}}
		tmpEnd.Port = make(map[string]int)
		tmpEnd.Port["tcp"] = int(valSeeds[i].TCP_Port)
		jsonData.Seeds[i].Endpoint = tmpEnd
	}
	encodeData, err := json.Marshal(&jsonData)
	if err != nil {
		t.Fatalf("Marshal failed: %s", err)
		return
	}
	_, err = f.Write(encodeData)
	if err != nil {
		t.Fatalf("Write failed: %s", err)
		return
	}

}
func generateVals(valsNum int) ([]crypto.PrivKey, []*common.Node) {
	privKeys := make([]crypto.PrivKey, valsNum)
	validators := make([]*common.Node, valsNum)
	startPort := 8000
	for i := 0; i < len(privKeys); i++ {
		privKeys[i] = crypto.GenPrivKeyEd25519()
		validators[i] = &common.Node{IP: net.ParseIP("127.0.0.1"), TCP_Port: uint16(startPort + i), ID: common.NodeID(crypto.Keccak256Hash(privKeys[i].PubKey().Bytes()))}
	}
	return privKeys, validators
}

func testValidator(t *testing.T, valSeedsFiles string, valsNum int, privKey crypto.PrivKey, validators []*common.Node) {
	cfg := common.Config{PrivateKey: privKey, SeedNodes: validators}
	table, err := NewHTTPTable(cfg, valSeedsFiles, logger)
	if err != nil {
		t.Fatalf("NewHTTPTable failed: %s", err)
		return
	}
	table.Start()

	n := table.GetMaxDialOutNum()
	assert.Equal(t, valsNum-1, n)
	n = table.GetMaxConNumFromCache()
	assert.Equal(t, valsNum-1, n)
	nodes := table.LookupRandom()
	assert.Equal(t, valsNum-1, len(nodes))
	tmpNodes := make([]*common.Node, 2)
	n = table.ReadRandomNodes(tmpNodes, nil)
	if len(tmpNodes) < valsNum {
		assert.Equal(t, len(tmpNodes), n)
	} else {
		assert.Equal(t, valsNum, n)
	}
	table.Stop()
}

func testCommon(nodeType types.NodeType, t *testing.T, valSeedsFiles string, valsNum int, privKey crypto.PrivKey, validators []*common.Node) {
	cfg := common.Config{PrivateKey: privKey, SeedNodes: validators}
	table, err := NewHTTPTable(cfg, valSeedsFiles, logger)
	if err != nil {
		t.Fatalf("NewHTTPTable failed: %s", err)
		return
	}
	table.Start()

	n := table.GetMaxDialOutNum()
	assert.Equal(t, valsNum, n)
	n = table.GetMaxConNumFromCache()
	assert.Equal(t, valsNum, n)
	tmpNodes := make([]*common.Node, 2)
	n = table.ReadRandomNodes(tmpNodes, nil)
	if len(tmpNodes) < valsNum {
		assert.Equal(t, len(tmpNodes), n)
	} else {
		assert.Equal(t, valsNum, n)
	}
	nodes := table.LookupRandom()
	if valSeedsFiles == "" {
		assert.Equal(t, 0, len(nodes))
	} else {
		assert.Equal(t, valsNum, len(nodes))
	}
	table.Stop()
}

func testOutValidator(t *testing.T, valSeedsFiles string, valsNum int, privKey crypto.PrivKey, validators []*common.Node) {
	testCommon(types.NodeValidator, t, valSeedsFiles, valsNum, privKey, validators)
}

func TestHttpTable(t *testing.T) {
	//init validators
	valsNum := 4
	privKeys, validators := generateVals(valsNum)

	//bootsvr is local json file
	valSeedsFiles := "/tmp/seeds.json"
	//validator
	savevalSeedsToFile(privKeys, validators, valSeedsFiles, t)
	testValidator(t, valSeedsFiles, valsNum, privKeys[0], validators)
}

func TestHTTPReadRandomNodes(t *testing.T) {
	var cfg common.Config
	cfg.PrivateKey = crypto.GenPrivKeyEd25519()
	cfg.SeedNodes = seeds
	self := generateSelfInfo(cfg.PrivateKey)
	if self == nil {
		t.Fatalf("generateSelfInfo self==nil")
		return
	}
	ntab, err := NewHTTPTable(cfg, "", logger)
	if err != nil {
		t.Fatalf("NewHTTPTable failed: %s", err)
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
