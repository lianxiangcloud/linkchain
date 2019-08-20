package daemon

import (
	"fmt"
	"testing"

	cfg "github.com/lianxiangcloud/linkchain/wallet/config"
)

func init() {
	config := cfg.DefaultConfig()
	config.Daemon.DaemonHost = "127.0.0.1"
	config.Daemon.DaemonPort = 18081

	InitDaemonClient(config.Daemon)
}

type gethashes struct {
	BlockIDs    []string `json:"block_ids"`
	StartHeight uint64   `json:"start_height"`
}
type getblocks struct {
	BlockIDs    []string `json:"block_ids"`
	StartHeight uint64   `json:"start_height"`
	Prune       bool     `json:"prune"`
	NoMinerTx   bool     `json:"no_miner_tx"`
}

func getTestData() (t map[string]interface{}) {
	t = make(map[string]interface{})
	t["get_height"] = nil
	p := make([]interface{}, 2)
	p[0] = "1"
	p[1] = true
	t["get_block_by_height"] = p
	// t["get_info"] = nil
	// t["gethashes"] = gethashes{BlockIDs: []string{"f28646b8ffd004fe405db1f304f3174c8bda9f1b8cbd1f87edd0c3ee1fc59cdb"}, StartHeight: uint64(0)}
	// t["getblocks"] = getblocks{
	// 	BlockIDs:    []string{"f28646b8ffd004fe405db1f304f3174c8bda9f1b8cbd1f87edd0c3ee1fc59cdb"},
	// 	StartHeight: uint64(0),
	// 	Prune:       false,
	// 	NoMinerTx:   false,
	// }

	return
}

func TestCallJSONRPC(t *testing.T) {
	testdata := getTestData()
	for method, param := range testdata {
		body, err := CallJSONRPC(method, param)
		if err != nil {
			t.Fatal("method:", method, ",param:", param, ",err:", err)
		}
		fmt.Printf("method:%s,param:%v,res body:%s\n", method, param, string(body))
	}
}
