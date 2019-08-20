package types

import (
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

func init() {
	RegisterBlockAmino()
}

func RegisterBlockAmino() {
	crypto.RegisterAmino()
	RegisterEvidences()
	RegisterTxData()
}
