package rtypes

import (
	"github.com/lianxiangcloud/linkchain/libs/common"
	"github.com/lianxiangcloud/linkchain/libs/ser"
	"github.com/lianxiangcloud/linkchain/types"
)

func init() {
	RegisterAmino()
}

func RegisterAmino() {
	ser.RegisterInterface((*ITX)(nil), nil)
	ser.RegisterConcrete(&RPCTx{}, "rpctx", nil)
	ser.RegisterConcrete(&common.Hash{}, "hash", nil)
	types.RegisterEventDatas()
	types.RegisterBlockAmino()
}
