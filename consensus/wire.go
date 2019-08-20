package consensus

import (
	"github.com/lianxiangcloud/linkchain/types"
)

func init() {
	RegisterConsensusMessages()
	RegisterWALMessages()
	types.RegisterBlockAmino()
}
