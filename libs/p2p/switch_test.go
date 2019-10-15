package p2p

import (
	"testing"

	"time"

	"github.com/lianxiangcloud/linkchain/config"
	cmn "github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/crypto"
	dbm "github.com/lianxiangcloud/linkchain/libs/db"
	"github.com/lianxiangcloud/linkchain/libs/log"
	"github.com/lianxiangcloud/linkchain/libs/p2p/common"
	"github.com/lianxiangcloud/linkchain/types"
	"github.com/lianxiangcloud/linkchain/version"
)

var (
	logger = log.Root()
)

func init() {
	logger.SetHandler(log.StdoutHandler)
}

func makeNodeInfo(chainID string, nodeType types.NodeType, moniker string, httpEndpoint string) NodeInfo {
	nodeInfo := NodeInfo{
		Network: chainID,
		Version: version.Version,
		Channels: []byte{
			byte(0x40),
		},
		Moniker: moniker,
		Other: []string{
			cmn.Fmt("p2p_version=%v", Version),
			cmn.Fmt("consensus_version=%v", "0.1.0"),
		},
		Type: nodeType,
	}

	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", httpEndpoint))
	return nodeInfo
}

func newTestDB() dbm.DB {
	return dbm.NewMemDB()
}

func TestSwitch(t *testing.T) {
	cfg := config.DefaultP2PConfig()
	cfg.ListenAddress = ""
	localNodeInfo := makeNodeInfo("chainID", types.NodeValidator, "test", "")
	var seeds []*common.Node
	db := newTestDB()

	privKey := crypto.GenPrivKeyEd25519()

	p2pmanager, err := NewP2pManager(logger, privKey, cfg, localNodeInfo, seeds, db)
	if err != nil {
		t.Fatal("NewP2pManager err", err)
	}
	p2pmanager.Start()
	time.Sleep(time.Second * 5)
	p2pmanager.Stop()

	/*for {
		time.Sleep(time.Second * 2)
	}*/
}
