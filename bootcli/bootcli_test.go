package bootcli

import (
	"encoding/json"
	"testing"

	"github.com/lianxiangcloud/linkchain/libs/log"
)

func TestParseResponse(t *testing.T) {
	buffer := `{"code":0,"type":6,"nodes":[{"pubkey":"0x724c2517228e6aa0a5586dc947efab3860f6d887ed827722b847c58e3c5bf3a63b43ba1ad52ff1cc","endpoint":{"ip":["127.0.0.1","192.168.1.100"],"port":{"http":8081,"tcp":10001,"udp":10001}}},{"pubkey":"0x724c2517228e6aa0721ff16786634ed614aaa322f51019614abddd5f4868d0c90146c07dd9e6038b","endpoint":{"ip":["127.0.0.1"],"port":{"http":8080,"tcp":10000,"udp":10000}}},{"pubkey":"0x724c2517228e6aa06f7d693e3547587070e788dfdaa2aaf2b2afb6a5f5e060a637230d1b81b07688","endpoint":{"ip":["127.0.0.1"],"port":{"http":8082,"tcp":10002}}}]}`
	var resp GeetSeedsResp
	err := json.Unmarshal([]byte(buffer), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Code != Succ {
		t.Fatal("code:", resp.Code)
	}

	t.Log("type:", resp.Type)
	nodes := RapNodes(resp.Seeds, log.Test())
	for _, n := range nodes {
		t.Logf("%v", n)
	}
}
