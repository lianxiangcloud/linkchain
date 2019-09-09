package discover

import (
	mrand "math/rand"

	crand "crypto/rand"

	"github.com/lianxiangcloud/linkchain/bootnode"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/types"

	"encoding/binary"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

//HTTPTable only include seeds node
type HTTPTable struct {
	priv     crypto.PrivKey
	logger   log.Logger
	bootSvr  string         //addr of bootnode server
	nodeType types.NodeType //NodeValidator
	seeds    []*node        // bootstrap nodes
	seedsNum int
	rand     *mrand.Rand // source of randomness, periodically reseeded
}

// NewHTTPTable starts get seeds from bootnode server.
func NewHTTPTable(cfg common.Config, bootSvr string, nodeType types.NodeType, log log.Logger) (*HTTPTable, error) {
	log.Info("NewHttpTable", "bootSvr", bootSvr, "nodeType", nodeType)
	table := &HTTPTable{
		priv:     cfg.PrivateKey,
		logger:   log,
		bootSvr:  bootSvr,
		nodeType: nodeType,
		rand:     mrand.New(mrand.NewSource(0)),
	}
	if err := table.setFallbackNodes(cfg.SeedNodes); err != nil {
		return nil, err
	}
	table.seedRand()
	return table, nil
}

func (tab *HTTPTable) setFallbackNodes(nodes []*common.Node) error {
	var splitedNodes []*common.Node
	seedsNum := 0
	seedsMap := make(map[string]bool)
	myID := common.TransPubKeyToNodeID(tab.priv.PubKey())
	for _, n := range nodes { //Get the number of real seed nodes according to ID
		tab.logger.Info("HTTPTable setFallbackNodes", "id", n.ID.String(), "ip", n.IP.String(), "tcpPort", n.TCP_Port)
		if n.ID == myID { //it is my self,skip
			tab.logger.Debug("it is my self", "n.ID", n.ID.String())
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
	tab.seeds = wrapNodes(splitedNodes)
	tab.seedsNum = seedsNum
	tab.logger.Info("HTTPTable setFallbackNodes", "seedsNum", seedsNum)
	return nil
}

func (tab *HTTPTable) seedRand() {
	var b [8]byte
	crand.Read(b[:])
	tab.rand.Seed(int64(binary.BigEndian.Uint64(b[:])))
}

func (tab *HTTPTable) Start() {}
func (tab *HTTPTable) Stop()  {}

func (tab *HTTPTable) IsDhtTable() bool {
	return false
}

//LookupRandom get seeds from bootnode server
func (tab *HTTPTable) LookupRandom() []*common.Node {
	seedNodes, _, _ := bootnode.GetSeeds(tab.bootSvr, tab.priv, tab.logger)
	var splitedNodes []*common.Node
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
		tab.logger.Debug("LookupRandom", "tab.seeds", tab.seeds)
	}
	return splitedNodes
}

//ReadRandomNodes get rand seeds from local cache
func (tab *HTTPTable) ReadRandomNodes(buf []*common.Node) (nodeNum int) {
	var i = 0
	tab.logger.Trace("ReadRandomNodes", "tab.seeds", tab.seeds)
	for ; i < len(buf) && i < len(tab.seeds); i++ {
		buf[i] = unwrapNode(tab.seeds[i])
	}
	nodeNum = i
	// Shuffle the buckets.
	for i := nodeNum - 1; i > 0; i-- {
		j := tab.rand.Intn(nodeNum)
		buf[i], buf[j] = buf[j], buf[i]
	}
	return nodeNum
}

//GetMaxDialOutNum return the max dialout num
func (tab *HTTPTable) GetMaxDialOutNum() int {
	if tab.seedsNum > 0 {
		return tab.seedsNum
	}
	return defaultSeeds
}

//GetMaxConNumFromCache return the max node's num from local cache
func (tab *HTTPTable) GetMaxConNumFromCache() int {
	return len(tab.seeds)
}
