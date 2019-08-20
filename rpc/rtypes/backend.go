package rtypes

import (
	"context"
	"math/big"

	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/common"
)

type Backend interface {
	SuggestPrice(ctx context.Context) (*big.Int, error)
	AccountManager() *accounts.Manager
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
}
