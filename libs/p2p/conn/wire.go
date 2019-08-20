package conn

import (
	"github.com/lianxiangcloud/linkchain/libs/crypto"
)

func init() {
	crypto.RegisterAmino()
	RegisterPacket()
}
