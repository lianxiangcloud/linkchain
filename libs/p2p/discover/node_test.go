package discover

import (
	"testing"

	"fmt"

	"github.com/lianxiangcloud/linkchain/libs/crypto"
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

func TestLogDist(t *testing.T) {
	a := common.NodeID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	b := common.NodeID{2, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	l := LogDist(a, b)
	fmt.Printf("l:%d bucketMinDistance:%d\n", l, bucketMinDistance)
}

func TestRandLogDist(t *testing.T) {
	for i := 0; i < 50; i++ {
		priv1 := crypto.GenPrivKeyEd25519()
		a := common.TransPubKeyToNodeID(priv1.PubKey())
		priv2 := crypto.GenPrivKeyEd25519()
		b := common.TransPubKeyToNodeID(priv2.PubKey())
		l := LogDist(a, b)
		if l < 250 {
			fmt.Printf("i:%d a:%v b:%v\n", i, a, b)
			fmt.Printf("l:%d bucketMinDistance:%d\n", l, bucketMinDistance)
		}
	}
}
