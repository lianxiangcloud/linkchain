package rpc

import (
	"github.com/lianxiangcloud/linkchain/accounts"
	"github.com/lianxiangcloud/linkchain/libs/bloombits"
	"github.com/lianxiangcloud/linkchain/libs/rpc"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	AccountManager() *accounts.Manager
	GetWallet() Wallet
}

func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		{
			Namespace: "ltk",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
			Public:    true,
		},
		{
			Namespace: "personal",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
			Public:    false,
		},
	}
}

type ApiBackend struct {
	s             *Service
	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
}

func (b *ApiBackend) context() *Context {
	return b.s.context()
}

// func (b *ApiBackend) bloomService() *BloomService {
// 	return b.s.bloom
// }

func NewApiBackend(s *Service) *ApiBackend {
	return &ApiBackend{
		s: s,
		// bloomRequests: make(chan chan *bloombits.Retrieval),
	}
}

func (b *ApiBackend) AccountManager() *accounts.Manager {
	return b.context().accManager
}

func (b *ApiBackend) GetWallet() Wallet {
	return b.context().wallet
}
