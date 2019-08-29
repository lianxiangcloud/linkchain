package discover

import (
	"testing"

	"fmt"

	common "github.com/lianxiangcloud/linkchain/libs/p2p/common"
)

func TestNodeMarshal(t *testing.T) {
	var id = common.NodeID{1, 3}
	data1, err := id.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON failed: %s", err)
		return
	}
	fmt.Printf("data1:%v\n", data1)
	var id2 = common.NodeID{}
	err = id2.UnmarshalJSON(data1)
	if err != nil {
		t.Fatalf("UnmarshalJSON failed: %s", err)
		return
	}
	if id != id2 {
		t.Fatalf("id:%v !=  id2:%v", id, id2)
	}
}
