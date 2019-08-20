package evidence

import (
	"github.com/lianxiangcloud/linkchain/types"
)

func init() {
	RegisterEvidenceMessages()
	types.RegisterBlockAmino()
}
