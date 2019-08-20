package mempool

import (
	"github.com/lianxiangcloud/linkchain/types"
)

func init() {
	RegisterMempoolMessages()
	types.RegisterBlockAmino()
}
