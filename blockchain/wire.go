package blockchain

import (
	"github.com/lianxiangcloud/linkchain/types"
)

func init() {
	RegisterBlockchainMessages()
	types.RegisterBlockAmino()
}
